package gogemini_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// hang blocks until the client goes away. The server only notices that once the
// request body has been read, so read it first.
func hang(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	<-r.Context().Done()
}

func TestLongStreamIsNotCut(t *testing.T) {
	// Six chunks over ~500ms with a 150ms timeout: the timeout covers the start only.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for i := range 6 {
			io.WriteString(w, sseChunk("x"))
			w.(http.Flusher).Flush()
			if i < 5 {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}, gogemini.WithTimeout(150*time.Millisecond))
	n := 0
	for _, err := range c.GenerateContentStream(context.Background(), "x") {
		if err != nil {
			t.Fatalf("stream cut after %d chunks: %v", n, err)
		}
		n++
	}
	if n != 6 {
		t.Fatalf("got %d chunks, want 6", n)
	}
}

func TestStreamThatDoesNotStartTimesOut(t *testing.T) {
	c := newTestClient(t, hang, gogemini.WithTimeout(100*time.Millisecond))
	start := time.Now()
	for _, err := range c.GenerateContentStream(context.Background(), "x") {
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "no reply within 100ms") {
			t.Fatalf("err = %v", err)
		}
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestGenerateAttemptTimeoutIsRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			hang(w, r)
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"pong"}]}}]}`)
	}))
	defer srv.Close()
	c, _ := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(srv.URL),
		gogemini.WithRetry(fastRetry), gogemini.WithTimeout(100*time.Millisecond))
	resp, err := c.GenerateContent(context.Background(), "ping")
	if err != nil || resp.Text() != "pong" || hits.Load() != 2 {
		t.Fatalf("resp=%v err=%v attempts=%d", resp, err, hits.Load())
	}
}

func TestTimeoutAppliesWithCustomClient(t *testing.T) {
	c := newTestClient(t, hang, gogemini.WithHTTPClient(&http.Client{}), gogemini.WithTimeout(100*time.Millisecond))
	start := time.Now()
	if _, err := c.GenerateContent(context.Background(), "x"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestCallerDeadlineIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		hang(w, r)
	}))
	defer srv.Close()
	c, _ := gogemini.New(gogemini.WithAPIKey(testKey), gogemini.WithBaseURL(srv.URL), gogemini.WithRetry(fastRetry))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := c.GenerateContent(ctx, "x")
	// The caller's own deadline comes back as it is: not as the client's timeout, and
	// without a retry wait in between.
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "no reply within") ||
		strings.Contains(err.Error(), "waiting to retry") {
		t.Fatalf("err = %v, want the caller's own deadline", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("attempts = %d, want 1: the caller's deadline is not retried", n)
	}
}
