package gogemini_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

const testKey = "test-key-not-real"

func newTestClient(t *testing.T, h http.HandlerFunc, opts ...gogemini.Option) *gogemini.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := gogemini.New(append([]gogemini.Option{gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestGenerateContent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if want := "/v1beta/models/gemini-test:generateContent"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		if got := r.Header.Get("x-goog-api-key"); got != testKey {
			t.Errorf("x-goog-api-key = %q", got)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, the key must not go in the URL", r.URL.RawQuery)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var req gogemini.GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Contents) != 1 || req.Contents[0].Role != "user" || req.Contents[0].Parts[0].Text != "ping" {
			t.Errorf("request = %+v", req)
		}
		io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"po"},{"text":"ng"}]},"finishReason":"STOP"}],
			"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2},"modelVersion":"gemini-test"}`)
	}, gogemini.WithModel("models/gemini-test"))

	resp, err := c.GenerateContent(context.Background(), "ping")
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if got := resp.Text(); got != "pong" {
		t.Errorf("Text() = %q, want pong", got)
	}
	if resp.UsageMetadata == nil || resp.UsageMetadata.TotalTokenCount != 2 {
		t.Errorf("UsageMetadata = %+v", resp.UsageMetadata)
	}
	if resp.Candidates[0].FinishReason != "STOP" || resp.ModelVersion != "gemini-test" {
		t.Errorf("response = %+v", resp)
	}
}

func TestAPIErrors(t *testing.T) {
	cases := []struct {
		name       string
		code       int
		body       string
		wantStatus string
		wantMsg    string
	}{
		{"bad request", 400, `{"error":{"code":400,"message":"API key not valid","status":"INVALID_ARGUMENT"}}`, "INVALID_ARGUMENT", "API key not valid"},
		{"forbidden", 403, `{"error":{"code":403,"message":"permission denied","status":"PERMISSION_DENIED"}}`, "PERMISSION_DENIED", "permission denied"},
		{"rate limited", 429, `{"error":{"code":429,"message":"quota exceeded","status":"RESOURCE_EXHAUSTED"}}`, "RESOURCE_EXHAUSTED", "quota exceeded"},
		{"not json", 502, `upstream went away`, "", "upstream went away"},
		{"empty body", 503, ``, "", "Service Unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				io.WriteString(w, tc.body)
			})
			_, err := c.GenerateContent(context.Background(), "hi")
			var apiErr *gogemini.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.StatusCode != tc.code || apiErr.Status != tc.wantStatus || apiErr.Message != tc.wantMsg {
				t.Errorf("APIError = %+v", apiErr)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("error message leaks the API key: %v", err)
			}
		})
	}
}

func TestMalformedResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"candidates":[`)
	})
	if _, err := c.GenerateContent(context.Background(), "hi"); err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("err = %v, want a decode error", err)
	}
}

func TestContextCanceled(t *testing.T) {
	release := make(chan struct{})
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.GenerateContent(ctx, "hi"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestBlockedPrompt(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"promptFeedback":{"blockReason":"SAFETY"}}`)
	})
	resp, err := c.GenerateContent(context.Background(), "hi")
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if resp.Text() != "" || resp.PromptFeedback == nil || resp.PromptFeedback.BlockReason != "SAFETY" {
		t.Errorf("response = %+v", resp)
	}
}

func TestNewAPIKey(t *testing.T) {
	t.Setenv(gogemini.APIKeyEnv, "")
	if _, err := gogemini.New(); !errors.Is(err, gogemini.ErrMissingAPIKey) {
		t.Fatalf("New() err = %v, want ErrMissingAPIKey", err)
	}

	t.Setenv(gogemini.APIKeyEnv, "from-env")
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-goog-api-key")
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	c, err := gogemini.New(gogemini.WithBaseURL(srv.URL + "/"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GenerateContent(context.Background(), "hi"); err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if got != "from-env" {
		t.Errorf("key from env = %q", got)
	}

	c, _ = gogemini.New(gogemini.WithAPIKey("explicit"), gogemini.WithBaseURL(srv.URL))
	c.GenerateContent(context.Background(), "hi")
	if got != "explicit" {
		t.Errorf("WithAPIKey should win over the env: got %q", got)
	}
}

func TestDefaults(t *testing.T) {
	c, err := gogemini.New(gogemini.WithAPIKey(testKey))
	if err != nil {
		t.Fatal(err)
	}
	if c.Model() != gogemini.DefaultModel {
		t.Errorf("Model() = %q", c.Model())
	}
	var nilResp *gogemini.Response
	if nilResp.Text() != "" {
		t.Error("Text() on nil response should be empty")
	}
}

type countingTransport struct{ calls int }

func (t *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls++
	return http.DefaultTransport.RoundTrip(r)
}

func TestWithHTTPClient(t *testing.T) {
	tr := &countingTransport{}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) },
		gogemini.WithHTTPClient(&http.Client{Transport: tr}))
	if _, err := c.GenerateContent(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if tr.calls != 1 {
		t.Errorf("custom transport calls = %d, want 1", tr.calls)
	}
}

func TestWithTimeout(t *testing.T) {
	release := make(chan struct{})
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}, gogemini.WithTimeout(50*time.Millisecond))
	defer close(release)
	start := time.Now()
	_, err := c.GenerateContent(context.Background(), "hi")
	if err == nil {
		t.Fatal("want a timeout error")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("timeout not applied: took %v", d)
	}
}
