package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
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
	certs   *certStore
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
	certs, err := newCertStore()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cert store")
	}
	return &Server{
		addr:    cfg.Addr,
		manager: cfg.Manager,
		fetch:   cfg.FetchClient,
		store:   cfg.Store,
		solver:  cfg.Solver,
		cache:   cfg.CacheEnabled,
		certs:   certs,
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

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	domain := r.URL.Hostname()

	if s.store != nil && !s.store.AllowRequest(ctx, domain) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	if s.cache && s.store != nil {
		if data, err := s.store.GetCache(ctx, r.URL.String()); err == nil {
			w.Header().Set("X-Cache", "HIT")
			w.Write(data)
			return
		}
	}

	proxyURL := s.manager.Pick()

	var solve *store.SolveResult
	if s.store != nil {
		solve, _ = s.store.GetSolveResult(ctx, domain)
	}

	if solve != nil {
		log.Info().Str("url", r.URL.String()).Str("proxy", proxyURL).Str("ua", solve.UserAgent).Interface("cookies", solve.Cookies).Msg("forwarding with cookies")
	} else {
		log.Info().Str("url", r.URL.String()).Str("proxy", proxyURL).Msg("forwarding without cookies")
	}

	resp, err := s.fetch.Forward(ctx, r, proxyURL, solve)
	if err != nil {
		log.Error().Err(err).Str("url", r.URL.String()).Msg("fetch failed")
		http.Error(w, "bad gateway", http.StatusBadGateway)
		s.manager.RecordFailure(proxyURL)
		return
	}
	s.manager.RecordSuccess(proxyURL)
	log.Info().Str("url", r.URL.String()).Int("status", resp.StatusCode).Int("body", len(resp.Body)).Msg("fetch result")

	if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
		log.Info().Str("url", r.URL.String()).Int("status", resp.StatusCode).Msg("CF challenge detected, triggering solver")
		go s.solver.Trigger(context.Background(), r.URL.String(), proxyURL)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "solving challenge", http.StatusServiceUnavailable)
		return
	}

	if s.cache && s.store != nil && resp.StatusCode == http.StatusOK {
		s.store.SetCache(ctx, r.URL.String(), resp.Body)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(resp.Body)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

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

	cert, err := s.certs.get(host)
	if err != nil {
		log.Error().Err(err).Str("host", host).Msg("cert generation failed")
		return
	}
	tlsConn := tls.Server(clientConn, &tls.Config{Certificates: []tls.Certificate{*cert}})
	if err := tlsConn.Handshake(); err != nil {
		log.Error().Err(err).Str("host", host).Msg("TLS handshake failed")
		return
	}
	defer tlsConn.Close()

	req, err := http.ReadRequest(bufio.NewReader(tlsConn))
	if err != nil {
		log.Error().Err(err).Msg("read request failed")
		return
	}
	defer req.Body.Close()

	req.URL.Scheme = "https"
	req.URL.Host = host

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	domain := host
	proxyURL := s.manager.Pick()

	var solve *store.SolveResult
	if s.store != nil {
		solve, _ = s.store.GetSolveResult(ctx, domain)
	}

	if solve != nil {
		log.Info().Str("url", req.URL.String()).Str("proxy", proxyURL).Str("ua", solve.UserAgent).Interface("cookies", solve.Cookies).Msg("forwarding with cookies")
	} else {
		log.Info().Str("url", req.URL.String()).Str("proxy", proxyURL).Msg("forwarding without cookies")
	}

	resp, err := s.fetch.Forward(ctx, req, proxyURL, solve)
	if err != nil {
		log.Error().Err(err).Str("url", req.URL.String()).Msg("fetch failed")
		tlsConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		s.manager.RecordFailure(proxyURL)
		return
	}
	s.manager.RecordSuccess(proxyURL)
	log.Info().Str("url", req.URL.String()).Int("status", resp.StatusCode).Int("body", len(resp.Body)).Msg("fetch result")

	if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
		log.Info().Str("url", req.URL.String()).Msg("CF challenge detected, triggering solver")
		go s.solver.Trigger(context.Background(), req.URL.String(), proxyURL)
		tlsConn.Write([]byte("HTTP/1.1 503 Service Unavailable\r\nRetry-After: 60\r\n\r\nsolving challenge\n"))
		return
	}

	fmt.Fprintf(tlsConn, "HTTP/1.1 %d %s\r\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	resp.Header.Write(tlsConn)
	tlsConn.Write([]byte("\r\n"))
	tlsConn.Write(resp.Body)
}
