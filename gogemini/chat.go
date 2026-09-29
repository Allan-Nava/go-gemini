package gogemini

import (
	"context"
	"strings"
	"sync"
)

// Chat is a multi-turn conversation: each Send carries the history so far, and a
// successful reply is appended to it. A Chat is safe for concurrent use, but its
// turns are sent one at a time, in the order Send is called.
type Chat struct {
	client  *Client
	mu      sync.Mutex
	history []Content
}

// NewChat starts a conversation, optionally from an earlier history
// (alternating "user" and "model" contents). The history is copied.
func (c *Client) NewChat(history ...Content) *Chat {
	return &Chat{client: c, history: cloneContents(history)}
}

// Send sends text as the next user turn and returns the model's reply. The turn and
// the reply are added to the history only when the model answers; on an error, or a
// blocked prompt with no candidates, the history is left as it was, so Send can be
// called again. An empty text returns ErrEmptyRequest.
func (ch *Chat) Send(ctx context.Context, text string) (*Response, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyRequest
	}
	ch.mu.Lock()
	defer ch.mu.Unlock()

	turn := userText(text)
	contents := append(cloneContents(ch.history), turn)
	resp, err := ch.client.Generate(ctx, &GenerateContentRequest{Contents: contents})
	if err != nil {
		return nil, err
	}
	if len(resp.Candidates) > 0 {
		reply := resp.Candidates[0].Content
		reply.Role = "model"
		reply.Parts = append([]Part(nil), reply.Parts...)
		ch.history = append(ch.history, turn, reply)
	}
	return resp, nil
}

// History returns a copy of the conversation so far.
func (ch *Chat) History() []Content {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return cloneContents(ch.history)
}

func cloneContents(in []Content) []Content {
	if len(in) == 0 {
		return nil
	}
	out := make([]Content, len(in))
	for i, c := range in {
		out[i] = Content{Role: c.Role, Parts: append([]Part(nil), c.Parts...)}
	}
	return out
}
