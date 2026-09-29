package gogemini

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// maxErrorBody caps how much of an error reply is read.
	maxErrorBody = 1 << 20
	// maxErrorMessage caps APIError.Message, so a proxy's HTML page does not flood the logs.
	maxErrorMessage = 1024
)

// APIError is a non-2xx reply from the Gemini API, or an error event in a stream.
type APIError struct {
	StatusCode int    // HTTP status, e.g. 429
	Status     string // Google status, e.g. "RESOURCE_EXHAUSTED"
	Message    string
	// Reason is the machine-readable reason from Google's ErrorInfo detail,
	// e.g. "API_KEY_INVALID", when the reply has one.
	Reason string
	// RetryDelay is how long Google asks to wait before retrying (RetryInfo detail),
	// or 0 when the reply does not say.
	RetryDelay time.Duration
}

func (e *APIError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("gogemini: HTTP %d %s: %s", e.StatusCode, e.Status, e.Message)
	}
	return fmt.Sprintf("gogemini: HTTP %d: %s", e.StatusCode, e.Message)
}

// Retryable reports whether the error is transient: 408, 429 or a 5xx status.
// The client retries these on its own, according to its RetryPolicy.
func (e *APIError) Retryable() bool {
	switch e.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return e.StatusCode >= 500 && e.StatusCode <= 599
}

// rpcStatus is Google's error body: {"error": {"code", "message", "status", "details"}}.
type rpcStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
	Details []struct {
		Type       string `json:"@type"`
		Reason     string `json:"reason"`
		RetryDelay string `json:"retryDelay"`
	} `json:"details"`
}

func (s *rpcStatus) apiError(httpStatus int) *APIError {
	e := &APIError{StatusCode: httpStatus, Status: s.Status, Message: truncate(s.Message, maxErrorMessage)}
	if e.StatusCode == 0 {
		e.StatusCode = s.Code
	}
	for _, d := range s.Details {
		switch {
		case strings.HasSuffix(d.Type, "google.rpc.ErrorInfo"):
			e.Reason = d.Reason
		case strings.HasSuffix(d.Type, "google.rpc.RetryInfo"):
			// Google encodes durations as decimal seconds, e.g. "37s" or "1.5s".
			if d, err := time.ParseDuration(d.RetryDelay); err == nil && d > 0 {
				e.RetryDelay = d
			}
		}
	}
	return e
}

func decodeError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	var wrapped struct {
		Error *rpcStatus `json:"error"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Error != nil && wrapped.Error.Message != "" {
		return wrapped.Error.apiError(resp.StatusCode)
	}
	msg := strings.TrimSpace(string(raw))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return &APIError{StatusCode: resp.StatusCode, Message: truncate(msg, maxErrorMessage)}
}

// truncate shortens s to at most n bytes without splitting a UTF-8 character.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}
