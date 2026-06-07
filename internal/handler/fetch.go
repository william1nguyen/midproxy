package handler

import (
	"context"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/william1nguyen/midproxy/internal/middleware"

	"github.com/gin-gonic/gin"
)

type FetchRequest struct {
	URL     string            `json:"url" binding:"required"`
	Browser string            `json:"browser"`
	Headers map[string]string `json:"headers"`
}

type FetchResponse struct {
	HTML       string `json:"html"`
	StatusCode int    `json:"status_code"`
	URL        string `json:"url"`
}

type FetchHandler struct {
	deps Deps
}

func NewFetchHandler(deps Deps) *FetchHandler {
	return &FetchHandler{deps: deps}
}

func (h *FetchHandler) Handle(c *gin.Context) {
	params := middleware.GetFetchParams(c)
	if params == nil {
		c.JSON(fhttp.StatusBadRequest, gin.H{"error": "missing_params"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	proxyURL := h.pickProxy()
	cookies := h.loadCookies(ctx, params.Domain)

	html, code, err := h.deps.Fetcher.Fetch(ctx, params.URL, params.Headers, proxyURL, cookies)
	if err != nil {
		c.JSON(code, gin.H{"error": "fetch_failed", "message": err.Error()})
		h.recordProxyResult(proxyURL, false)
		return
	}
	h.recordProxyResult(proxyURL, true)

	if h.needSolver(params.Browser, code, html) {
		html, code, err = h.solveAndRefetch(ctx, c, params, proxyURL)
		if err != nil {
			return
		}
	}
	c.PureJSON(fhttp.StatusOK, FetchResponse{HTML: html, StatusCode: code, URL: params.URL})
}

func (h *FetchHandler) pickProxy() string {
	if h.deps.ProxyPicker == nil {
		return ""
	}
	return h.deps.ProxyPicker.Pick()
}

func (h *FetchHandler) recordProxyResult(proxyURL string, success bool) {
	if h.deps.ProxyPicker == nil || proxyURL == "" {
		return
	}

	if success {
		h.deps.ProxyPicker.RecordSuccess(proxyURL)
	} else {
		h.deps.ProxyPicker.RecordFailure(proxyURL)
	}
}

func (h *FetchHandler) loadCookies(ctx context.Context, domain string) []*fhttp.Cookie {
	if h.deps.CookieStore == nil {
		return nil
	}
	cookies, _ := h.deps.CookieStore.Get(ctx, domain)
	return cookies
}

func (h *FetchHandler) needSolver(browser string, code int, body string) bool {
	if h.deps.Solver == nil {
		return false
	}
	if browser == "true" {
		return true
	}
	if browser == "false" {
		return false
	}
	if h.deps.CFDetector == nil {
		return false
	}
	return h.deps.CFDetector.IsChallenge(code, body)
}

func (h *FetchHandler) solveAndRefetch(
	ctx context.Context,
	c *gin.Context,
	params *middleware.FetchParams,
	proxyURL string,
) (string, int, error) {
	cookies, err := h.deps.Solver.Solve(ctx, params.URL, proxyURL)
	if err != nil {
		c.JSON(fhttp.StatusBadGateway, gin.H{"error": "solver_failed", "message": err.Error()})
		return "", 0, err
	}

	if h.deps.CookieStore != nil {
		h.deps.CookieStore.Set(ctx, params.Domain, cookies)
	}

	html, code, err := h.deps.Fetcher.Fetch(ctx, params.URL, params.Headers, proxyURL, cookies)
	if err != nil {
		c.JSON(fhttp.StatusBadGateway, gin.H{"error": "fetch_after_solve_failed", "message": err.Error()})
		return "", 0, err
	}
	return html, code, nil
}
