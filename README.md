# go-gemini
[![Go build](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml)
[![Go test workflow](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Allan-Nava/go-gemini/gogemini.svg)](https://pkg.go.dev/github.com/Allan-Nava/go-gemini/gogemini)

A small Go client for the [official Gemini API](https://ai.google.dev/api). Standard library only:
the module has no dependencies. Latest release: **v0.2.0** ([changelog](CHANGELOG.md));
pre-1.0, so minor versions may still change the API.

Site: https://allan-nava.github.io/go-gemini/ · Status and priorities:
[docs/audit-2026-09-29.md](docs/audit-2026-09-29.md) · [Backlog](docs/backlog.md) ·
[Milestones](docs/milestone.md)

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
| `WithModel(name)` | `gemini-3.8-flash` | model; a `models/` prefix is accepted |
| `WithTimeout(d)` | 60 s | timeout of the default HTTP client |
| `WithHTTPClient(c)` | — | your own `*http.Client` (transport, TLS, timeouts) |
| `WithBaseURL(u)` | `https://generativelanguage.googleapis.com` | proxy or test server |

`GenerateContent(ctx, prompt)` sends one user prompt; `Generate(ctx, req)` takes a full
`GenerateContentRequest`. `Response.Text()` returns the first candidate's text, and is empty when
the prompt was blocked (`Response.PromptFeedback.BlockReason`).

### Errors

- `gogemini.ErrMissingAPIKey` from `New` when neither `WithAPIKey` nor `GEMINI_API_KEY` gives a key.
- `*gogemini.APIError` for any non-2xx reply, with the HTTP code, Google's status
  (e.g. `RESOURCE_EXHAUSTED`) and message:

```go
var apiErr *gogemini.APIError
if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
	// quota exceeded: back off
}
```

## Authentication

Create a key in [Google AI Studio](https://aistudio.google.com/apikey) and put it in
`GEMINI_API_KEY`. The client sends it in the `x-goog-api-key` header, never in the URL, so it does
not end up in access logs or error messages.

Never commit a key: `.env*` files are git-ignored. The Bard-era approach (copying the
`__Secure-1PSID` browser cookie) is not supported: a session cookie grants access to the whole
Google account, and the web endpoint is undocumented and not meant for automated use.

## Try it

```bash
export GEMINI_API_KEY=...   # from Google AI Studio
go run ./examples/generate "Explain goroutines in one sentence"
```

`-model` and `-timeout` flags are available; token usage is printed on stderr.

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
```

The tests use `httptest` and never call Google; no environment variables are needed.

Milestones and their issues on GitHub are generated from [docs/milestone.md](docs/milestone.md) by the
*Milestone sync* workflow: edit the file, not the issues. Preview locally with
`python3 .github/scripts/milestone_sync.py --dry-run`.

## License
MIT. See [LICENSE](LICENSE). Not affiliated with Google.
