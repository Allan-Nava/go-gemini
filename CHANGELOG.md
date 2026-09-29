# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/) (pre-1.0: minor versions may break the API).

## [Unreleased] — v0.2.0

First working client, on the official Gemini API.

### Added
- `gogemini.New(opts...)` with `WithAPIKey`, `WithModel`, `WithTimeout`, `WithHTTPClient` and
  `WithBaseURL`; the key falls back to `GEMINI_API_KEY`. Default model `gemini-3.8-flash`, 60 s timeout.
- `Client.GenerateContent(ctx, prompt)` and `Client.Generate(ctx, req)` on
  `POST /v1beta/models/{model}:generateContent`, with the key in the `x-goog-api-key` header.
- `Response.Text()`, `PromptFeedback`, `UsageMetadata`, `ModelVersion`.
- Errors: `ErrMissingAPIKey`, and `*APIError` (HTTP code, Google status, message) for non-2xx replies.
- `examples/generate`, a command that sends one prompt.
- Offline tests (`httptest`) and runnable examples for pkg.go.dev.

### Removed (breaking)
- The Bard web client: `NewGoGemini`, `IGoGemini`, `GetAnswer` (a stub that never sent a request),
  and the `configuration`, `constants` and `env` packages.
- The `IS_DEBUG`, `_BARD_API_KEY` and `APP_ENV` environment variables.
- All third-party dependencies (`resty`, `caarlos0/env`, `godotenv`): the module is standard library only.

### Changed
- Minimum Go version 1.26. CI tests on 1.26 and stable, with `go vet`, `-race`, `go mod tidy -diff`
  and `govulncheck`.
- Documentation site rebuilt as a static GitHub Pages site; milestones and their issues are
  generated from `docs/milestone.md`.

## [v0.1.0] — 2023-08-16

- Skeleton of a Google Bard client (session cookie and `SNlM0e` token). Never sent a request.

[Unreleased]: https://github.com/Allan-Nava/go-gemini/compare/v0.1.0...main
[v0.1.0]: https://github.com/Allan-Nava/go-gemini/releases/tag/v0.1.0
