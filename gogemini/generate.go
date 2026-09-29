package gogemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxErrorBody caps how much of an error reply is read.
const maxErrorBody = 1 << 20

// Part is one piece of a message. Only text is supported for now.
type Part struct {
	Text string `json:"text,omitempty"`
}

// Content is a message: a role ("user" or "model") and its parts.
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// GenerateContentRequest is the body of a generateContent call.
type GenerateContentRequest struct {
	Contents []Content `json:"contents"`
}

// Candidate is one generated answer.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
	Index        int     `json:"index"`
}

// PromptFeedback reports why a prompt was blocked, if it was.
type PromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

// UsageMetadata counts the tokens of a call.
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Response is the reply of a generateContent call.
type Response struct {
	Candidates     []Candidate     `json:"candidates"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *UsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
}

// Text returns the text of the first candidate, or "" when there is none
// (for example when the prompt was blocked: see PromptFeedback).
func (r *Response) Text() string {
	if r == nil || len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

// APIError is a non-2xx reply from the Gemini API.
type APIError struct {
	StatusCode int    // HTTP status, e.g. 429
	Status     string // Google status, e.g. "RESOURCE_EXHAUSTED"
	Message    string
}

func (e *APIError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("gogemini: HTTP %d %s: %s", e.StatusCode, e.Status, e.Message)
	}
	return fmt.Sprintf("gogemini: HTTP %d: %s", e.StatusCode, e.Message)
}

// GenerateContent sends a single user prompt and returns the model's reply.
func (c *Client) GenerateContent(ctx context.Context, prompt string) (*Response, error) {
	return c.Generate(ctx, &GenerateContentRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: prompt}}}},
	})
}

// Generate sends a full request, for callers that build the contents themselves.
func (c *Client) Generate(ctx context.Context, req *GenerateContentRequest) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("gogemini: encode request: %w", err)
	}
	endpoint := c.baseURL + "/v1beta/models/" + url.PathEscape(c.model) + ":generateContent"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gogemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The key goes in a header, not in the URL, so it stays out of logs and error messages.
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gogemini: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		return nil, decodeError(httpResp)
	}
	var out Response
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gogemini: decode response: %w", err)
	}
	return &out, nil
}

func decodeError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	apiErr := &APIError{StatusCode: resp.StatusCode}
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Error.Message != "" {
		apiErr.Message, apiErr.Status = wrapped.Error.Message, wrapped.Error.Status
	} else {
		apiErr.Message = strings.TrimSpace(string(raw))
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
	}
	return apiErr
}
