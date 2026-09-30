package gogemini

import (
	"context"
	"encoding/json"
	"errors"
)

// Tool is a set of functions the model may call. The model does not run them:
// it answers with a FunctionCall, the caller runs the function and sends back a
// FunctionResponse. Chat.AddFunction does that round trip for you.
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// FunctionDeclaration describes a function to the model.
type FunctionDeclaration struct {
	// Name may contain letters, digits, underscores, colons, dots and dashes,
	// up to 128 characters.
	Name        string `json:"name"`
	Description string `json:"description"`
	// Parameters is a JSON Schema of an object whose properties are the
	// function's parameters, as any value encoding/json can marshal.
	// Nil means the function takes no parameters.
	Parameters any `json:"parametersJsonSchema,omitempty"`
}

// ToolConfig configures how the model uses the tools of a request.
type ToolConfig struct {
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// FunctionCallingConfig sets when the model calls functions.
type FunctionCallingConfig struct {
	// Mode is one of the FunctionCalling constants; the API's default is AUTO.
	Mode string `json:"mode,omitempty"`
	// AllowedFunctionNames limits which functions the model calls, with mode ANY or VALIDATED.
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// Modes for FunctionCallingConfig.Mode.
const (
	// FunctionCallingAuto lets the model choose between a call and a text answer.
	FunctionCallingAuto = "AUTO"
	// FunctionCallingAny makes the model always call a function.
	FunctionCallingAny = "ANY"
	// FunctionCallingNone disables function calls, as if no functions were declared.
	FunctionCallingNone = "NONE"
	// FunctionCallingValidated is AUTO with calls checked against the declarations.
	FunctionCallingValidated = "VALIDATED"
)

// FunctionCall is the model asking to run a declared function.
type FunctionCall struct {
	// ID, when set, must be echoed in the matching FunctionResponse.
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	// Args are the arguments as a JSON object; decode them with json.Unmarshal.
	Args json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse carries the result of a FunctionCall back to the model.
type FunctionResponse struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	// Response is the result, as a value that encodes to a JSON object, e.g.
	// map[string]any{"result": …}; use an "error" key to report a failure.
	Response any `json:"response"`
}

// FunctionHandler runs a function the model called, with its arguments as a JSON
// object. The result must encode to a JSON object (a struct or a map); a non-nil
// error is reported to the model as {"error": err.Error()}, and the model decides
// what to do with it. ctx is the one passed to Chat.Send.
type FunctionHandler func(ctx context.Context, args json.RawMessage) (any, error)

// ErrFunctionCallLimit is returned by Chat.Send when the model keeps calling
// functions for more rounds than SetMaxFunctionRounds allows.
var ErrFunctionCallLimit = errors.New("gogemini: too many rounds of function calls")

// defaultMaxFunctionRounds bounds the call → result → call loop of Chat.Send.
const defaultMaxFunctionRounds = 10

// FunctionCallPart returns a part with a function call, e.g. to rebuild a history.
func FunctionCallPart(name string, args json.RawMessage) Part {
	return Part{FunctionCall: &FunctionCall{Name: name, Args: args}}
}

// FunctionResponsePart returns a part with the result of a function call. Pass the
// ID of the FunctionCall it answers, or "" if the call had none.
func FunctionResponsePart(id, name string, response any) Part {
	return Part{FunctionResponse: &FunctionResponse{ID: id, Name: name, Response: response}}
}

// FunctionCalls returns the function calls in the first candidate, in order, or nil.
func (r *Response) FunctionCalls() []FunctionCall {
	if r == nil || len(r.Candidates) == 0 {
		return nil
	}
	var calls []FunctionCall
	for _, p := range r.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			calls = append(calls, *p.FunctionCall)
		}
	}
	return calls
}
