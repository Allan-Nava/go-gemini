package gogemini

import "strings"

// Part is one piece of a message. Only text is supported for now.
type Part struct {
	Text string `json:"text,omitempty"`
}

// Content is a message: a role ("user" or "model") and its parts.
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// GenerationConfig tunes how the model generates. Nil pointers and zero values
// are left out of the request, so the API uses its own defaults; use Ptr for
// fields where zero is a meaningful value, such as a temperature of 0.
type GenerationConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
	MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
	CandidateCount   int      `json:"candidateCount,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMIMEType string   `json:"responseMimeType,omitempty"` // e.g. "application/json"
	Seed             *int64   `json:"seed,omitempty"`
}

// Ptr returns a pointer to v, for the optional fields of GenerationConfig:
//
//	gogemini.GenerationConfig{Temperature: gogemini.Ptr(0.2)}
func Ptr[T any](v T) *T { return &v }

// GenerateContentRequest is the body of a generateContent call.
// SystemInstruction and GenerationConfig, when nil, fall back to the client's
// WithSystemInstruction and WithGenerationConfig defaults.
type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
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
	PromptTokenCount        int `json:"promptTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

// Response is the reply of a generateContent call, or one chunk of a stream.
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

// userText is a single-part user message.
func userText(text string) Content {
	return Content{Role: "user", Parts: []Part{{Text: text}}}
}
