package errs

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeBadGateway    Code = "BAD_GATEWAY"
	CodeRateLimited   Code = "RATE_LIMITED"
	CodeSolving       Code = "SOLVING_CHALLENGE"
	CodeHijackFailed  Code = "HIJACK_FAILED"
	CodeInternalError Code = "INTERNAL_ERROR"
)

type ProxyError struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

func (e *ProxyError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func BadGateway(msg string) *ProxyError {
	return &ProxyError{Code: CodeBadGateway, Message: msg, StatusCode: http.StatusBadGateway}
}

func RateLimited(retryAfter int) *ProxyError {
	return &ProxyError{
		Code:       CodeRateLimited,
		Message:    "too many requests",
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: retryAfter,
	}
}

func Solving(retryAfter int) *ProxyError {
	return &ProxyError{
		Code:       CodeSolving,
		Message:    "solving challenge, retry later",
		StatusCode: http.StatusServiceUnavailable,
		RetryAfter: retryAfter,
	}
}

func Internal(msg string) *ProxyError {
	return &ProxyError{Code: CodeInternalError, Message: msg, StatusCode: http.StatusInternalServerError}
}

func WriteJSON(w http.ResponseWriter, e *ProxyError) {
	w.Header().Set("Content-Type", "application/json")
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", e.RetryAfter))
	}
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(e)
}

func WriteRaw(w http.ResponseWriter, e *ProxyError) string {
	body, _ := json.Marshal(e)
	return fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: application/json\r\n", e.StatusCode, http.StatusText(e.StatusCode)) +
		func() string {
			if e.RetryAfter > 0 {
				return fmt.Sprintf("Retry-After: %d\r\n", e.RetryAfter)
			}
			return ""
		}() +
		fmt.Sprintf("\r\n%s\n", body)
}
