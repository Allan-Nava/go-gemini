# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/) (pre-1.0: minor versions may break the API).

## [Unreleased]

## [v1.0.0] — 2026-09-29

The first stable release: the exported API is frozen, and from here CI rejects incompatible changes
(`gorelease` against the latest tag). Compared with v0.4.0 it fixes long streams being cut and makes
the timeout per attempt. See `docs/api-review-v1.0.0-2026-09-29.md` for what was reviewed.

### Fixed
- **Long streams are no longer cut.** The default client set `http.Client.Timeout` (60 s), which also
  covers reading the body, so a stream longer than that stopped halfway. Each attempt is now bounded
  through its context instead: for `Generate`, the request and the whole reply; for a stream, only the
  wait for the reply to start.

### Changed
- **Breaking:** the exported `DefaultTimeout` constant is removed; the default is now 5 minutes per
  attempt. Constants whose value may change are not exported (see `docs/api-review-v1.0.0-2026-09-29.md`).
- `WithTimeout` also applies with `WithHTTPClient`, and an attempt that runs out of time is retried.
  The caller's own `ctx` deadline is never retried.
- `examples/generate`: `-timeout` defaults to 5 minutes.

## [v0.4.0] — 2026-09-29

Streaming, multi-turn chat, generation parameters and retries: the SDK experience before v1.0.0.
Everything is additive; the one change in behaviour is that transient failures are now retried.

### Added
- Streaming: `Client.GenerateContentStream` and `Client.GenerateStream` return an
  `iter.Seq2[*Response, error]` over `:streamGenerateContent?alt=sse`. Breaking out of the loop closes
  the connection; failures before the first chunk are retried, an error event ends the stream.
- Chat: `Client.NewChat(history...)`, `Chat.Send`, `Chat.History`. A turn is kept only when the model
  answers; safe for concurrent use, turns are sent one at a time.
- Generation parameters: `GenerationConfig` (`Temperature`, `TopP`, `TopK`, `MaxOutputTokens`,
  `CandidateCount`, `StopSequences`, `ResponseMIMEType`, `Seed`) and `SystemInstruction` on
  `GenerateContentRequest`; `WithGenerationConfig` and `WithSystemInstruction` set client defaults;
  `Ptr` for optional values.
- Retries: `WithRetry(RetryPolicy)`, `DefaultRetryPolicy`, `ErrInvalidRetryPolicy`. 408, 429, 5xx and
  network errors are retried with exponential backoff and jitter, honouring Google's `RetryInfo`.
- `APIError.Reason` (from `ErrorInfo`), `APIError.RetryDelay` (from `RetryInfo`) and `APIError.Retryable()`.
- `UsageMetadata.CachedContentTokenCount` and `ThoughtsTokenCount`.
- `examples/generate`: `-stream` and `-system` flags.
- CI: `golangci-lint` v2.14.0 (config in `.golangci.yml`).

### Changed
- **Behaviour:** transient failures are now retried by default (4 attempts, 1 s to 30 s). Use
  `WithRetry(gogemini.RetryPolicy{MaxAttempts: 1})` for the previous single-attempt behaviour.
- `ErrInvalidBaseURL` now wraps the parse error, so `errors.As` can reach it.

### Removed
- Dependabot configuration: Renovate alone keeps modules and actions up to date (`config:recommended`).

## [v0.3.0] — 2026-09-29

Hardening from the [v0.2.0 audit](docs/audit-v0.2.0-2026-09-29.md), and the groundwork to freeze the
API for v1.0.0.

### Security
- The API key is no longer sent to another host on a redirect. `net/http` copies custom headers,
  `x-goog-api-key` included, to any redirect target; redirects to a different host or scheme are now
  refused with `ErrRedirectOtherHost`. A client passed with `WithHTTPClient` and no `CheckRedirect`
  gets the same rule, on a copy.
- `WithBaseURL` only accepts HTTPS (plain HTTP for loopback hosts only), so the key is never sent in clear.

### Added
- `ErrInvalidBaseURL`, `ErrInvalidTimeout`, `ErrEmptyModel` from `New`; `ErrEmptyRequest` for an empty
  prompt, a `nil` request or no contents, returned without sending anything.
- `User-Agent: go-gemini/<version>` on every request.
- Runnable examples for `Client.Generate`, `APIError` and `WithHTTPClient`.
- `CONTRIBUTING.md` and `SECURITY.md` (private vulnerability reporting).
- *Release* workflow: pushing a `vX.Y.Z` tag checks the changelog section and the SDK version, runs
  the tests and creates the GitHub release from the changelog.

### Changed
- `WithTimeout` with a duration `<= 0` now makes `New` fail, instead of silently removing the timeout.
- `APIError.Message` is truncated to about 1 KiB; successful replies are read up to 32 MiB and drained,
  so connections are reused.
- CI runs `staticcheck`; `staticcheck` and `govulncheck` versions are pinned. CI fails if coverage of
  `gogemini` drops below 90%, and runs `gorelease` against the latest tag (blocking from v1.0.0).

### Removed
- **Breaking:** the exported `DefaultModel` constant. The default model follows Google's recommendation
  and has to change over time, and changing an exported constant is an incompatible API change. Read
  the model with `Client.Model()`, or pin one with `WithModel`.
- `Dockerfile` (it could not build a library) and `.vscode/`. The Bard-era backlog moved to `docs/archive/`.

## [v0.2.0] — 2026-09-29

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

[Unreleased]: https://github.com/Allan-Nava/go-gemini/compare/v1.0.0...main
[v1.0.0]: https://github.com/Allan-Nava/go-gemini/compare/v0.4.0...v1.0.0
[v0.4.0]: https://github.com/Allan-Nava/go-gemini/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/Allan-Nava/go-gemini/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/Allan-Nava/go-gemini/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/Allan-Nava/go-gemini/releases/tag/v0.1.0
