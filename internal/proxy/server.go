package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/william1nguyen/midproxy/internal/fetch"
	"github.com/william1nguyen/midproxy/internal/solver"
	"github.com/william1nguyen/midproxy/internal/store"
)

type Server struct {
	addr    string
	manager *Manager
	fetch   *fetch.Client
	store   *store.Store
	solver  *solver.Solver
	cache   bool
}

type ServerConfig struct {
	Addr         string
	Manager      *Manager
	FetchClient  *fetch.Client
	Store        *store.Store
	Solver       *solver.Solver
	CacheEnabled bool
}

func NewServer(cfg ServerConfig) *Server {
	return &Server{
		addr:    cfg.Addr,
		manager: cfg.Manager,
		fetch:   cfg.FetchClient,
		store:   cfg.Store,
		solver:  cfg.Solver,
		cache:   cfg.CacheEnabled,
	}
}

func (s *Server) ListenAndServe() error {
	log.Info().Str("addr", s.addr).Msg("proxy server starting")
	return (&http.Server{Addr: s.addr, Handler: s}).ListenAndServe()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

// handleHTTP forwards HTTP requests with TLS fingerprinting, cache, cookies, rate limit, CF solving.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	domain := r.URL.Hostname()

	// rate limit
	if s.store != nil && !s.store.AllowRequest(ctx, domain) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	// cache check
	if s.cache && s.store != nil {
		if data, err := s.store.GetCache(ctx, r.URL.String()); err == nil {
			w.Header().Set("X-Cache", "HIT")
			w.Write(data)
			return
		}
	}

	proxyURL := s.manager.Pick()

	// load stored cookies
	var cookies []store.Cookie
	if s.store != nil {
		cookies, _ = s.store.GetCookies(ctx, domain)
	}

	// fetch with TLS client (Chrome fingerprint)
	resp, err := s.fetch.Forward(ctx, r, proxyURL, cookies)
	if err != nil {
		log.Error().Err(err).Str("url", r.URL.String()).Msg("fetch failed")
		http.Error(w, "bad gateway", http.StatusBadGateway)
		s.manager.RecordFailure(proxyURL)
		return
	}
	s.manager.RecordSuccess(proxyURL)

	// cloudflare challenge → solve → refetch
	if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
		solved, err := s.solver.Solve(ctx, r.URL.String(), proxyURL)
		if err == nil {
			if s.store != nil {
				s.store.SetCookies(ctx, domain, solved)
			}
			if retry, err := s.fetch.Forward(ctx, r, proxyURL, solved); err == nil {
				resp = retry
			}
		} else {
			log.Warn().Err(err).Msg("solver failed, returning original response")
		}
	}

	// cache successful responses
	if s.cache && s.store != nil && resp.StatusCode == http.StatusOK {
		s.store.SetCache(ctx, r.URL.String(), resp.Body)
	}

	// forward response to client
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(resp.Body)
}

// handleConnect tunnels HTTPS through upstream proxy (no fingerprinting possible).
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	proxyURL := s.manager.Pick()

	var targetConn net.Conn
	var err error

	if proxyURL != "" {
		targetConn, err = dialViaProxy(proxyURL, r.Host)
		if err != nil {
			log.Error().Err(err).Str("host", r.Host).Msg("CONNECT failed")
			http.Error(w, "bad gateway", http.StatusBadGateway)
			s.manager.RecordFailure(proxyURL)
			return
		}
		s.manager.RecordSuccess(proxyURL)
	} else {
		targetConn, err = net.DialTimeout("tcp", r.Host, 30*time.Second)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
	}
	defer targetConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	done := make(chan struct{}, 2)
	go func() { io.Copy(targetConn, clientConn); done <- struct{}{} }()
	go func() { io.Copy(clientConn, targetConn); done <- struct{}{} }()
	<-done
}

func dialViaProxy(proxyURL, targetHost string) (net.Conn, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", u.Host, 30*time.Second)
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetHost},
		Host:   targetHost,
		Header: make(http.Header),
	}
	if u.User != nil {
		pw, _ := u.User.Password()
		creds := base64.StdEncoding.EncodeToString([]byte(u.User.Username() + ":" + pw))
		req.Header.Set("Proxy-Authorization", "Basic "+creds)
	}

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("upstream CONNECT returned %d", resp.StatusCode)
	}
	return conn, nil
}
