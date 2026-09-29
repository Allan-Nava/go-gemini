package gogemini_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// redirectPair returns an API server that redirects every request to a second
// server, and a pointer to the API key that second server received.
func redirectPair(t *testing.T) (apiURL string, seen *atomic.Value) {
	t.Helper()
	seen = &atomic.Value{}
	seen.Store("")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("x-goog-api-key"))
		io.WriteString(w, `{}`)
	}))
	t.Cleanup(other.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(api.Close)
	return api.URL, seen
}

func TestRedirectToOtherHostIsRefused(t *testing.T) {
	apiURL, seen := redirectPair(t)
	c, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(apiURL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GenerateContent(context.Background(), "hi")
	if !errors.Is(err, gogemini.ErrRedirectOtherHost) {
		t.Fatalf("err = %v, want ErrRedirectOtherHost", err)
	}
	if got := seen.Load().(string); got != "" {
		t.Fatalf("the other host received the API key %q", got)
	}
}

func TestRedirectPolicyAppliedToCustomClient(t *testing.T) {
	apiURL, seen := redirectPair(t)
	custom := &http.Client{Timeout: 5 * time.Second}
	c, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(apiURL), gogemini.WithHTTPClient(custom))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GenerateContent(context.Background(), "hi"); !errors.Is(err, gogemini.ErrRedirectOtherHost) {
		t.Fatalf("err = %v, want ErrRedirectOtherHost", err)
	}
	if seen.Load().(string) != "" {
		t.Fatal("the other host received the API key")
	}
	if custom.CheckRedirect != nil {
		t.Error("New must not modify the caller's *http.Client")
	}
}

func TestCustomCheckRedirectIsKept(t *testing.T) {
	apiURL, _ := redirectPair(t)
	called := false
	custom := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		called = true
		return http.ErrUseLastResponse
	}}
	c, _ := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(apiURL), gogemini.WithHTTPClient(custom))
	_, err := c.GenerateContent(context.Background(), "hi")
	var apiErr *gogemini.APIError
	if !called || !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("called=%v err=%v, want the caller's policy and a 307 APIError", called, err)
	}
}

func TestRedirectOnSameHostIsFollowed(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/moved") {
			http.Redirect(w, r, srv.URL+r.URL.Path+"/moved", http.StatusTemporaryRedirect)
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer srv.Close()
	c, _ := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(srv.URL))
	resp, err := c.GenerateContent(context.Background(), "hi")
	if err != nil || resp.Text() != "ok" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestNewValidatesOptions(t *testing.T) {
	cases := []struct {
		name string
		opt  gogemini.Option
		want error
	}{
		{"plain http to a remote host", gogemini.WithBaseURL("http://example.com"), gogemini.ErrInvalidBaseURL},
		{"no host", gogemini.WithBaseURL("https://"), gogemini.ErrInvalidBaseURL},
		{"relative", gogemini.WithBaseURL("generativelanguage.googleapis.com"), gogemini.ErrInvalidBaseURL},
		{"other scheme", gogemini.WithBaseURL("ftp://example.com"), gogemini.ErrInvalidBaseURL},
		{"malformed", gogemini.WithBaseURL("https://[::1"), gogemini.ErrInvalidBaseURL},
		{"zero timeout", gogemini.WithTimeout(0), gogemini.ErrInvalidTimeout},
		{"negative timeout", gogemini.WithTimeout(-time.Second), gogemini.ErrInvalidTimeout},
		{"empty model", gogemini.WithModel(""), gogemini.ErrEmptyModel},
		{"blank model", gogemini.WithModel("  models/ "), gogemini.ErrEmptyModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := gogemini.New(gogemini.WithAPIKey(testKey), tc.opt); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	for _, ok := range []string{"https://example.com", "http://127.0.0.1:8080", "http://[::1]:9", "http://localhost:1"} {
		if _, err := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(ok)); err != nil {
			t.Errorf("WithBaseURL(%q): %v", ok, err)
		}
	}
}

func TestEmptyRequestsAreNotSent(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, `{}`)
	})
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"empty prompt": func() error { _, err := c.GenerateContent(ctx, ""); return err },
		"blank prompt": func() error { _, err := c.GenerateContent(ctx, " \n\t"); return err },
		"nil request":  func() error { _, err := c.Generate(ctx, nil); return err },
		"no contents":  func() error { _, err := c.Generate(ctx, &gogemini.GenerateContentRequest{}); return err },
	} {
		if err := call(); !errors.Is(err, gogemini.ErrEmptyRequest) {
			t.Errorf("%s: err = %v, want ErrEmptyRequest", name, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests reached the server", n)
	}
}

func TestErrorMessageIsTruncated(t *testing.T) {
	page := "<html>" + strings.Repeat("è", 5000) + "</html>"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, page)
	})
	_, err := c.GenerateContent(context.Background(), "hi")
	var apiErr *gogemini.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if len(apiErr.Message) > 1024+len("…") || !strings.HasSuffix(apiErr.Message, "…") || !utf8.ValidString(apiErr.Message) {
		t.Errorf("message not truncated cleanly: %d bytes, valid UTF-8 %v", len(apiErr.Message), utf8.ValidString(apiErr.Message))
	}
}

func TestUserAgent(t *testing.T) {
	var ua string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		io.WriteString(w, `{}`)
	})
	if _, err := c.GenerateContent(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ua, "go-gemini/") || len(ua) <= len("go-gemini/") {
		t.Errorf("User-Agent = %q, want go-gemini/<version>", ua)
	}
}

func TestConnectionIsReused(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Trailing data after the JSON value must not keep the connection from being reused.
		io.WriteString(w, `{"candidates":[]}`+strings.Repeat(" ", 64<<10))
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	c, _ := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(srv.URL))
	for range 3 {
		if _, err := c.GenerateContent(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 1 {
		t.Errorf("opened %d connections for 3 sequential calls, want 1", n)
	}
}
