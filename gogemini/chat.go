package gogemini

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Chat is a multi-turn conversation: each Send carries the history so far, and a
// successful reply is appended to it, thought signatures included. A Chat is safe
// for concurrent use; its turns are sent one at a time, in the order they start.
type Chat struct {
	client *Client
	turn   sync.Mutex // held for a whole turn, so turns do not interleave
	mu     sync.Mutex // guards the fields below; never held while waiting on the network
	// history is only appended to, never modified in place.
	history   []Content
	functions map[string]chatFunction
	maxRounds int
}

type chatFunction struct {
	decl    FunctionDeclaration
	handler FunctionHandler
}

// AddFunction declares a function the model may call in this chat, and the Go
// function that runs it. Send then handles calls on its own: it runs the handlers,
// sends their results back and returns the model's final answer. A later call with
// the same name replaces it; a nil handler removes it. SendStream does not declare
// or run functions.
func (ch *Chat) AddFunction(decl FunctionDeclaration, handler FunctionHandler) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if handler == nil {
		delete(ch.functions, decl.Name)
		return
	}
	if ch.functions == nil {
		ch.functions = map[string]chatFunction{}
	}
	ch.functions[decl.Name] = chatFunction{decl: decl, handler: handler}
}

// SetMaxFunctionRounds sets how many rounds of function calls one Send may go through
// before it gives up with ErrFunctionCallLimit. The default is 10; n below 1 means 1.
func (ch *Chat) SetMaxFunctionRounds(n int) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.maxRounds = max(n, 1)
}

// NewChat starts a conversation, optionally from an earlier history
// (alternating "user" and "model" contents). The history is copied; the bytes of
// inline data are shared, not copied.
func (c *Client) NewChat(history ...Content) *Chat {
	return &Chat{client: c, history: cloneContents(history)}
}

// Send sends text as the next user turn and returns the model's reply. The turn and
// the reply are added to the history only when the model answers; on an error, or a
// blocked prompt with no candidates, the history is left as it was, so Send can be
// called again. An empty text returns ErrEmptyRequest.
//
// With functions added by AddFunction, the reply may be a request to call them. Send
// then runs them in order, sends the results and repeats until the model answers,
// and returns that answer; the calls and results go into the history with it. If a
// round fails, nothing of the turn is kept.
func (ch *Chat) Send(ctx context.Context, text string) (*Response, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyRequest
	}
	return ch.SendParts(ctx, TextPart(text))
}

// SendParts is Send for a turn made of parts, e.g. a question and an image:
//
//	chat.SendParts(ctx, gogemini.TextPart("What is in this picture?"), gogemini.InlineDataPart("image/png", img))
//
// No parts returns ErrEmptyRequest.
func (ch *Chat) SendParts(ctx context.Context, parts ...Part) (*Response, error) {
	if len(parts) == 0 {
		return nil, ErrEmptyRequest
	}
	ch.turn.Lock()
	defer ch.turn.Unlock()

	functions, tools, maxRounds := ch.functionSet()
	history := ch.contentsWith()
	pending := []Content{UserContent(slices.Clone(parts)...)} // this turn, not yet in the history
	for round := 0; ; round++ {
		req := &GenerateContentRequest{Contents: append(slices.Clone(history), pending...), Tools: tools}
		resp, err := ch.client.Generate(ctx, req)
		if err != nil {
			return nil, err
		}
		if len(resp.Candidates) == 0 { // blocked: keep nothing, as for any failed turn
			return resp, nil
		}
		reply := Content{Role: "model", Parts: slices.Clone(resp.Candidates[0].Content.Parts)}
		pending = append(pending, reply)
		calls := resp.FunctionCalls()
		if len(calls) == 0 || len(functions) == 0 {
			ch.append(pending...)
			return resp, nil
		}
		if round >= maxRounds {
			return nil, fmt.Errorf("%w (%d)", ErrFunctionCallLimit, maxRounds)
		}
		pending = append(pending, runFunctions(ctx, functions, calls))
	}
}

// functionSet returns a snapshot of the chat's functions, their declarations for a
// request (sorted by name, so requests are stable), and the round limit.
func (ch *Chat) functionSet() (map[string]chatFunction, []Tool, int) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	maxRounds := ch.maxRounds
	if maxRounds == 0 {
		maxRounds = defaultMaxFunctionRounds
	}
	if len(ch.functions) == 0 {
		return nil, nil, maxRounds
	}
	functions := make(map[string]chatFunction, len(ch.functions))
	decls := make([]FunctionDeclaration, 0, len(ch.functions))
	for name, f := range ch.functions {
		functions[name] = f
		decls = append(decls, f.decl)
	}
	sort.Slice(decls, func(i, j int) bool { return decls[i].Name < decls[j].Name })
	return functions, []Tool{{FunctionDeclarations: decls}}, maxRounds
}

// runFunctions runs the calls in order and returns the user turn with their results.
// Failures are reported to the model rather than returned: it can retry or explain.
func runFunctions(ctx context.Context, functions map[string]chatFunction, calls []FunctionCall) Content {
	results := make([]Part, 0, len(calls))
	for _, call := range calls {
		var response any
		if f, ok := functions[call.Name]; !ok {
			response = map[string]string{"error": "unknown function " + call.Name}
		} else {
			args := call.Args
			if len(args) == 0 {
				args = json.RawMessage("{}")
			}
			out, err := f.handler(ctx, args)
			if err != nil {
				response = map[string]string{"error": err.Error()}
			} else {
				response = asObject(out)
			}
		}
		results = append(results, FunctionResponsePart(call.ID, call.Name, response))
	}
	return UserContent(results...)
}

// asObject makes v encode to a JSON object, as the API requires of a function
// response: a result that is not one is wrapped as {"result": v}.
func asObject(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]string{"error": "cannot encode the function's result: " + err.Error()}
	}
	if len(b) > 0 && b[0] == '{' {
		return json.RawMessage(b)
	}
	return map[string]json.RawMessage{"result": b}
}

// SendStream is Send with the reply streamed as it is generated. When the stream
// ends, the whole reply is added to the history; on an error, or if the loop is
// left early, the history is left as it was. Calling History inside the loop is
// fine; calling Send or SendStream on the same Chat there would wait forever.
func (ch *Chat) SendStream(ctx context.Context, text string) iter.Seq2[*Response, error] {
	if strings.TrimSpace(text) == "" {
		return yieldErr(ErrEmptyRequest)
	}
	return func(yield func(*Response, error) bool) {
		ch.turn.Lock()
		defer ch.turn.Unlock()

		turn := userText(text)
		var parts []Part
		for chunk, err := range ch.client.GenerateStream(ctx, &GenerateContentRequest{Contents: ch.contentsWith(turn)}) {
			if err != nil {
				yield(nil, err)
				return
			}
			if len(chunk.Candidates) > 0 {
				parts = append(parts, chunk.Candidates[0].Content.Parts...)
			}
			if !yield(chunk, nil) {
				return
			}
		}
		if len(parts) > 0 {
			ch.append(turn, Content{Role: "model", Parts: mergeText(parts)})
		}
	}
}

// History returns a copy of the conversation so far.
func (ch *Chat) History() []Content {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return cloneContents(ch.history)
}

// contentsWith returns the history followed by turns, for a request. The contents are
// shared with the history, which is never modified in place, so no deep copy is needed.
func (ch *Chat) contentsWith(turns ...Content) []Content {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return append(slices.Clone(ch.history), turns...)
}

func (ch *Chat) append(turns ...Content) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.history = append(ch.history, turns...)
}

// mergeText joins consecutive plain text parts of a streamed reply into one, so the
// history holds the answer rather than one part per chunk. Parts carrying a thought,
// a signature or data are kept as they are.
func mergeText(parts []Part) []Part {
	plain := func(p Part) bool {
		return !p.Thought && p.ThoughtSignature == "" && p.InlineData == nil && p.FileData == nil &&
			p.FunctionCall == nil && p.FunctionResponse == nil
	}
	var out []Part
	for _, p := range parts {
		if n := len(out); n > 0 && plain(p) && plain(out[n-1]) {
			out[n-1].Text += p.Text
			continue
		}
		out = append(out, p)
	}
	return out
}

func cloneContents(in []Content) []Content {
	if len(in) == 0 {
		return nil
	}
	out := make([]Content, len(in))
	for i, c := range in {
		out[i] = Content{Role: c.Role, Parts: slices.Clone(c.Parts)}
	}
	return out
}
