# go-gemini
[![Go build](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml)
[![Go test workflow](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Allan-Nava/go-gemini/gogemini.svg)](https://pkg.go.dev/github.com/Allan-Nava/go-gemini/gogemini)

A small Go client for the [official Gemini API](https://ai.google.dev/api). Standard library only:
the module has no dependencies. Latest release: **v1.0.0** ([changelog](CHANGELOG.md)), the first
stable one: the API is frozen and 1.x releases only add to it.

Site: https://allan-nava.github.io/go-gemini/ · Status and priorities:
[latest audit](docs/audit-v0.2.0-2026-09-29.md) · [Milestones](docs/milestone.md) ·
[Changelog](CHANGELOG.md)

## Install

```bash
go get github.com/Allan-Nava/go-gemini
```

Go 1.26 or newer.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

func main() {
	client, err := gogemini.New() // reads GEMINI_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.GenerateContent(context.Background(), "Explain goroutines in one sentence")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Text())
}
```

Options for `gogemini.New`:

| Option | Default | Meaning |
|---|---|---|
| `WithAPIKey(key)` | `$GEMINI_API_KEY` | API key; wins over the environment |
| `WithModel(name)` | `gemini-3.8-flash`, may change in a minor release | model; a `models/` prefix is accepted; read it with `Client.Model()` |
| `WithTimeout(d)` | 5 min | limit for one attempt: the whole reply for `Generate`, the start for a stream; must be positive |
| `WithHTTPClient(c)` | — | your own `*http.Client` (transport, TLS, timeouts) |
| `WithBaseURL(u)` | `https://generativelanguage.googleapis.com` | proxy or test server; HTTPS only, plain HTTP allowed for loopback |

`GenerateContent(ctx, prompt)` sends one user prompt; `Generate(ctx, req)` takes a full
`GenerateContentRequest`. `Response.Text()` returns the first candidate's text, and is empty when
the prompt was blocked (`Response.PromptFeedback.BlockReason`).

### Streaming

```go
for chunk, err := range client.GenerateContentStream(ctx, "Tell me a short story") {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text())
}
```

`GenerateStream(ctx, req)` is the same with a full request. Breaking out of the loop closes the
connection. Failures before the first chunk are retried; after that the error is yielded and the
stream ends.

### Chat

```go
chat := client.NewChat()
resp, err := chat.Send(ctx, "What are goroutines?")
resp, err = chat.Send(ctx, "And channels?") // carries the first question and answer
history := chat.History()
```

A turn is kept only when the model answers, so a failed `Send` can simply be called again.
`NewChat(history...)` resumes an earlier conversation. `chat.SendStream(ctx, text)` streams the reply
and adds it to the history when the stream ends; `chat.SendParts` sends images or files.

### Generation parameters and system instruction

```go
client, err := gogemini.New(
	gogemini.WithSystemInstruction("Answer in one short sentence."),
	gogemini.WithGenerationConfig(gogemini.GenerationConfig{
		Temperature:     gogemini.Ptr(0.2),
		MaxOutputTokens: 256,
	}),
)
```

These are defaults: a request's own `SystemInstruction` or `GenerationConfig` wins. `GenerationConfig`
also has `TopP`, `TopK`, `CandidateCount`, `StopSequences`, `ResponseMIMEType` and `Seed`; use
`gogemini.Ptr` where zero is a meaningful value.

### Images and files

```go
img, _ := os.ReadFile("photo.png")
resp, err := client.Generate(ctx, &gogemini.GenerateContentRequest{
	Contents: []gogemini.Content{gogemini.UserContent(
		gogemini.TextPart("What is in this picture?"),
		gogemini.InlineDataPart("image/png", img),
	)},
})
```

`FileDataPart(mimeType, uri)` refers to a file by URI instead. In a chat, `chat.SendParts(ctx, parts...)`
sends a turn made of parts.

### JSON output with a schema

```go
schema := map[string]any{
	"type":       "object",
	"properties": map[string]any{"name": map[string]any{"type": "string"}},
	"required":   []string{"name"},
}
resp, err := client.Generate(ctx, &gogemini.GenerateContentRequest{
	Contents:         []gogemini.Content{gogemini.UserContent(gogemini.TextPart("Who wrote the first program?"))},
	GenerationConfig: &gogemini.GenerationConfig{ResponseFormat: gogemini.JSONResponse(schema)},
})
// json.Unmarshal([]byte(resp.Text()), &out)
```

### Safety settings

```go
client, err := gogemini.New(gogemini.WithSafetySettings(
	gogemini.SafetySetting{Category: gogemini.HarmCategoryHarassment, Threshold: gogemini.BlockOnlyHigh},
))
```

A request's own `SafetySettings` win. Each `Candidate` carries its `SafetyRatings`.

### Thinking

```go
cfg := gogemini.GenerationConfig{ThinkingConfig: &gogemini.ThinkingConfig{
	ThinkingLevel:   gogemini.ThinkingLevelLow,
	IncludeThoughts: true,
}}
```

`resp.Text()` leaves the reasoning out; `resp.Thoughts()` returns its summary. A `Chat` keeps the
parts' `ThoughtSignature` and sends it back in later turns, as newer models require.

### Counting tokens

```go
n, err := client.CountTokens(ctx, req) // n.TotalTokens, with the client's defaults applied
```

### Retries

429, 408 and 5xx replies and network errors are retried with exponential backoff and jitter:
4 attempts by default, waiting from 1 s up to 30 s. When Google sends a `RetryInfo` delay the client
waits at least that long, and gives up if it is longer than `MaxDelay`. Change it with
`WithRetry(gogemini.RetryPolicy{...})`; `RetryPolicy{MaxAttempts: 1}` turns retries off.

### Errors

- From `New`: `ErrMissingAPIKey` (no key from `WithAPIKey` or `GEMINI_API_KEY`), `ErrInvalidBaseURL`,
  `ErrInvalidTimeout`, `ErrEmptyModel`, `ErrInvalidRetryPolicy`.
- `ErrEmptyRequest` for an empty prompt, a `nil` request or no contents: nothing is sent.
- `ErrRedirectOtherHost` when the server redirects to another host: the redirect is not followed.
- `*gogemini.APIError` for any non-2xx reply (after retries), with the HTTP code, Google's status
  (e.g. `RESOURCE_EXHAUSTED`), the message, the `Reason` from `ErrorInfo` (e.g. `API_KEY_INVALID`)
  and the `RetryDelay` Google asked for; `Retryable()` tells transient errors apart:

```go
var apiErr *gogemini.APIError
if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
	// quota exceeded: back off
}
```

## Authentication

Create a key in [Google AI Studio](https://aistudio.google.com/apikey) and put it in
`GEMINI_API_KEY`. The client sends it in the `x-goog-api-key` header, never in the URL, so it does
not end up in access logs or error messages. It only goes over HTTPS, and never to another host:
`net/http` would copy the header to any redirect target, so redirects to a different host are
refused. With `WithHTTPClient`, a client without `CheckRedirect` gets the same rule; if you set your
own `CheckRedirect`, keeping the key on the right host is up to it.

Never commit a key: `.env*` files are git-ignored. The Bard-era approach (copying the
`__Secure-1PSID` browser cookie) is not supported: a session cookie grants access to the whole
Google account, and the web endpoint is undocumented and not meant for automated use.

## Try it

Copy your key from Google AI Studio, then write it to a git-ignored `.env` straight from the
clipboard, so it never appears on screen or in shell history:

```bash
printf 'GEMINI_API_KEY=%s\n' "$(pbpaste | tr -d '[:space:]')" > .env && chmod 600 .env
```

Check the format before loading it (`1` means fine). A file without the `GEMINI_API_KEY=` prefix
would make the shell print the key in an error:

```bash
grep -c '^GEMINI_API_KEY=.' .env
```

```bash
set -a && source ./.env && set +a && go run ./examples/generate "Explain goroutines in one sentence"
```

Flags: `-model`, `-timeout`, `-stream` (print the answer as it arrives), `-system` (system
instruction), `-image file` (send an image with the prompt) and `-count` (only count tokens). The model and token usage are printed on stderr.

## Upgrading from v0.4.0

- The exported `DefaultTimeout` constant is gone, and the default is now 5 minutes per attempt.
- `WithTimeout` now limits one attempt: the request and the whole reply for `Generate`, and only the
  wait for the reply to start for a stream, which is then bounded by your `ctx` alone. Before, a
  60-second `http.Client.Timeout` could cut a long stream. It also applies with `WithHTTPClient` now.
- An attempt that runs out of time is retried; your own `ctx` deadline is not.

## Upgrading from v0.3.0

Nothing to change in your code: v0.4.0 only adds. One behaviour is new: 408, 429 and 5xx replies and
network errors are now retried (4 attempts, 1 s up to 30 s), so a call can take longer before it
returns an error. For the old single attempt, pass `WithRetry(gogemini.RetryPolicy{MaxAttempts: 1})`.

## Upgrading from v0.2.0

The exported `DefaultModel` constant is gone: read the model with `client.Model()`, or pin one with
`WithModel`. `New` now also fails for an invalid base URL, a timeout `<= 0` or an empty model, and
redirects to another host are refused with `ErrRedirectOtherHost`.

## Upgrading from v0.1.0

v0.2.0 is a breaking change. The Bard-era API is gone: `gogemini.NewGoGemini`, `IGoGemini`,
`GetAnswer` (a stub that never sent a request), and the `configuration`, `constants` and `env`
packages. `IS_DEBUG`, `_BARD_API_KEY` and `APP_ENV` are no longer read. Replace them with
`gogemini.New(...)` and `Client.GenerateContent`, and set `GEMINI_API_KEY`. See
[CHANGELOG.md](CHANGELOG.md).

## Development

```bash
gofmt -l .
go vet ./...
go test -race ./...
golangci-lint run ./...   # v2.14.0, config in .golangci.yml
```

The tests use `httptest` and never call Google; no environment variables are needed.

Milestones and their issues on GitHub are generated from [docs/milestone.md](docs/milestone.md) by the
*Milestone sync* workflow: edit the file, not the issues. Preview locally with
`python3 .github/scripts/milestone_sync.py --dry-run`.

## Stability

The API follows [Semantic Versioning](https://semver.org/) and is stable since v1.0.0: 1.x releases
only add to it, and CI rejects incompatible changes to the exported API (`gorelease`). The
default model follows Google's recommendation and may change in a minor release: pin one with
`WithModel` if your output must not change.

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities privately, as described in
[SECURITY.md](SECURITY.md).

## License
MIT. See [LICENSE](LICENSE). Not affiliated with Google.
