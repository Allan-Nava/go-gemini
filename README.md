# go-gemini
[![Go build](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-build.yml)
[![Go test workflow](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml/badge.svg)](https://github.com/Allan-Nava/go-gemini/actions/workflows/go-test.yml)

A small Go client for Google Gemini. **Experimental**: the package builds, but it does not send
prompts yet. It started as a client for the Bard web interface; Bard has since become Gemini and
those endpoints are gone, so the request path is being rebuilt on the
[official Gemini API](https://ai.google.dev/api).

Site: https://allan-nava.github.io/go-gemini/ · Status and priorities:
[docs/audit-2026-09-29.md](docs/audit-2026-09-29.md) · [Backlog](docs/backlog.md) ·
[Milestones](docs/milestone.md)

### Usage (current build)

```go
cfg := configuration.GetConfiguration() // reads IS_DEBUG and _BARD_API_KEY
client := gogemini.NewGoGemini(cfg)

err := client.GetAnswer() // stub: returns nil, sends no request
```

### Authentication

The original design used the `__Secure-1PSID` session cookie copied from the browser. That is no
longer supported: the endpoints it relied on have been removed, and a session cookie grants access
to the whole Google account. The planned replacement is an API key from
[Google AI Studio](https://aistudio.google.com/apikey), read from `GEMINI_API_KEY`.

Never commit a key or a cookie. With `IS_DEBUG=true` resty logs request headers, so do not paste
debug output into issues.

### Environment variables

| Variable | Used by | Meaning |
|---|---|---|
| `IS_DEBUG` | `configuration` | `true` turns on resty debug output |
| `_BARD_API_KEY` | `configuration` | legacy Bard cookie value; to be replaced by `GEMINI_API_KEY` |
| `APP_ENV` | `env`, tests | `local`, `runner` or `test`: which `env/.env.*` file to load |

### Development

```bash
gofmt -l .
go vet ./...
APP_ENV=runner go test ./...
```

`go test ./...` without `APP_ENV` currently fails, because the tests default to `APP_ENV=test` and
`env/.env.test` does not exist (see the audit).

### License
MIT. See [LICENSE](LICENSE).
