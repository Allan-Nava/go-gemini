package gogemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBody caps how much of a successful reply is read.
const maxResponseBody = 32 << 20

// GenerateContent sends a single user prompt and returns the model's reply.
// An empty or blank prompt returns ErrEmptyRequest without sending anything.
func (c *Client) GenerateContent(ctx context.Context, prompt string) (*Response, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, ErrEmptyRequest
	}
	return c.Generate(ctx, &GenerateContentRequest{Contents: []Content{userText(prompt)}})
}

// Generate sends a full request, for callers that build the contents themselves.
// A nil request or one with no contents returns ErrEmptyRequest without sending anything.
// Transient failures are retried according to the client's RetryPolicy.
func (c *Client) Generate(ctx context.Context, req *GenerateContentRequest) (*Response, error) {
	body, err := c.encode(req)
	if err != nil {
		return nil, err
	}
	httpResp, release, err := c.send(ctx, ":generateContent", body, false)
	if err != nil {
		return nil, err
	}
	defer release()
	defer drainClose(httpResp)

	var out Response
	if err := json.NewDecoder(io.LimitReader(httpResp.Body, maxResponseBody)).Decode(&out); err != nil {
		return nil, fmt.Errorf("gogemini: decode response: %w", err)
	}
	return &out, nil
}

// encode validates req, applies the client's defaults to a copy and marshals it.
func (c *Client) encode(req *GenerateContentRequest) ([]byte, error) {
	if req == nil || len(req.Contents) == 0 {
		return nil, ErrEmptyRequest
	}
	r := *req
	if r.SystemInstruction == nil {
		r.SystemInstruction = c.systemInstruction
	}
	if r.GenerationConfig == nil {
		r.GenerationConfig = c.generationConfig
	}
	body, err := json.Marshal(&r)
	if err != nil {
		return nil, fmt.Errorf("gogemini: encode request: %w", err)
	}
	return body, nil
}

// send POSTs body to the model's method and returns a 2xx response. The caller must
// close its body and then call release. Non-2xx replies become *APIError; transient
// failures, attempt timeouts included, are retried.
//
// Each attempt runs under its own context. For a plain call it has the client's timeout
// and covers reading the reply, which the caller does before release. For a stream the
// timeout only covers the wait for the reply to start: a long stream must not be cut.
func (c *Client) send(ctx context.Context, method string, body []byte, stream bool) (*http.Response, func(), error) {
	var lastErr error
	for attempt := 1; ; attempt++ {
		resp, release, err := c.attempt(ctx, method, body, stream)
		if err == nil {
			return resp, release, nil
		}
		lastErr = err
		if attempt >= c.retry.MaxAttempts || !retryable(ctx, err) {
			return nil, nil, lastErr
		}
		d, ok := c.retry.wait(attempt, serverDelay(err))
		if !ok {
			return nil, nil, lastErr
		}
		if err := sleep(ctx, d); err != nil {
			return nil, nil, fmt.Errorf("gogemini: %w while waiting to retry, after %d attempts: %w", err, attempt, lastErr)
		}
	}
}

// errNotStarted cancels a stream attempt whose reply did not start within the timeout.
var errNotStarted = errors.New("reply did not start in time")

func (c *Client) attempt(ctx context.Context, method string, body []byte, stream bool) (*http.Response, func(), error) {
	var actx context.Context
	var release func()
	var started func() // called once the reply has started
	if stream {
		cctx, cancel := context.WithCancelCause(ctx)
		timer := time.AfterFunc(c.timeout, func() { cancel(errNotStarted) })
		actx, release, started = cctx, func() { cancel(nil) }, func() { timer.Stop() }
	} else {
		tctx, cancel := context.WithTimeout(ctx, c.timeout)
		actx, release, started = tctx, cancel, func() {}
	}
	resp, err := c.sendOnce(actx, method, body)
	started()
	if err != nil {
		// Check before release, which cancels actx too. If our own limit fired and not
		// the caller's, report a timeout, which is retried.
		if ctx.Err() == nil && actx.Err() != nil {
			err = fmt.Errorf("gogemini: no reply within %v: %w", c.timeout, context.DeadlineExceeded)
		}
		release()
		return nil, nil, err
	}
	return resp, release, nil
}

func (c *Client) sendOnce(ctx context.Context, method string, body []byte) (*http.Response, error) {
	endpoint := c.baseURL + "/v1beta/models/" + url.PathEscape(c.model) + method
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gogemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The key goes in a header, not in the URL, so it stays out of logs and error messages.
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	httpReq.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gogemini: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer drainClose(resp)
		return nil, decodeError(resp)
	}
	return resp, nil
}

// drainClose reads what is left of the body, so the connection can be reused, and closes it.
func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))
	_ = resp.Body.Close()
}
