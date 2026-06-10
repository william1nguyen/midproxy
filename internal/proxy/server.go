package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{Addr: s.addr, Handler: s}

	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down proxy server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info().Str("addr", s.addr).Msg("proxy server starting")
	return srv.ListenAndServe()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

func (s *Server) resolveProxy(ctx context.Context, domain string) (*store.SolveResult, string) {
	var solve *store.SolveResult
	if s.store != nil {
		solve, _ = s.store.GetSolveResult(ctx, domain)
	}

	if solve != nil && solve.ProxyURL != "" {
		return solve, solve.ProxyURL
	}
	return solve, s.manager.Pick()
}

func (s *Server) cacheGet(ctx context.Context, method, url string) ([]byte, *store.CachedResponse, bool) {
	if !s.cache || s.store == nil || method != http.MethodGet {
		return nil, nil, false
	}
	cached, err := s.store.GetCachedResponse(ctx, method, url)
	if err != nil {
		return nil, nil, false
	}
	body, err := cached.DecodeBody()
	if err != nil {
		return nil, nil, false
	}
	log.Info().Str("url", url).Int("status", cached.StatusCode).Int("body", len(body)).Msg("cache hit")
	return body, cached, true
}

func (s *Server) cacheSet(ctx context.Context, method, url string, resp *fetch.Response) {
	if s.cache && s.store != nil && method == http.MethodGet && resp.StatusCode == http.StatusOK {
		s.store.SetCachedResponse(ctx, method, url, store.EncodeCachedResponse(resp.StatusCode, resp.Header, resp.Body))
	}
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	domain := r.URL.Hostname()

	if s.store != nil && !s.store.AllowRequest(ctx, domain) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	if body, cached, ok := s.cacheGet(ctx, r.Method, r.URL.String()); ok {
		for k, vv := range cached.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(cached.StatusCode)
		w.Write(body)
		return
	}

	solve, proxyURL := s.resolveProxy(ctx, domain)
	logForward(r.URL.String(), proxyURL, solve)

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
		log.Info().Str("url", r.URL.String()).Msg("CF challenge detected, triggering solver")
		go s.solver.Trigger(context.Background(), r.URL.String())
		w.Header().Set("Retry-After", "60")
		http.Error(w, "solving challenge", http.StatusServiceUnavailable)
		return
	}

	s.cacheSet(ctx, r.Method, r.URL.String(), resp)

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

	reader := bufio.NewReader(tlsConn)

	for {
		tlsConn.SetReadDeadline(time.Now().Add(60 * time.Second))

		req, err := http.ReadRequest(reader)
		if err != nil {
			break
		}

		req.URL.Scheme = "https"
		req.URL.Host = host

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		if body, cached, ok := s.cacheGet(ctx, req.Method, req.URL.String()); ok {
			writeRawResponse(tlsConn, cached.StatusCode, cached.Header, body)
			cancel()
			continue
		}

		solve, proxyURL := s.resolveProxy(ctx, host)
		logForward(req.URL.String(), proxyURL, solve)

		resp, err := s.fetch.Forward(ctx, req, proxyURL, solve)
		req.Body.Close()
		if err != nil {
			log.Error().Err(err).Str("url", req.URL.String()).Msg("fetch failed")
			tlsConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			s.manager.RecordFailure(proxyURL)
			cancel()
			break
		}
		s.manager.RecordSuccess(proxyURL)
		log.Info().Str("url", req.URL.String()).Int("status", resp.StatusCode).Int("body", len(resp.Body)).Msg("fetch result")

		if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
			log.Info().Str("url", req.URL.String()).Msg("CF challenge detected, triggering solver")
			go s.solver.Trigger(context.Background(), req.URL.String())
			tlsConn.Write([]byte("HTTP/1.1 503 Service Unavailable\r\nRetry-After: 60\r\n\r\nsolving challenge\n"))
			cancel()
			break
		}

		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(resp.Body)))
		resp.Header.Del("Transfer-Encoding")

		s.cacheSet(ctx, req.Method, req.URL.String(), resp)

		writeRawResponse(tlsConn, resp.StatusCode, resp.Header, resp.Body)
		cancel()
	}
}

func writeRawResponse(w io.Writer, statusCode int, header http.Header, body []byte) {
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))
	header.Write(w)
	w.Write([]byte("\r\n"))
	w.Write(body)
}

func logForward(url, proxy string, solve *store.SolveResult) {
	if solve != nil {
		log.Info().Str("url", url).Str("proxy", proxy).Str("ua", solve.UserAgent).Msg("forwarding with cookies")
	} else {
		log.Info().Str("url", url).Str("proxy", proxy).Msg("forwarding without cookies")
	}
}
