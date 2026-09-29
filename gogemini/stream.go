package gogemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
)

// GenerateContentStream sends a single user prompt and streams the reply as it is
// generated. Each chunk is a *Response whose Text is the next piece of the answer:
//
//	for chunk, err := range client.GenerateContentStream(ctx, "Tell me a story") {
//		if err != nil {
//			return err
//		}
//		fmt.Print(chunk.Text())
//	}
//
// An empty prompt yields ErrEmptyRequest without sending anything.
func (c *Client) GenerateContentStream(ctx context.Context, prompt string) iter.Seq2[*Response, error] {
	if strings.TrimSpace(prompt) == "" {
		return yieldErr(ErrEmptyRequest)
	}
	return c.GenerateStream(ctx, &GenerateContentRequest{Contents: []Content{userText(prompt)}})
}

// GenerateStream is the streaming form of Generate. Failures before the first chunk
// are retried according to the client's RetryPolicy; once chunks have arrived, an
// error is yielded and the stream ends, since retrying would repeat what was sent.
// Breaking out of the loop closes the connection.
func (c *Client) GenerateStream(ctx context.Context, req *GenerateContentRequest) iter.Seq2[*Response, error] {
	body, err := c.encode(req)
	if err != nil {
		return yieldErr(err)
	}
	return func(yield func(*Response, error) bool) {
		httpResp, err := c.send(ctx, ":streamGenerateContent?alt=sse", body)
		if err != nil {
			yield(nil, err)
			return
		}
		// Close without draining: after a break the server is still sending, and reading
		// the rest would block until it finishes or the client times out.
		defer func() { _ = httpResp.Body.Close() }()

		sc := bufio.NewScanner(httpResp.Body)
		sc.Buffer(make([]byte, 0, 64<<10), maxResponseBody)
		var data bytes.Buffer
		// flush decodes the event collected so far and hands it to the caller.
		flush := func() bool {
			if data.Len() == 0 {
				return true
			}
			defer data.Reset()
			var ev struct {
				Response
				Error *rpcStatus `json:"error"`
			}
			if err := json.Unmarshal(data.Bytes(), &ev); err != nil {
				yield(nil, fmt.Errorf("gogemini: decode stream event: %w", err))
				return false
			}
			if ev.Error != nil {
				yield(nil, ev.Error.apiError(0))
				return false
			}
			r := ev.Response
			return yield(&r, nil)
		}
		for sc.Scan() {
			line := sc.Bytes()
			switch {
			case len(line) == 0: // end of event
				if !flush() {
					return
				}
			case bytes.HasPrefix(line, []byte("data:")):
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.Write(bytes.TrimPrefix(bytes.TrimPrefix(line, []byte("data:")), []byte(" ")))
			}
			// Other SSE fields (event:, id:, retry:, comments) carry nothing we use.
		}
		if err := sc.Err(); err != nil {
			yield(nil, fmt.Errorf("gogemini: read stream: %w", err))
			return
		}
		flush()
	}
}

func yieldErr(err error) iter.Seq2[*Response, error] {
	return func(yield func(*Response, error) bool) { yield(nil, err) }
}
