# AGENTS.md — go-gemini

Regole operative per gli agenti AI (Copilot, Claude, altri) che lavorano su `go-gemini`, un SDK Go
per l'**API ufficiale Gemini**, senza dipendenze esterne. Le regole complete, le trappole note e lo
stato del progetto sono in [`CLAUDE.md`](CLAUDE.md): questo file ne è il riassunto e non deve
contraddirlo.

## Regole di lavoro (SEMPRE)

- **Mai `git push`, tag o release**: li fa l'utente. L'agente può preparare i comandi pronti da lanciare.
- **Commit solo se richiesto**, mai `Co-Authored-By` né footer di attribuzione.
- **Nessun segreto**: mai chiavi API, cookie o token in codice, test, esempi, log o issue. Esempi con
  placeholder (`YOUR_API_KEY`); la chiave reale sta in `GEMINI_API_KEY` o in un `.env` (ignorato da git).
- **Verifiche prima di dire "fatto"**: `gofmt -l .`, `go vet ./...`, `go test -race ./...`,
  `go mod tidy -diff`, `staticcheck`, `govulncheck`, `golangci-lint`.
- **Documentare**: ogni modifica visibile agli utenti aggiorna `README.md`, `docs/index.html` e
  `CHANGELOG.md` (*Unreleased*) nello stesso commit.
- **Pianificare in `docs/milestone.md`**: è la fonte di verità delle milestone e delle issue GitHub
  (workflow *Milestone sync*). Non modificare quelle issue su GitHub.
- **Cancellare file solo su richiesta esplicita** dell'utente.

## Pattern operativi

- Codice di libreria: restituire `error` (sentinel esportati), mai `panic`, `log.Fatal` o log su stdout.
- Test senza rete: `httptest` + `WithBaseURL`; nessun test chiama Google.
- Niente dipendenze nuove senza una ragione scritta: il modulo è solo standard library.
- Non reintrodurre lo scraping del web client di Gemini/Bard (cookie, `SNlM0e`).

## Puntatori

- [`CLAUDE.md`](CLAUDE.md) — regole complete e trappole.
- [`README.md`](README.md) — uso e autenticazione per l'utente finale.
- [`docs/milestone.md`](docs/milestone.md) · [`CHANGELOG.md`](CHANGELOG.md) · audit in `docs/audit-*.md`.
