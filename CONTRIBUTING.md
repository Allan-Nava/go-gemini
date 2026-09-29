# Contributing to go-gemini

Thanks for helping. This is a small SDK with a few firm rules; the checks below are the same ones
CI runs, so a pull request that passes them locally will usually pass there too.

## Ground rules

- **Standard library only.** The module has no dependencies. A new one needs a written reason in the
  pull request.
- **No secrets, ever.** No API keys, cookies or real responses in code, tests, examples, issues or
  logs. Use placeholders such as `YOUR_API_KEY`. Keep your own key in `GEMINI_API_KEY` or in a `.env`
  file, which git ignores.
- **Tests never call Google.** Use `httptest.NewServer` with `gogemini.WithBaseURL`. Every exported
  identifier gets a runnable `Example` when it makes sense; they show up on pkg.go.dev.
- **Errors, not panics.** Library code returns `error` values (exported sentinels or `*APIError`),
  never calls `panic`, `log.Fatal` or prints.
- **The API key stays on the API host.** Keep the same-host redirect rule and the HTTPS check; the
  regression tests in `gogemini/hardening_test.go` must keep passing.

## Before you open a pull request

```bash
gofmt -l .
go vet ./...
go test -race ./...
go mod tidy -diff
go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

CI also checks that coverage of the `gogemini` package stays at 90% or more, and runs `gorelease`
against the latest tag: from v1.0.0 on, an incompatible change to the exported API fails the build.

## Changelog and documentation

- Add a line under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md) for anything a user can notice.
- Update [README.md](README.md) and [docs/index.html](docs/index.html) when the public API or the
  authentication changes.

## Planning

Planned work lives in [docs/milestone.md](docs/milestone.md). The *Milestone sync* workflow turns it
into GitHub milestones and issues labelled `milestone-sync`: change the file, not those issues. Each
item needs a stable `<!-- id:… -->`; pull requests run `milestone_sync.py --check` on it.

## Releases (maintainers)

1. Move the `[Unreleased]` entries to `## [vX.Y.Z] — YYYY-MM-DD` in `CHANGELOG.md` and set `version`
   in `gogemini/client.go` to `X.Y.Z`.
2. Tag and push: `git tag -a vX.Y.Z -m "vX.Y.Z"` then `git push origin vX.Y.Z`.
3. The *Release* workflow checks the changelog section and the version, runs the tests and creates the
   GitHub release with the notes from the changelog. Check them locally with
   `python3 .github/scripts/release_notes.py vX.Y.Z --check`.

Versioning follows [Semantic Versioning](https://semver.org/). The default model may change in a
minor release; pin one with `WithModel` if your output must not change.

## Security

Please do not open a public issue for a security problem. See [SECURITY.md](SECURITY.md).
