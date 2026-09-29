package gogemini_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// echoModel replies "re: <last user text>" and records every request body.
type echoModel struct {
	mu     sync.Mutex
	bodies []gogemini.GenerateContentRequest
	raw    []map[string]any
	fail   bool
	block  bool
}

func (m *echoModel) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var req gogemini.GenerateContentRequest
		var raw map[string]any
		if err := json.Unmarshal(data, &req); err != nil || json.Unmarshal(data, &raw) != nil {
			t.Errorf("bad request body: %s", data)
		}
		m.mu.Lock()
		m.bodies, m.raw = append(m.bodies, req), append(m.raw, raw)
		fail, block := m.fail, m.block
		m.mu.Unlock()
		switch {
		case fail:
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":{"code":400,"message":"nope","status":"INVALID_ARGUMENT"}}`)
		case block:
			io.WriteString(w, `{"promptFeedback":{"blockReason":"SAFETY"}}`)
		default:
			last := req.Contents[len(req.Contents)-1].Parts[0].Text
			fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"re: %s"}]}}]}`, last)
		}
	}
}

func TestChatCarriesHistory(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t))
	chat := c.NewChat()
	for _, msg := range []string{"hello", "how are you?"} {
		resp, err := chat.Send(context.Background(), msg)
		if err != nil || resp.Text() != "re: "+msg {
			t.Fatalf("Send(%q) = %v, %v", msg, resp, err)
		}
	}
	second := m.bodies[1].Contents
	roles := []string{}
	for _, c := range second {
		roles = append(roles, c.Role+":"+c.Parts[0].Text)
	}
	if got := strings.Join(roles, " | "); got != "user:hello | model:re: hello | user:how are you?" {
		t.Errorf("second request contents = %s", got)
	}
	if h := chat.History(); len(h) != 4 || h[3].Role != "model" || h[3].Parts[0].Text != "re: how are you?" {
		t.Errorf("History() = %+v", h)
	}
}

func TestChatHistoryUnchangedOnErrorOrBlock(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t))
	chat := c.NewChat()
	if _, err := chat.Send(context.Background(), "first"); err != nil {
		t.Fatal(err)
	}

	m.fail = true
	if _, err := chat.Send(context.Background(), "fails"); err == nil {
		t.Fatal("want an error")
	}
	m.fail, m.block = false, true
	if resp, err := chat.Send(context.Background(), "blocked"); err != nil || resp.Text() != "" {
		t.Fatalf("blocked: %v, %v", resp, err)
	}
	if h := chat.History(); len(h) != 2 {
		t.Errorf("history has %d contents, want 2: failed and blocked turns must not be kept", len(h))
	}
	if _, err := chat.Send(context.Background(), " "); err == nil {
		t.Error("empty text must fail")
	}
}

func TestChatCopiesHistory(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t))
	start := []gogemini.Content{{Role: "user", Parts: []gogemini.Part{{Text: "earlier"}}}, {Role: "model", Parts: []gogemini.Part{{Text: "reply"}}}}
	chat := c.NewChat(start...)
	start[0].Parts[0].Text = "changed by the caller"
	h := chat.History()
	h[1].Parts[0].Text = "changed through History"
	if got := chat.History(); got[0].Parts[0].Text != "earlier" || got[1].Parts[0].Text != "reply" {
		t.Errorf("history was modified from outside: %+v", got)
	}
}

func TestChatConcurrentSends(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t))
	chat := c.NewChat()
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := chat.Send(context.Background(), fmt.Sprint("msg ", i)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	h := chat.History()
	if len(h) != 20 {
		t.Fatalf("history has %d contents, want 20", len(h))
	}
	for i := 0; i < len(h); i += 2 {
		if h[i].Role != "user" || h[i+1].Role != "model" || h[i+1].Parts[0].Text != "re: "+h[i].Parts[0].Text {
			t.Fatalf("turn %d out of order: %+v / %+v", i/2, h[i], h[i+1])
		}
	}
}

func TestClientDefaultsAndOverrides(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t),
		gogemini.WithSystemInstruction("You are terse."),
		gogemini.WithGenerationConfig(gogemini.GenerationConfig{Temperature: gogemini.Ptr(0.0), MaxOutputTokens: 64}),
	)
	if _, err := c.GenerateContent(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	raw := m.raw[0]
	si, _ := json.Marshal(raw["systemInstruction"])
	gc, _ := json.Marshal(raw["generationConfig"])
	if string(si) != `{"parts":[{"text":"You are terse."}]}` {
		t.Errorf("systemInstruction = %s", si)
	}
	// Temperature 0 must be sent; fields that were not set must not be.
	if string(gc) != `{"maxOutputTokens":64,"temperature":0}` {
		t.Errorf("generationConfig = %s", gc)
	}

	// A request's own values win, and the caller's request is not modified.
	req := &gogemini.GenerateContentRequest{
		Contents:         []gogemini.Content{{Role: "user", Parts: []gogemini.Part{{Text: "b"}}}},
		GenerationConfig: &gogemini.GenerationConfig{TopK: gogemini.Ptr(5)},
	}
	if _, err := c.Generate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	gc, _ = json.Marshal(m.raw[1]["generationConfig"])
	if string(gc) != `{"topK":5}` {
		t.Errorf("per-request generationConfig = %s", gc)
	}
	if req.SystemInstruction != nil {
		t.Error("Generate must not write the client's defaults into the caller's request")
	}
}

func TestWithGenerationConfigCopies(t *testing.T) {
	m := &echoModel{}
	stops := []string{"END"}
	c := newTestClient(t, m.handler(t), gogemini.WithGenerationConfig(gogemini.GenerationConfig{StopSequences: stops}))
	stops[0] = "changed"
	if _, err := c.GenerateContent(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if got := m.bodies[0].GenerationConfig.StopSequences; len(got) != 1 || got[0] != "END" {
		t.Errorf("StopSequences = %q", got)
	}
}

func TestNoDefaultsSent(t *testing.T) {
	m := &echoModel{}
	c := newTestClient(t, m.handler(t), gogemini.WithSystemInstruction("   "))
	if _, err := c.GenerateContent(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"systemInstruction", "generationConfig"} {
		if _, ok := m.raw[0][k]; ok {
			t.Errorf("%s sent without being configured", k)
		}
	}
}
