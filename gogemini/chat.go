package gogemini

import (
	"context"
	"iter"
	"slices"
	"strings"
	"sync"
)

// Chat is a multi-turn conversation: each Send carries the history so far, and a
// successful reply is appended to it, thought signatures included. A Chat is safe
// for concurrent use; its turns are sent one at a time, in the order they start.
type Chat struct {
	client *Client
	turn   sync.Mutex // held for a whole turn, so turns do not interleave
	mu     sync.Mutex // guards history; never held while waiting on the network
	// history is only appended to, never modified in place.
	history []Content
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

	turn := UserContent(slices.Clone(parts)...)
	resp, err := ch.client.Generate(ctx, &GenerateContentRequest{Contents: ch.contentsWith(turn)})
	if err != nil {
		return nil, err
	}
	if len(resp.Candidates) > 0 {
		reply := resp.Candidates[0].Content
		ch.append(turn, Content{Role: "model", Parts: slices.Clone(reply.Parts)})
	}
	return resp, nil
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

// contentsWith returns the history followed by turn, for a request. The contents are
// shared with the history, which is never modified in place, so no deep copy is needed.
func (ch *Chat) contentsWith(turn Content) []Content {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return append(slices.Clone(ch.history), turn)
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
		return !p.Thought && p.ThoughtSignature == "" && p.InlineData == nil && p.FileData == nil
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
