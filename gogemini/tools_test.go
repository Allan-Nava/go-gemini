package gogemini_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// replyServerError makes scripted answer that request with a 500.
const replyServerError = "500"

// scripted answers each request with the next reply and records the requests.
type scripted struct {
	mu       sync.Mutex
	replies  []string
	requests []gogemini.GenerateContentRequest
	raw      []string
}

func (s *scripted) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var req gogemini.GenerateContentRequest
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("bad request body: %s", data)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests, s.raw = append(s.requests, req), append(s.raw, string(data))
		if len(s.replies) == 0 {
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		reply := s.replies[0]
		s.replies = s.replies[1:]
		if reply == replyServerError {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error":{"code":500,"message":"boom","status":"INTERNAL"}}`)
			return
		}
		io.WriteString(w, reply)
	}
}

const (
	callWeather = `{"candidates":[{"content":{"role":"model","parts":[
		{"functionCall":{"id":"call-1","name":"get_weather","args":{"city":"Rome"}},"thoughtSignature":"c2ln"}]}}]}`
	finalAnswer = `{"candidates":[{"content":{"role":"model","parts":[{"text":"It is sunny in Rome, 24°C."}]}}]}`
)

var weatherDecl = gogemini.FunctionDeclaration{
	Name:        "get_weather",
	Description: "Current weather in a city.",
	Parameters: map[string]any{
		"type":       "object",
		"properties": map[string]any{"city": map[string]any{"type": "string"}},
		"required":   []string{"city"},
	},
}

func TestChatRunsFunctions(t *testing.T) {
	s := &scripted{replies: []string{callWeather, finalAnswer}}
	c := newTestClient(t, s.handler(t))
	chat := c.NewChat()
	type ctxKey struct{}
	var gotCity string
	var gotCtx any
	chat.AddFunction(weatherDecl, func(ctx context.Context, args json.RawMessage) (any, error) {
		var p struct {
			City string `json:"city"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		gotCity, gotCtx = p.City, ctx.Value(ctxKey{})
		return map[string]any{"condition": "sunny", "celsius": 24}, nil
	})

	ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
	resp, err := chat.Send(ctx, "Weather in Rome?")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "It is sunny in Rome, 24°C." || gotCity != "Rome" || gotCtx != "caller" {
		t.Fatalf("text=%q city=%q ctx=%v", resp.Text(), gotCity, gotCtx)
	}

	// The declaration goes with every request.
	if !strings.Contains(s.raw[0], `"tools":[{"functionDeclarations":[{"name":"get_weather","description":"Current weather in a city.","parametersJsonSchema":{`) {
		t.Errorf("first request = %s", s.raw[0])
	}
	// The second request carries the call, with its thought signature, and the result.
	second := s.requests[1].Contents
	if len(second) != 3 || second[1].Role != "model" || second[1].Parts[0].FunctionCall.Name != "get_weather" ||
		second[1].Parts[0].ThoughtSignature != "c2ln" || second[2].Role != "user" {
		t.Fatalf("second request contents = %+v", second)
	}
	fr := second[2].Parts[0].FunctionResponse
	if fr == nil || fr.ID != "call-1" || fr.Name != "get_weather" {
		t.Fatalf("function response = %+v", fr)
	}
	if !strings.Contains(s.raw[1], `"functionResponse":{"id":"call-1","name":"get_weather","response":{"celsius":24,"condition":"sunny"}}`) {
		t.Errorf("second request = %s", s.raw[1])
	}
	// History: question, call, result, answer.
	if h := chat.History(); len(h) != 4 || h[3].Parts[0].Text != "It is sunny in Rome, 24°C." {
		t.Fatalf("history = %+v", h)
	}
}

func TestFunctionErrorsGoToTheModel(t *testing.T) {
	twoCalls := `{"candidates":[{"content":{"role":"model","parts":[
		{"functionCall":{"name":"get_weather","args":{"city":"Atlantis"}}},
		{"functionCall":{"name":"launch_rocket"}},
		{"functionCall":{"name":"now"}}]}}]}`
	s := &scripted{replies: []string{twoCalls, finalAnswer}}
	c := newTestClient(t, s.handler(t))
	chat := c.NewChat()
	chat.AddFunction(weatherDecl, func(ctx context.Context, args json.RawMessage) (any, error) {
		return nil, errors.New("no such city")
	})
	var nowArgs string
	chat.AddFunction(gogemini.FunctionDeclaration{Name: "now", Description: "The time."}, func(ctx context.Context, args json.RawMessage) (any, error) {
		nowArgs = string(args)
		return "12:00", nil // not an object: must be wrapped
	})
	if _, err := chat.Send(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"functionResponse":{"name":"get_weather","response":{"error":"no such city"}}}`,
		`{"functionResponse":{"name":"launch_rocket","response":{"error":"unknown function launch_rocket"}}}`,
		`{"functionResponse":{"name":"now","response":{"result":"12:00"}}}`,
	} {
		if !strings.Contains(s.raw[1], want) {
			t.Errorf("second request lacks %s\nin %s", want, s.raw[1])
		}
	}
	if nowArgs != "{}" {
		t.Errorf("a call without args must get {}, got %q", nowArgs)
	}
	// Results come back in the order of the calls.
	parts := s.requests[1].Contents[2].Parts
	if len(parts) != 3 || parts[0].FunctionResponse.Name != "get_weather" || parts[2].FunctionResponse.Name != "now" {
		t.Errorf("results = %+v", parts)
	}
}

func TestFunctionRoundLimit(t *testing.T) {
	s := &scripted{replies: []string{callWeather, callWeather, callWeather}}
	c := newTestClient(t, s.handler(t))
	chat := c.NewChat()
	chat.AddFunction(weatherDecl, func(context.Context, json.RawMessage) (any, error) { return map[string]any{}, nil })
	chat.SetMaxFunctionRounds(2)
	_, err := chat.Send(context.Background(), "loop")
	if !errors.Is(err, gogemini.ErrFunctionCallLimit) {
		t.Fatalf("err = %v, want ErrFunctionCallLimit", err)
	}
	if len(s.requests) != 3 {
		t.Errorf("requests = %d, want 3 (the first and 2 rounds)", len(s.requests))
	}
	if h := chat.History(); len(h) != 0 {
		t.Errorf("history = %+v, want nothing kept from a failed turn", h)
	}
}

func TestFunctionTurnFailingMidwayKeepsNothing(t *testing.T) {
	s := &scripted{replies: []string{callWeather, replyServerError}}
	c := newTestClient(t, s.handler(t))
	chat := c.NewChat()
	chat.AddFunction(weatherDecl, func(context.Context, json.RawMessage) (any, error) { return map[string]any{}, nil })
	if _, err := chat.Send(context.Background(), "x"); err == nil {
		t.Fatal("want the error of the second round")
	}
	if h := chat.History(); len(h) != 0 {
		t.Errorf("history = %+v", h)
	}
}

func TestRemovedFunctionIsNotDeclared(t *testing.T) {
	s := &scripted{replies: []string{finalAnswer}}
	c := newTestClient(t, s.handler(t))
	chat := c.NewChat()
	chat.AddFunction(weatherDecl, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	chat.AddFunction(weatherDecl, nil)
	if _, err := chat.Send(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.raw[0], "tools") {
		t.Errorf("request declares removed function: %s", s.raw[0])
	}
}

func TestToolsAndToolConfigJSON(t *testing.T) {
	var bodies []map[string]any
	c := newTestClient(t, capture(t, callWeather, &bodies))
	resp, err := c.Generate(context.Background(), &gogemini.GenerateContentRequest{
		Contents: []gogemini.Content{gogemini.UserContent(gogemini.TextPart("x"))},
		Tools:    []gogemini.Tool{{FunctionDeclarations: []gogemini.FunctionDeclaration{{Name: "f", Description: "d"}}}},
		ToolConfig: &gogemini.ToolConfig{FunctionCallingConfig: &gogemini.FunctionCallingConfig{
			Mode: gogemini.FunctionCallingAny, AllowedFunctionNames: []string{"f"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonOf(t, bodies[0]["tools"]); got != `[{"functionDeclarations":[{"description":"d","name":"f"}]}]` {
		t.Errorf("tools = %s", got)
	}
	if got := jsonOf(t, bodies[0]["toolConfig"]); got != `{"functionCallingConfig":{"allowedFunctionNames":["f"],"mode":"ANY"}}` {
		t.Errorf("toolConfig = %s", got)
	}
	calls := resp.FunctionCalls()
	if len(calls) != 1 || calls[0].ID != "call-1" || string(calls[0].Args) != `{"city":"Rome"}` {
		t.Errorf("FunctionCalls() = %+v", calls)
	}
	var nilResp *gogemini.Response
	if nilResp.FunctionCalls() != nil {
		t.Error("FunctionCalls on nil must be nil")
	}
}

func TestFunctionPartsJSON(t *testing.T) {
	p := gogemini.FunctionCallPart("f", json.RawMessage(`{"a":1}`))
	r := gogemini.FunctionResponsePart("id-7", "f", map[string]int{"b": 2})
	if got := jsonOf(t, []gogemini.Part{p, r}); got != `[{"functionCall":{"name":"f","args":{"a":1}}},{"functionResponse":{"id":"id-7","name":"f","response":{"b":2}}}]` {
		t.Errorf("parts = %s", got)
	}
}

func TestSendStreamDoesNotDeclareFunctions(t *testing.T) {
	var raw string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		io.WriteString(w, sseChunk("hi"))
	})
	chat := c.NewChat()
	chat.AddFunction(weatherDecl, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	for _, err := range chat.SendStream(context.Background(), "x") {
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(raw, "tools") {
		t.Errorf("SendStream declared functions: %s", raw)
	}
}
