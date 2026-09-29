# Milestone

Roadmap di `go-gemini`, dalla più vicina. Contesto e priorità: [`audit-2026-09-29.md`](audit-2026-09-29.md);
attività di dettaglio: [`backlog.md`](backlog.md).

> **Questo file è la fonte di verità** per le [milestone](https://github.com/Allan-Nava/go-gemini/milestones) e le
> issue con label `milestone-sync`: il workflow *Milestone sync* le riallinea a ogni push su `main`.
> Ogni `## vX.Y.Z — Titolo` è una milestone, ogni `- [ ] <!-- id:… -->` una issue (`[x]` = chiusa;
> item tolto = chiusa come *not planned*). Gli `id` sono stabili: non rinominarli, o nasce una issue nuova.

```
  v0.1.0 (tag)          v0.1.x (fatto)            v0.2.0 (nuova)                v0.3.0            v1.0.0
  scheletro Bard ──►  audit, sito, dipendenze ──► primo client funzionante ──► esperienza SDK ──► API stabile
                      Go 1.26, 0 vuln             API ufficiale + test          streaming, chat
```

## v0.2.0 — Primo client funzionante (codice completo, in attesa del tag)

**Obiettivo:** `go get` + una API key → una risposta di Gemini in cinque righe di codice, con test che
non toccano la rete.

**Direzione:** API ufficiale Gemini (`generativelanguage.googleapis.com`, header `x-goog-api-key`), opzione
A dell'audit §1. L'endpoint web `StreamGenerate` risponde ancora (audit §0) ma resta fuori da questa
milestone: non è documentato e non è pensato per uso automatizzato.

### Scope

- [x] <!-- id:v020-options --> **Configurazione a opzioni funzionali**: `gogemini.New(opts...) (*Client, error)` con `WithAPIKey`,
      `WithModel`, `WithBaseURL`, `WithHTTPClient`, `WithTimeout`; default di timeout sensato.
- [x] <!-- id:v020-no-panic --> **Niente panic nella libreria**: via `panic` da `configuration`, `log.Fatal` da `env`, `log.Println` da
      `gogemini`; errori esportati (`ErrMissingAPIKey`, `*APIError` con status e messaggio di Google).
- [x] <!-- id:v020-generate-content --> **`GenerateContent(ctx, prompt string) (*Response, error)`** su `POST /v1beta/models/{model}:generateContent`,
      con tipi di request/response minimi (`contents`, `candidates[].content.parts[].text`) e helper `Response.Text()`.
- [x] <!-- id:v020-api-key-env --> **Chiave da ambiente**: `GEMINI_API_KEY` letta solo se il chiamante non passa `WithAPIKey`; `_BARD_API_KEY` rimossa.
- [x] <!-- id:v020-offline-tests --> **Test senza rete**: `httptest.Server` per successo, 400/403/429, body malformato, context cancellato;
      `Example` per `New` e `GenerateContent`. `go test ./...` verde **senza** `APP_ENV`.
- [x] <!-- id:v020-legacy-cleanup --> **Pulizia legacy**: rimuovere `getSnim0e`, `RequestGetAnswer`, costanti `bard.google.com`, header
      browser (`Host`, `Origin`, UA Chrome 91), package `env` e file `env/.env.*`; `.gitignore` con `.env*`.
- [x] <!-- id:v020-docs --> **Docs allineate**: README (uso, auth, variabili), `docs/index.html` (hero, Quickstart, tabella API,
      diagramma auth senza "planned"), CLAUDE.md, backlog.
- [ ] <!-- id:v020-release --> **Rilascio v0.2.0**: prova reale a mano con una chiave di test
      (`go run ./examples/generate "…"`), poi `CHANGELOG.md` da *Unreleased* a data, tag `v0.2.0` e
      release GitHub con le note del changelog. Si spunta dopo il tag: chiude la milestone.

> **Avanzamento 2026-09-29**: codice completo. Client in `gogemini/client.go` + `generate.go` (solo stdlib,
> modulo senza dipendenze), test `httptest`, package Bard rimossi, `examples/generate`, `CHANGELOG.md`.
> Resta solo `v020-release` (prova reale + tag), che è dell'utente.

### Fuori scope

Streaming, chat multi-turno, immagini/file, tool calling, Vertex AI, endpoint web anonimo.

### Criteri di chiusura

1. `gofmt -l .` vuoto, `go vet ./...`, `go test -race ./...` (senza variabili d'ambiente) e `govulncheck` verdi, in locale e in CI.
2. Una chiamata reale con una chiave di test, eseguita **a mano** dall'utente, restituisce testo.
3. Nessuna chiave, cookie o risposta reale nei file tracciati (`git grep` + `git log -p`).
4. Tag `v0.2.0` con note di rilascio che segnalano il **breaking change** (`NewGoGemini`/`GetAnswer` rimossi).

## v0.3.0 — Esperienza SDK

**Obiettivo:** coprire i casi d'uso comuni oltre la singola domanda: streaming, conversazioni, parametri, retry.

- [ ] <!-- id:v030-streaming --> **Streaming**: `streamGenerateContent` (SSE) esposto come iteratore.
- [ ] <!-- id:v030-chat --> **Chat multi-turno**: storico `contents` con ruoli `user`/`model`.
- [ ] <!-- id:v030-generation-config --> **Parametri di generazione**: `temperature`, `maxOutputTokens`, system instruction.
- [ ] <!-- id:v030-retry --> **Retry con backoff** su 429/5xx, configurabile.
- [ ] <!-- id:v030-lint-deps --> **Lint e dipendenze**: `golangci-lint` in CI; tenere una sola tra Renovate e Dependabot.

## v1.0.0 — API stabile

**Obiettivo:** una superficie pubblica su cui gli utenti possano contare senza breaking change.

- [ ] <!-- id:v100-api-freeze --> **API congelata**: superficie pubblica documentata su pkg.go.dev.
- [ ] <!-- id:v100-contributing --> **Contributi e rilasci**: `CONTRIBUTING.md`, changelog, release automatiche da tag.
- [ ] <!-- id:v100-coverage --> **Copertura di test** sulle funzioni pubbliche.

## Storico

- **v0.1.0** — scheletro del client Bard (cookie `__Secure-1PSID`, token `SNlM0e`); `GetAnswer()` mai implementato.
- **v0.1.x** (2026-09-29) — audit, `CLAUDE.md`, sito GitHub Pages statico, resty v2.17.2 / `x/net` v0.59.0,
  floor Go 1.26, CI con vet/race/govulncheck.
- Le milestone precedenti (stabilizzare l'integrazione Bard, estrarre `SNlM0e`) sono **superate** dalla
  scelta dell'API ufficiale.
