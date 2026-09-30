package gogemini_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// capture records the raw JSON of each request and answers with reply.
func capture(t *testing.T, reply string, bodies *[]map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		*bodies = append(*bodies, raw)
		io.WriteString(w, reply)
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const okReply = `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]}}]}`

func TestInlineAndFileParts(t *testing.T) {
	var bodies []map[string]any
	c := newTestClient(t, capture(t, okReply, &bodies))
	img := []byte{0x89, 'P', 'N', 'G'}
	_, err := c.Generate(context.Background(), &gogemini.GenerateContentRequest{Contents: []gogemini.Content{
		gogemini.UserContent(
			gogemini.TextPart("describe"),
			gogemini.InlineDataPart("image/png", img),
			gogemini.FileDataPart("application/pdf", "https://example.invalid/files/abc"),
		),
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := jsonOf(t, bodies[0]["contents"])
	want := `[{"parts":[{"text":"describe"},{"inlineData":{"data":"iVBORw==","mimeType":"image/png"}},` +
		`{"fileData":{"fileUri":"https://example.invalid/files/abc","mimeType":"application/pdf"}}],"role":"user"}]`
	if got != want {
		t.Errorf("contents =\n%s\nwant\n%s", got, want)
	}
}

func TestChatSendParts(t *testing.T) {
	var bodies []map[string]any
	c := newTestClient(t, capture(t, okReply, &bodies))
	chat := c.NewChat()
	if _, err := chat.SendParts(context.Background(), gogemini.TextPart("what is this?"), gogemini.InlineDataPart("image/jpeg", []byte("x"))); err != nil {
		t.Fatal(err)
	}
	if h := chat.History(); len(h) != 2 || len(h[0].Parts) != 2 || h[0].Parts[1].InlineData.MIMEType != "image/jpeg" {
		t.Errorf("history = %+v", h)
	}
	if _, err := chat.SendParts(context.Background()); !errors.Is(err, gogemini.ErrEmptyRequest) {
		t.Errorf("no parts: err = %v", err)
	}
}

func TestSafetySettingsAndRatings(t *testing.T) {
	var bodies []map[string]any
	reply := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"safetyRatings":[{"category":"HARM_CATEGORY_HARASSMENT","probability":"LOW"}]}],
		"promptFeedback":{"safetyRatings":[{"category":"HARM_CATEGORY_HATE_SPEECH","probability":"HIGH","blocked":true}]}}`
	settings := []gogemini.SafetySetting{{Category: gogemini.HarmCategoryHarassment, Threshold: gogemini.BlockOnlyHigh}}
	c := newTestClient(t, capture(t, reply, &bodies), gogemini.WithSafetySettings(settings...))
	settings[0].Threshold = "changed by the caller"

	resp, err := c.GenerateContent(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonOf(t, bodies[0]["safetySettings"]); got != `[{"category":"HARM_CATEGORY_HARASSMENT","threshold":"BLOCK_ONLY_HIGH"}]` {
		t.Errorf("safetySettings = %s", got)
	}
	if r := resp.Candidates[0].SafetyRatings; len(r) != 1 || r[0].Probability != "LOW" {
		t.Errorf("candidate ratings = %+v", r)
	}

	// A request's own settings win.
	_, err = c.Generate(context.Background(), &gogemini.GenerateContentRequest{
		Contents:       []gogemini.Content{gogemini.UserContent(gogemini.TextPart("hi"))},
		SafetySettings: []gogemini.SafetySetting{{Category: gogemini.HarmCategoryDangerousContent, Threshold: gogemini.BlockNone}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonOf(t, bodies[1]["safetySettings"]); !strings.Contains(got, "DANGEROUS") || strings.Contains(got, "HARASSMENT") {
		t.Errorf("per-request safetySettings = %s", got)
	}
}

func TestJSONResponseFormat(t *testing.T) {
	var bodies []map[string]any
	c := newTestClient(t, capture(t, `{"candidates":[{"content":{"parts":[{"text":"{\"name\":\"Ada\",\"year\":1815}"}]}}]}`, &bodies))
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}, "year": map[string]any{"type": "integer"}},
		"required":   []string{"name", "year"},
	}
	resp, err := c.Generate(context.Background(), &gogemini.GenerateContentRequest{
		Contents:         []gogemini.Content{gogemini.UserContent(gogemini.TextPart("who wrote the first program?"))},
		GenerationConfig: &gogemini.GenerationConfig{ResponseFormat: gogemini.JSONResponse(schema)},
	})
	if err != nil {
		t.Fatal(err)
	}
	gc := jsonOf(t, bodies[0]["generationConfig"])
	if !strings.HasPrefix(gc, `{"responseFormat":{"text":{"mimeType":"APPLICATION_JSON","schema":{`) {
		t.Errorf("generationConfig = %s", gc)
	}
	var out struct {
		Name string `json:"name"`
		Year int    `json:"year"`
	}
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil || out.Name != "Ada" || out.Year != 1815 {
		t.Errorf("decoded %+v, %v", out, err)
	}
}

func TestThinking(t *testing.T) {
	var bodies []map[string]any
	reply := `{"candidates":[{"content":{"role":"model","parts":[
		{"text":"First I consider the question.","thought":true},
		{"text":"42","thoughtSignature":"c2lnbmF0dXJl"}]}}],"usageMetadata":{"thoughtsTokenCount":12,"totalTokenCount":20}}`
	c := newTestClient(t, capture(t, reply, &bodies), gogemini.WithGenerationConfig(gogemini.GenerationConfig{
		ThinkingConfig: &gogemini.ThinkingConfig{IncludeThoughts: true, ThinkingBudget: gogemini.Ptr(0), ThinkingLevel: gogemini.ThinkingLevelLow},
	}))
	chat := c.NewChat()
	resp, err := chat.Send(context.Background(), "answer?")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "42" || resp.Thoughts() != "First I consider the question." || resp.UsageMetadata.ThoughtsTokenCount != 12 {
		t.Errorf("Text=%q Thoughts=%q usage=%+v", resp.Text(), resp.Thoughts(), resp.UsageMetadata)
	}
	if got := jsonOf(t, bodies[0]["generationConfig"]); got != `{"thinkingConfig":{"includeThoughts":true,"thinkingBudget":0,"thinkingLevel":"LOW"}}` {
		t.Errorf("generationConfig = %s", got)
	}
	// The next turn must carry the thought signature back.
	if _, err := chat.Send(context.Background(), "and?"); err != nil {
		t.Fatal(err)
	}
	if got := jsonOf(t, bodies[1]["contents"]); !strings.Contains(got, `"thoughtSignature":"c2lnbmF0dXJl"`) {
		t.Errorf("second request lost the thought signature: %s", got)
	}
}

func TestWithGenerationConfigCopiesNested(t *testing.T) {
	var bodies []map[string]any
	tc := &gogemini.ThinkingConfig{ThinkingLevel: gogemini.ThinkingLevelHigh}
	rf := gogemini.JSONResponse(map[string]any{"type": "string"})
	c := newTestClient(t, capture(t, okReply, &bodies), gogemini.WithGenerationConfig(gogemini.GenerationConfig{ThinkingConfig: tc, ResponseFormat: rf}))
	tc.ThinkingLevel = "changed"
	rf.Text.MIMEType = "changed"
	if _, err := c.GenerateContent(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if got := jsonOf(t, bodies[0]["generationConfig"]); strings.Contains(got, "changed") {
		t.Errorf("the client's config was changed from outside: %s", got)
	}
}

func TestChatSendStream(t *testing.T) {
	var bodies []map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		bodies = append(bodies, raw)
		for _, p := range []string{"Hel", "lo", " there"} {
			io.WriteString(w, sseChunk(p))
		}
	})
	chat := c.NewChat()
	var got string
	for chunk, err := range chat.SendStream(context.Background(), "hi") {
		if err != nil {
			t.Fatal(err)
		}
		got += chunk.Text()
		_ = chat.History() // must not deadlock inside the loop
	}
	h := chat.History()
	if got != "Hello there" || len(h) != 2 || len(h[1].Parts) != 1 || h[1].Parts[0].Text != "Hello there" || h[1].Role != "model" {
		t.Fatalf("text=%q history=%+v", got, h)
	}
	// The next turn carries the merged reply.
	for _, err := range chat.SendStream(context.Background(), "again") {
		if err != nil {
			t.Fatal(err)
		}
	}
	if contents := jsonOf(t, bodies[1]["contents"]); !strings.Contains(contents, `"text":"Hello there"`) {
		t.Errorf("second request contents = %s", contents)
	}
}

func TestChatSendStreamKeepsHistoryOnBreakOrError(t *testing.T) {
	var fail atomic.Bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, sseChunk("part"))
		if fail.Load() {
			io.WriteString(w, `data: {"error":{"code":500,"message":"boom","status":"INTERNAL"}}`+"\n\n")
		}
	})
	chat := c.NewChat()
	for range chat.SendStream(context.Background(), "hi") {
		break
	}
	fail.Store(true)
	var sawErr bool
	for _, err := range chat.SendStream(context.Background(), "hi") {
		if err != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("want the stream error")
	}
	if h := chat.History(); len(h) != 0 {
		t.Errorf("history = %+v, want it untouched after a break and an error", h)
	}
	for _, err := range chat.SendStream(context.Background(), " ") {
		if !errors.Is(err, gogemini.ErrEmptyRequest) {
			t.Errorf("empty text: err = %v", err)
		}
	}
}

func TestChatStreamAndSendDoNotInterleave(t *testing.T) {
	var mu sync.Mutex
	var sizes []int // number of contents in each request
	started := make(chan struct{})
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req gogemini.GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		mu.Lock()
		sizes = append(sizes, len(req.Contents))
		mu.Unlock()
		if strings.Contains(r.URL.RawQuery, "alt=sse") {
			io.WriteString(w, sseChunk("streamed"))
			w.(http.Flusher).Flush()
			close(started)
			time.Sleep(100 * time.Millisecond) // the stream is still open while Send is called
			return
		}
		io.WriteString(w, okReply)
	})
	chat := c.NewChat()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, err := range chat.SendStream(context.Background(), "first") {
			if err != nil {
				t.Error(err)
			}
		}
	}()
	<-started
	if _, err := chat.Send(context.Background(), "second"); err != nil {
		t.Fatal(err)
	}
	<-done
	// Send must wait for the stream's turn to finish, then carry it: 1 content, then 3.
	if fmt.Sprint(sizes) != "[1 3]" {
		t.Fatalf("contents per request = %v, want [1 3]", sizes)
	}
	if h := chat.History(); len(h) != 4 || h[0].Parts[0].Text != "first" || h[2].Parts[0].Text != "second" {
		t.Fatalf("history = %+v", h)
	}
}

func TestCountTokens(t *testing.T) {
	var bodies []map[string]any
	var path string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		capture(t, `{"totalTokens":31,"cachedContentTokenCount":4}`, &bodies)(w, r)
	}, gogemini.WithModel("gemini-test"), gogemini.WithSystemInstruction("Be brief."))
	resp, err := c.CountTokens(context.Background(), &gogemini.GenerateContentRequest{
		Contents: []gogemini.Content{gogemini.UserContent(gogemini.TextPart("count me"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalTokens != 31 || resp.CachedContentTokenCount != 4 {
		t.Errorf("resp = %+v", resp)
	}
	if path != "/v1beta/models/gemini-test:countTokens" {
		t.Errorf("path = %s", path)
	}
	gcr, _ := bodies[0]["generateContentRequest"].(map[string]any)
	if gcr["model"] != "models/gemini-test" || !strings.Contains(jsonOf(t, gcr["systemInstruction"]), "Be brief.") || gcr["contents"] == nil {
		t.Errorf("generateContentRequest = %s", jsonOf(t, gcr))
	}
	if _, err := c.CountTokens(context.Background(), nil); !errors.Is(err, gogemini.ErrEmptyRequest) {
		t.Errorf("nil request: err = %v", err)
	}
}

func TestCountTokensIsRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"totalTokens":3}`)
	}))
	defer srv.Close()
	resp, err := retryClient(t, srv.URL, fastRetry).CountTokens(context.Background(), &gogemini.GenerateContentRequest{
		Contents: []gogemini.Content{gogemini.UserContent(gogemini.TextPart("x"))},
	})
	if err != nil || resp.TotalTokens != 3 || hits.Load() != 2 {
		t.Fatalf("resp=%v err=%v attempts=%d", resp, err, hits.Load())
	}
}
