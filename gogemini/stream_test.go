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

func sseChunk(text string) string {
	return `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"` + text + `"}]}}]}` + "\n\n"
}

func TestGenerateContentStream(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if want := "/v1beta/models/gemini-test:streamGenerateContent"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		if got := r.URL.Query().Get("alt"); got != "sse" {
			t.Errorf("alt = %q, want sse", got)
		}
		if r.URL.Query().Has("key") || r.Header.Get("x-goog-api-key") != testKey {
			t.Error("the key must travel in the header only")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": keep-alive comment\n\n")
		io.WriteString(w, sseChunk("Once "))
		io.WriteString(w, "event: message\r\n"+`data: {"candidates":[{"content":{"parts":[{"text":"upon "}]}}]}`+"\r\n\r\n")
		// The last event has no trailing blank line: it must still be delivered.
		io.WriteString(w, `data: {"candidates":[{"content":{"parts":[{"text":"a time"}]}}],"usageMetadata":{"totalTokenCount":7}}`)
	}, gogemini.WithModel("gemini-test"))

	var got strings.Builder
	var last *gogemini.Response
	for chunk, err := range c.GenerateContentStream(context.Background(), "story") {
		if err != nil {
			t.Fatal(err)
		}
		got.WriteString(chunk.Text())
		last = chunk
	}
	if got.String() != "Once upon a time" {
		t.Errorf("text = %q", got.String())
	}
	if last == nil || last.UsageMetadata == nil || last.UsageMetadata.TotalTokenCount != 7 {
		t.Errorf("last chunk = %+v", last)
	}
}

func TestStreamMultiLineData(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "data: {\"candidates\":[{\"content\":\ndata: {\"parts\":[{\"text\":\"joined\"}]}}]}\n\n")
	})
	for chunk, err := range c.GenerateContentStream(context.Background(), "x") {
		if err != nil || chunk.Text() != "joined" {
			t.Fatalf("chunk=%v err=%v", chunk, err)
		}
	}
}

func TestStreamBreakClosesConnection(t *testing.T) {
	closed := make(chan struct{})
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, sseChunk("first"))
		w.(http.Flusher).Flush()
		<-r.Context().Done() // the client going away ends the request
		close(closed)
	})
	start := time.Now()
	for chunk, err := range c.GenerateContentStream(context.Background(), "x") {
		if err != nil || chunk.Text() != "first" {
			t.Fatalf("chunk=%v err=%v", chunk, err)
		}
		break
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("breaking out of the stream took %v, want it to return at once", d)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("breaking out of the loop did not close the connection")
	}
}

func TestStreamErrorEvent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, sseChunk("partial"))
		io.WriteString(w, `data: {"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}`+"\n\n")
		io.WriteString(w, sseChunk("never seen"))
	})
	var texts []string
	var gotErr error
	for chunk, err := range c.GenerateContentStream(context.Background(), "x") {
		if err != nil {
			gotErr = err
			continue
		}
		texts = append(texts, chunk.Text())
	}
	var apiErr *gogemini.APIError
	if !errors.As(gotErr, &apiErr) || apiErr.StatusCode != 503 || apiErr.Status != "UNAVAILABLE" {
		t.Fatalf("err = %v, want a 503 APIError", gotErr)
	}
	if len(texts) != 1 || texts[0] != "partial" {
		t.Errorf("chunks = %q, want only the one before the error", texts)
	}
}

func TestStreamMalformedEvent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "data: {not json\n\n")
	})
	for _, err := range c.GenerateContentStream(context.Background(), "x") {
		if err == nil || !strings.Contains(err.Error(), "decode stream event") {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestStreamRetriesBeforeFirstChunk(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, sseChunk("ok"))
	}))
	defer srv.Close()
	var got string
	for chunk, err := range retryClient(t, srv.URL, fastRetry).GenerateContentStream(context.Background(), "x") {
		if err != nil {
			t.Fatal(err)
		}
		got += chunk.Text()
	}
	if got != "ok" || hits.Load() != 2 {
		t.Fatalf("text=%q attempts=%d", got, hits.Load())
	}
}

func TestStreamEmptyPrompt(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	n := 0
	for _, err := range c.GenerateContentStream(context.Background(), "  ") {
		n++
		if !errors.Is(err, gogemini.ErrEmptyRequest) {
			t.Fatalf("err = %v", err)
		}
	}
	for _, err := range c.GenerateStream(context.Background(), nil) {
		n++
		if !errors.Is(err, gogemini.ErrEmptyRequest) {
			t.Fatalf("err = %v", err)
		}
	}
	if n != 2 || hits.Load() != 0 {
		t.Errorf("yields=%d requests=%d", n, hits.Load())
	}
}
