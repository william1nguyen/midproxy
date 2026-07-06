package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/william1nguyen/midproxy/internal/errs"
	"github.com/william1nguyen/midproxy/internal/fetch"
	"github.com/william1nguyen/midproxy/internal/ratelimit"
	"github.com/william1nguyen/midproxy/internal/solver"
	"github.com/william1nguyen/midproxy/internal/store"
)

const maxReplayBodyBytes = 1 << 20

type Server struct {
	addr           string
	manager        *Manager
	fetch          *fetch.Client
	store          *store.Store
	solver         *solver.Solver
	limiter        ratelimit.Limiter
	cache          bool
	certs          *CertStore
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

type ServerConfig struct {
	Addr           string
	Manager        *Manager
	FetchClient    *fetch.Client
	Store          *store.Store
	Solver         *solver.Solver
	Limiter        ratelimit.Limiter
	CacheEnabled   bool
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

func NewServer(cfg *ServerConfig) *Server {
	certs, err := NewCertStore()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cert store")
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseDelay := cfg.RetryBaseDelay
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	maxDelay := cfg.RetryMaxDelay
	if maxDelay <= 0 {
		maxDelay = 8 * time.Second
	}
	return &Server{
		addr:           cfg.Addr,
		manager:        cfg.Manager,
		fetch:          cfg.FetchClient,
		store:          cfg.Store,
		solver:         cfg.Solver,
		limiter:        cfg.Limiter,
		cache:          cfg.CacheEnabled,
		certs:          certs,
		maxRetries:     maxRetries,
		retryBaseDelay: baseDelay,
		retryMaxDelay:  maxDelay,
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

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

	if s.limiter != nil && !s.limiter.Allow(r.Context(), clientIP(r)) {
		errs.WriteJSON(w, errs.RateLimited(60))
		return
	}
	s.handleHTTP(w, r)
}

func (s *Server) backoff(attempt int) time.Duration {
	delay := s.retryBaseDelay << uint(attempt)
	if delay > s.retryMaxDelay {
		delay = s.retryMaxDelay
	}
	return delay
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

func cacheControlValues(h http.Header) []string {
	values := h.Values("Cache-Control")
	if len(values) == 0 {
		return nil
	}
	var directives []string
	for _, value := range values {
		for _, directive := range strings.Split(value, ",") {
			if directive = strings.TrimSpace(strings.ToLower(directive)); directive != "" {
				directives = append(directives, directive)
			}
		}
	}
	return directives
}

func hasCacheDirective(h http.Header, names ...string) bool {
	for _, directive := range cacheControlValues(h) {
		key := directive
		if i := strings.IndexByte(key, '='); i >= 0 {
			key = key[:i]
		}
		for _, name := range names {
			if key == name {
				return true
			}
		}
	}
	return false
}

func hasZeroMaxAge(h http.Header) bool {
	for _, directive := range cacheControlValues(h) {
		key, value, ok := strings.Cut(directive, "=")
		if !ok || (key != "max-age" && key != "s-maxage") {
			continue
		}
		seconds, err := strconv.Atoi(strings.Trim(value, `"`))
		if err == nil && seconds <= 0 {
			return true
		}
	}
	return false
}

func cacheMaxAge(h http.Header) (time.Duration, bool) {
	var maxAge time.Duration
	for _, directive := range cacheControlValues(h) {
		key, value, ok := strings.Cut(directive, "=")
		if !ok || (key != "max-age" && key != "s-maxage") {
			continue
		}
		seconds, err := strconv.Atoi(strings.Trim(value, `"`))
		if err == nil && seconds > 0 {
			if key == "s-maxage" {
				return time.Duration(seconds) * time.Second, true
			}
			maxAge = time.Duration(seconds) * time.Second
		}
	}
	return maxAge, maxAge > 0
}

func requestBypassesCache(r *http.Request) bool {
	return r.Method != http.MethodGet ||
		r.Header.Get("Authorization") != "" ||
		r.Header.Get("Cookie") != "" ||
		strings.EqualFold(r.Header.Get("Pragma"), "no-cache") ||
		hasCacheDirective(r.Header, "no-cache", "no-store") ||
		hasZeroMaxAge(r.Header)
}

func responseCacheable(resp *fetch.Response) bool {
	return resp.StatusCode == http.StatusOK &&
		resp.Header.Get("Set-Cookie") == "" &&
		resp.Header.Get("Vary") == "" &&
		!hasCacheDirective(resp.Header, "private", "no-cache", "no-store") &&
		!hasZeroMaxAge(resp.Header)
}

func (s *Server) cacheGet(ctx context.Context, r *http.Request) ([]byte, *store.CachedResponse, bool) {
	if !s.cache || s.store == nil || requestBypassesCache(r) {
		return nil, nil, false
	}
	cached, err := s.store.GetCachedResponse(ctx, r.Method, r.URL.String())
	if err != nil {
		return nil, nil, false
	}
	body, err := cached.DecodeBody()
	if err != nil {
		return nil, nil, false
	}
	log.Info().Str("url", r.URL.String()).Int("status", cached.StatusCode).Int("body", len(body)).Msg("cache hit")
	return body, cached, true
}

func (s *Server) cacheSet(ctx context.Context, r *http.Request, resp *fetch.Response) {
	if s.cache && s.store != nil && !requestBypassesCache(r) && responseCacheable(resp) {
		cached := store.EncodeCachedResponse(resp.StatusCode, resp.Header, resp.Body)
		if ttl, ok := cacheMaxAge(resp.Header); ok {
			s.store.SetCachedResponseWithTTL(ctx, r.Method, r.URL.String(), cached, ttl)
			return
		}
		s.store.SetCachedResponse(ctx, r.Method, r.URL.String(), cached)
	}
}

func (s *Server) triggerSolve(ctx context.Context, targetURL, domain string, solve *store.SolveResult) int {
	force := solve != nil
	if force {
		s.store.InvalidateSolveResult(ctx, domain)
	}
	return s.solver.Trigger(ctx, targetURL, domain, force)
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	domain := r.URL.Hostname()

	if body, cached, ok := s.cacheGet(ctx, r); ok {
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

	replayable, err := prepareBodyReplay(r)
	if err != nil {
		errs.WriteJSON(w, errs.BadGateway(err.Error()))
		return
	}
	attempts := s.maxRetries
	if !replayable {
		attempts = 1
	}

	for attempt := range attempts {
		solve, proxyURL := s.resolveProxy(ctx, domain)
		logForward(r.URL.String(), proxyURL, solve)

		resp, err := s.fetch.Forward(ctx, r, proxyURL, solve)
		if err != nil {
			log.Error().Err(err).Str("url", r.URL.String()).Int("attempt", attempt+1).Msg("fetch failed")
			s.manager.RecordFailure(proxyURL)
			if attempt < attempts-1 {
				time.Sleep(s.backoff(attempt))
				continue
			}
			errs.WriteJSON(w, errs.BadGateway("upstream request failed after retries"))
			return
		}
		s.manager.RecordSuccess(proxyURL)
		log.Info().Str("url", r.URL.String()).Int("status", resp.StatusCode).Int("body", len(resp.Body)).Msg("fetch result")

		if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
			if solve != nil && attempt < attempts-1 {
				log.Info().Str("url", r.URL.String()).Int("attempt", attempt+1).Msg("CF challenge with cookies, retrying")
				time.Sleep(s.backoff(attempt))
				continue
			}
			log.Info().Str("url", r.URL.String()).Msg("CF challenge detected, triggering solver")
			retryAfter := s.triggerSolve(ctx, r.URL.String(), domain, solve)
			errs.WriteJSON(w, errs.Solving(retryAfter))
			return
		}

		s.cacheSet(ctx, r, resp)

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(resp.Body)
		return
	}
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		errs.WriteJSON(w, errs.Internal("hijacking not supported"))
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	cert, err := s.certs.Get(host)
	if err != nil {
		log.Error().Err(err).Str("host", host).Msg("cert generation failed")
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
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

		if s.limiter != nil && !s.limiter.Allow(ctx, clientIP(r)) {
			tlsConn.Write([]byte(errs.WriteRaw(nil, errs.RateLimited(60))))
			cancel()
			break
		}

		if body, cached, ok := s.cacheGet(ctx, req); ok {
			writeRawResponse(tlsConn, cached.StatusCode, cached.Header, body)
			cancel()
			continue
		}

		result := s.forwardWithRetry(ctx, req, host)
		req.Body.Close()

		if result.err != nil {
			tlsConn.Write([]byte(errs.WriteRaw(nil, errs.BadGateway("upstream request failed"))))
			cancel()
			break
		}

		if result.solving {
			tlsConn.Write([]byte(errs.WriteRaw(nil, errs.Solving(result.retryAfter))))
			cancel()
			break
		}

		result.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(result.resp.Body)))
		result.resp.Header.Del("Transfer-Encoding")

		s.cacheSet(ctx, req, result.resp)

		writeRawResponse(tlsConn, result.resp.StatusCode, result.resp.Header, result.resp.Body)
		cancel()
	}
}

type forwardResult struct {
	resp       *fetch.Response
	err        error
	solving    bool
	retryAfter int
}

func (s *Server) forwardWithRetry(ctx context.Context, req *http.Request, domain string) forwardResult {
	replayable, err := prepareBodyReplay(req)
	if err != nil {
		return forwardResult{err: err}
	}
	attempts := s.maxRetries
	if !replayable {
		attempts = 1
	}

	for attempt := range attempts {
		solve, proxyURL := s.resolveProxy(ctx, domain)
		logForward(req.URL.String(), proxyURL, solve)

		resp, err := s.fetch.Forward(ctx, req, proxyURL, solve)
		if err != nil {
			log.Error().Err(err).Str("url", req.URL.String()).Int("attempt", attempt+1).Msg("fetch failed")
			s.manager.RecordFailure(proxyURL)
			if attempt < attempts-1 {
				time.Sleep(s.backoff(attempt))
				continue
			}
			return forwardResult{err: err}
		}
		s.manager.RecordSuccess(proxyURL)
		log.Info().Str("url", req.URL.String()).Int("status", resp.StatusCode).Int("body", len(resp.Body)).Msg("fetch result")

		if s.solver != nil && fetch.IsCloudflareChallenge(resp.StatusCode, resp.Body) {
			if solve != nil && attempt < attempts-1 {
				log.Info().Str("url", req.URL.String()).Int("attempt", attempt+1).Msg("CF challenge with cookies, retrying")
				time.Sleep(s.backoff(attempt))
				continue
			}
			log.Info().Str("url", req.URL.String()).Msg("CF challenge detected, triggering solver")
			retryAfter := s.triggerSolve(ctx, req.URL.String(), domain, solve)
			return forwardResult{solving: true, retryAfter: retryAfter}
		}

		return forwardResult{resp: resp}
	}
	return forwardResult{err: fmt.Errorf("max retries exceeded for %s", domain)}
}

func prepareBodyReplay(req *http.Request) (bool, error) {
	if req.Body == nil || req.Body == http.NoBody {
		req.Body = http.NoBody
		req.GetBody = func() (io.ReadCloser, error) {
			return http.NoBody, nil
		}
		return true, nil
	}

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return false, fmt.Errorf("prepare request body: %w", err)
		}
		req.Body = body
		return true, nil
	}

	original := req.Body
	limited := io.LimitReader(original, maxReplayBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return false, fmt.Errorf("read request body: %w", err)
	}
	if len(body) > maxReplayBodyBytes {
		req.Body = readCloser{
			Reader: io.MultiReader(bytes.NewReader(body), original),
			Closer: original,
		}
		return false, nil
	}
	original.Close()

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return true, nil
}

type readCloser struct {
	io.Reader
	io.Closer
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
