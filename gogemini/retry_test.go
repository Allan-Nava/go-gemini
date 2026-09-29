package gogemini_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

var fastRetry = gogemini.RetryPolicy{MaxAttempts: 4, InitialDelay: time.Millisecond, MaxDelay: 20 * time.Millisecond}

// flaky answers with the given statuses in order, then 200 with a "pong" reply.
func flaky(t *testing.T, hits *atomic.Int32, statuses ...int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		if n <= len(statuses) {
			w.WriteHeader(statuses[n-1])
			io.WriteString(w, `{"error":{"code":1,"message":"try later","status":"UNAVAILABLE"}}`)
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"pong"}]}}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func retryClient(t *testing.T, url string, p gogemini.RetryPolicy) *gogemini.Client {
	t.Helper()
	c, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(url), gogemini.WithRetry(p))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRetryTransientThenSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := flaky(t, &hits, http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusInternalServerError)
	resp, err := retryClient(t, srv.URL, fastRetry).GenerateContent(context.Background(), "ping")
	if err != nil || resp.Text() != "pong" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if n := hits.Load(); n != 4 {
		t.Errorf("attempts = %d, want 4", n)
	}
}

func TestRetryGivesUpAfterMaxAttempts(t *testing.T) {
	var hits atomic.Int32
	srv := flaky(t, &hits, 503, 503, 503, 503, 503, 503)
	_, err := retryClient(t, srv.URL, fastRetry).GenerateContent(context.Background(), "ping")
	var apiErr *gogemini.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("err = %v, want the last 503", err)
	}
	if n := hits.Load(); n != 4 {
		t.Errorf("attempts = %d, want 4", n)
	}
}

func TestNoRetryOnClientErrors(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404} {
		var hits atomic.Int32
		srv := flaky(t, &hits, code)
		if _, err := retryClient(t, srv.URL, fastRetry).GenerateContent(context.Background(), "ping"); err == nil {
			t.Fatalf("%d: want an error", code)
		}
		if n := hits.Load(); n != 1 {
			t.Errorf("%d: attempts = %d, want 1", code, n)
		}
	}
}

func TestRetryHonoursRetryInfo(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED","details":[
				{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED","domain":"googleapis.com"},
				{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"0.08s"}]}}`)
			return
		}
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	p := gogemini.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Second}
	start := time.Now()
	if _, err := retryClient(t, srv.URL, p).GenerateContent(context.Background(), "ping"); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Errorf("retried after %v, want at least the 80ms Google asked for", d)
	}
}

func TestRetryInfoLongerThanMaxDelayGivesUp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED","details":[
			{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"37s"}]}}`)
	}))
	defer srv.Close()
	_, err := retryClient(t, srv.URL, fastRetry).GenerateContent(context.Background(), "ping")
	var apiErr *gogemini.APIError
	if !errors.As(err, &apiErr) || apiErr.RetryDelay != 37*time.Second {
		t.Fatalf("err = %v, want an APIError with RetryDelay 37s", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("attempts = %d, want 1: waiting 37s exceeds MaxDelay", n)
	}
}

func TestAPIErrorDetails(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"code":400,"message":"API key not valid.","status":"INVALID_ARGUMENT","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID","domain":"googleapis.com"}]}}`)
	})
	_, err := c.GenerateContent(context.Background(), "ping")
	var apiErr *gogemini.APIError
	if !errors.As(err, &apiErr) || apiErr.Reason != "API_KEY_INVALID" || apiErr.RetryDelay != 0 || apiErr.Retryable() {
		t.Fatalf("APIError = %+v", apiErr)
	}
}

func TestRetryStopsWhenContextIsCanceled(t *testing.T) {
	var hits atomic.Int32
	srv := flaky(t, &hits, 503, 503, 503)
	p := gogemini.RetryPolicy{MaxAttempts: 4, InitialDelay: time.Hour, MaxDelay: time.Hour}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := retryClient(t, srv.URL, p).GenerateContent(ctx, "ping")
	var apiErr *gogemini.APIError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want the deadline and the last APIError", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("attempts = %d, want 1", n)
	}
}

func TestRetryOnNetworkError(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close() // no answer at all
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"pong"}]}}]}`)
	}))
	defer srv.Close()
	resp, err := retryClient(t, srv.URL, fastRetry).GenerateContent(context.Background(), "ping")
	if err != nil || resp.Text() != "pong" || hits.Load() != 2 {
		t.Fatalf("resp=%v err=%v attempts=%d", resp, err, hits.Load())
	}
}

func TestRedirectRefusalIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer api.Close()
	_, err := retryClient(t, api.URL, fastRetry).GenerateContent(context.Background(), "ping")
	if !errors.Is(err, gogemini.ErrRedirectOtherHost) || hits.Load() != 1 {
		t.Fatalf("err=%v attempts=%d", err, hits.Load())
	}
}

func TestInvalidRetryPolicy(t *testing.T) {
	for name, p := range map[string]gogemini.RetryPolicy{
		"zero attempts":    {},
		"negative":         {MaxAttempts: -1},
		"no initial delay": {MaxAttempts: 3, MaxDelay: time.Second},
		"max below init":   {MaxAttempts: 3, InitialDelay: time.Second, MaxDelay: time.Millisecond},
	} {
		if _, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithRetry(p)); !errors.Is(err, gogemini.ErrInvalidRetryPolicy) {
			t.Errorf("%s: err = %v, want ErrInvalidRetryPolicy", name, err)
		}
	}
	if _, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithRetry(gogemini.RetryPolicy{MaxAttempts: 1})); err != nil {
		t.Errorf("MaxAttempts 1 without delays must be valid: %v", err)
	}
	if p := gogemini.DefaultRetryPolicy(); p.MaxAttempts != 4 || p.InitialDelay != time.Second || p.MaxDelay != 30*time.Second {
		t.Errorf("DefaultRetryPolicy() = %+v", p)
	}
}

func TestAPIErrorRetryable(t *testing.T) {
	for code, want := range map[int]bool{400: false, 403: false, 404: false, 408: true, 429: true, 500: true, 502: true, 503: true, 504: true} {
		if got := (&gogemini.APIError{StatusCode: code}).Retryable(); got != want {
			t.Errorf("Retryable(%d) = %v, want %v", code, got, want)
		}
	}
}
