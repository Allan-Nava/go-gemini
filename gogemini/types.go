package gogemini

import "strings"

// Part is one piece of a message: text, inline bytes (an image, a PDF…) or a
// reference to a file. Set one of Text, InlineData or FileData; TextPart,
// InlineDataPart and FileDataPart build them.
type Part struct {
	Text       string    `json:"text,omitempty"`
	InlineData *Blob     `json:"inlineData,omitempty"`
	FileData   *FileData `json:"fileData,omitempty"`
	// Thought marks a part of the model's reasoning, returned when
	// ThinkingConfig.IncludeThoughts is set. Response.Text leaves these out.
	Thought bool `json:"thought,omitempty"`
	// ThoughtSignature is an opaque token of the model's reasoning. Send it back
	// unchanged in later turns (Chat does this); dropping it can end a reply with
	// finish reason MISSING_THOUGHT_SIGNATURE.
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
}

// Blob is inline media. Data is sent base64-encoded.
type Blob struct {
	MIMEType string `json:"mimeType"` // e.g. "image/png", "application/pdf"
	Data     []byte `json:"data"`
}

// FileData refers to a file by URI, such as one uploaded through the Files API.
type FileData struct {
	MIMEType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri"`
}

// TextPart returns a text part.
func TextPart(text string) Part { return Part{Text: text} }

// InlineDataPart returns a part carrying data inline, e.g. an image read from disk.
// The request grows by about 4/3 of len(data); for large files use a file reference.
func InlineDataPart(mimeType string, data []byte) Part {
	return Part{InlineData: &Blob{MIMEType: mimeType, Data: data}}
}

// FileDataPart returns a part that refers to a file by URI.
func FileDataPart(mimeType, uri string) Part {
	return Part{FileData: &FileData{MIMEType: mimeType, FileURI: uri}}
}

// UserContent returns a user turn made of parts, for Generate and GenerateStream.
func UserContent(parts ...Part) Content { return Content{Role: "user", Parts: parts} }

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
	// ResponseFormat asks for output in a given format, e.g. JSON matching a
	// schema: see JSONResponse.
	ResponseFormat *ResponseFormat `json:"responseFormat,omitempty"`
	// ThinkingConfig controls the reasoning of models that think before answering.
	ThinkingConfig *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

// ResponseFormat configures the format of the output.
type ResponseFormat struct {
	Text *TextFormat `json:"text,omitempty"`
}

// TextFormat configures text output: its MIME type ("APPLICATION_JSON" or
// "TEXT_PLAIN") and, for JSON, the JSON Schema the output must conform to.
type TextFormat struct {
	MIMEType string `json:"mimeType,omitempty"`
	Schema   any    `json:"schema,omitempty"`
}

// JSONResponse returns a ResponseFormat for JSON output that conforms to schema,
// a JSON Schema given as any value encoding/json can marshal (a map, a struct,
// or json.RawMessage). Decode Response.Text with encoding/json.
func JSONResponse(schema any) *ResponseFormat {
	return &ResponseFormat{Text: &TextFormat{MIMEType: "APPLICATION_JSON", Schema: schema}}
}

// ThinkingConfig controls the reasoning of models that think before answering.
// Setting it for a model that does not think makes the API return an error.
type ThinkingConfig struct {
	// IncludeThoughts returns a summary of the reasoning as parts with Thought set;
	// read them with Response.Thoughts.
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
	// ThinkingBudget is the number of tokens the model may spend thinking.
	ThinkingBudget *int `json:"thinkingBudget,omitempty"`
	// ThinkingLevel is how much to think: one of the ThinkingLevel constants.
	// Newer models use it instead of ThinkingBudget.
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

// Thinking levels for ThinkingConfig.ThinkingLevel.
const (
	ThinkingLevelMinimal = "MINIMAL"
	ThinkingLevelLow     = "LOW"
	ThinkingLevelMedium  = "MEDIUM"
	ThinkingLevelHigh    = "HIGH"
)

// SafetySetting sets the threshold at which content of a harm category is blocked.
type SafetySetting struct {
	Category  string `json:"category"`  // e.g. HarmCategoryHarassment
	Threshold string `json:"threshold"` // e.g. BlockOnlyHigh
}

// SafetyRating is how likely a piece of content is to belong to a harm category.
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"` // e.g. "NEGLIGIBLE", "LOW", "MEDIUM", "HIGH"
	Blocked     bool   `json:"blocked,omitempty"`
}

// Harm categories for SafetySetting.Category.
const (
	HarmCategoryHarassment       = "HARM_CATEGORY_HARASSMENT"
	HarmCategoryHateSpeech       = "HARM_CATEGORY_HATE_SPEECH"
	HarmCategorySexuallyExplicit = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	HarmCategoryDangerousContent = "HARM_CATEGORY_DANGEROUS_CONTENT"
	HarmCategoryCivicIntegrity   = "HARM_CATEGORY_CIVIC_INTEGRITY"
)

// Thresholds for SafetySetting.Threshold.
const (
	BlockLowAndAbove    = "BLOCK_LOW_AND_ABOVE"
	BlockMediumAndAbove = "BLOCK_MEDIUM_AND_ABOVE"
	BlockOnlyHigh       = "BLOCK_ONLY_HIGH"
	BlockNone           = "BLOCK_NONE"
)

// Ptr returns a pointer to v, for the optional fields of GenerationConfig:
//
//	gogemini.GenerationConfig{Temperature: gogemini.Ptr(0.2)}
func Ptr[T any](v T) *T { return &v }

// GenerateContentRequest is the body of a generateContent call.
// SystemInstruction, GenerationConfig and SafetySettings, when unset, fall back
// to the client's WithSystemInstruction, WithGenerationConfig and
// WithSafetySettings defaults.
type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings    []SafetySetting   `json:"safetySettings,omitempty"`
}

// Candidate is one generated answer.
type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  string         `json:"finishReason,omitempty"`
	Index         int            `json:"index"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

// PromptFeedback reports why a prompt was blocked, if it was.
//
// It has no SafetyRatings field, although the API sends one: a slice would make
// PromptFeedback incomparable, an incompatible change in 1.x. Candidates carry
// their own SafetyRatings.
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

// Text returns the text of the first candidate, without the parts of its
// reasoning, or "" when there is none (for example when the prompt was
// blocked: see PromptFeedback).
func (r *Response) Text() string { return r.text(false) }

// Thoughts returns the reasoning summary of the first candidate, which the
// model sends when ThinkingConfig.IncludeThoughts is set, or "".
func (r *Response) Thoughts() string { return r.text(true) }

func (r *Response) text(thoughts bool) string {
	if r == nil || len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		if p.Thought == thoughts {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// CountTokensResponse is the reply of CountTokens.
type CountTokensResponse struct {
	TotalTokens             int `json:"totalTokens"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

// userText is a single-part user message.
func userText(text string) Content { return UserContent(TextPart(text)) }
