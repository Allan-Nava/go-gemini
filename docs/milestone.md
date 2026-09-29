# Milestone

Roadmap di `go-gemini`, dalla più vicina. Contesto e priorità: [`audit-2026-09-29.md`](audit-2026-09-29.md);
audit: [`audit-v0.2.0-2026-09-29.md`](audit-v0.2.0-2026-09-29.md).

> **Questo file è la fonte di verità** per le [milestone](https://github.com/Allan-Nava/go-gemini/milestones) e le
> issue con label `milestone-sync`: il workflow *Milestone sync* le riallinea a ogni push su `main`.
> Ogni `## vX.Y.Z — Titolo` è una milestone, ogni `- [ ] <!-- id:… -->` una issue (`[x]` = chiusa;
> item tolto = chiusa come *not planned*). Gli `id` sono stabili: non rinominarli, o nasce una issue nuova.

```
  v0.1.0 (tag)       v0.2.0 (rilasciata)          v0.3.0 (da taggare)     v0.4.0            v1.0.0
  scheletro Bard ──► primo client funzionante ──► hardening + pulizia ──► esperienza SDK ──► API stabile
                     API ufficiale, 0 dipendenze   audit v0.2.0, prep 1.0   streaming, chat    API congelata
```

## v0.2.0 — Primo client funzionante (rilasciata, 2026-09-29)

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
- [x] <!-- id:v020-release --> **Rilascio v0.2.0**: prova reale a mano con una chiave di test
      (`go run ./examples/generate "…"`), poi `CHANGELOG.md` da *Unreleased* a data, tag `v0.2.0` e
      release GitHub con le note del changelog. Si spunta dopo il tag: chiude la milestone.

> **Avanzamento 2026-09-29**: codice completo. Client in `gogemini/client.go` + `generate.go` (solo stdlib,
> modulo senza dipendenze), test `httptest`, package Bard rimossi, `examples/generate`, `CHANGELOG.md`.
> Prova reale 2026-09-29, eseguita a mano dall'utente con `examples/generate`: risposta corretta da
> `gemini-3.5-flash-lite`; sul default `gemini-3.8-flash` un `503 UNAVAILABLE` temporaneo (alta domanda),
> gestito come `*APIError` — motiva `v030-retry`. Tag `v0.2.0`.

### Fuori scope

Streaming, chat multi-turno, immagini/file, tool calling, Vertex AI, endpoint web anonimo.

### Criteri di chiusura

1. `gofmt -l .` vuoto, `go vet ./...`, `go test -race ./...` (senza variabili d'ambiente) e `govulncheck` verdi, in locale e in CI.
2. Una chiamata reale con una chiave di test, eseguita **a mano** dall'utente, restituisce testo.
3. Nessuna chiave, cookie o risposta reale nei file tracciati (`git grep` + `git log -p`).
4. Tag `v0.2.0` con note di rilascio che segnalano il **breaking change** (`NewGoGemini`/`GetAnswer` rimossi).

## v0.3.0 — Hardening sicurezza e pulizia (codice completo, in attesa del tag)

**Obiettivo:** chiudere i finding dell'audit v0.2.0: la chiave non esce mai dall'host previsto, le opzioni sbagliate falliscono subito, il repository contiene solo ciò che serve.

Fonte: [`audit-v0.2.0-2026-09-29.md`](audit-v0.2.0-2026-09-29.md). Pianificata come patch v0.2.1, rilasciata come **v0.3.0** (decisione 2026-09-29): la preparazione della v1.0.0
rimuove la costante esportata `DefaultModel`, un cambiamento incompatibile (`gorelease` suggerisce v0.3.0). Gli `id` `v021-*`
restano quelli originali, perché sono stabili.

- [x] <!-- id:v021-redirect-key --> **Chiave mai inoltrata sui redirect** (A1, ALTO): il client di default rifiuta i redirect verso un altro host;
      con `WithHTTPClient` senza `CheckRedirect` si usa una copia con la stessa regola. Test di regressione con due server `httptest`.
- [x] <!-- id:v021-base-url --> **Validare `WithBaseURL`** (A2): URL valido e `https`, `http` solo per loopback (test); errore da `New`.
- [x] <!-- id:v021-timeout --> **Timeout sempre positivo** (A3): `WithTimeout(d <= 0)` fa fallire `New` invece di togliere il timeout.
- [x] <!-- id:v021-input-validation --> **Errori locali per input vuoti** (A4): `ErrEmptyModel`, `ErrEmptyRequest` per modello vuoto, richiesta `nil`,
      `Contents` o prompt vuoti — nessuna richiesta inviata.
- [x] <!-- id:v021-response-body --> **Corpo delle risposte** (A5, A6): `APIError.Message` troncato a ~1 KiB; risposta 2xx letta con limite e drenata.
- [x] <!-- id:v021-user-agent --> **`User-Agent: go-gemini/<versione>`** su ogni richiesta (A7), dalla costante non esportata `version`.
- [x] <!-- id:v021-repo-cleanup --> **Pulizia repository** (R3, R4, R6, R8): via `Dockerfile` e `.vscode/`; `backlog.md` archiviato nello storico;
      workflow Pages rinominato `pages.yml`. Le cancellazioni le approva l'utente.
- [x] <!-- id:v021-ci-pin --> **Strumenti CI fissati** (R5): versione esplicita di `govulncheck` (e di `staticcheck`, aggiunto come step).
- [x] <!-- id:v021-docs-align --> **Documentazione allineata** (R2, R7): `AGENTS.md` coerente con `CLAUDE.md`; README "Try it" con il flusso `.env`
      sicuro (`printf … "$(pbpaste)"` + controllo del formato prima di `source`).
- [ ] <!-- id:v021-release --> **Rilascio v0.3.0**: release GitHub di `v0.2.0` recuperata (R1, fatto); prova reale con una chiave;
      tag `v0.3.0` — la release la crea il workflow `release.yml` dal changelog.

## v0.4.0 — Esperienza SDK

**Obiettivo:** coprire i casi d'uso comuni oltre la singola domanda: streaming, conversazioni, parametri, retry.

- [ ] <!-- id:v030-streaming --> **Streaming**: `streamGenerateContent` (SSE) esposto come iteratore.
- [ ] <!-- id:v030-chat --> **Chat multi-turno**: storico `contents` con ruoli `user`/`model`.
- [ ] <!-- id:v030-generation-config --> **Parametri di generazione**: `temperature`, `maxOutputTokens`, system instruction.
- [ ] <!-- id:v030-retry --> **Retry con backoff** su 429/5xx, configurabile; `APIError` espone i `details` di Google (`RetryInfo.retryDelay`, `ErrorInfo.reason`) — audit v0.2.0 A8.
- [ ] <!-- id:v030-lint-deps --> **Lint e dipendenze**: `golangci-lint` in CI; tenere una sola tra Renovate e Dependabot.

## v1.0.0 — API stabile (preparazione completa, 2026-09-29)

**Obiettivo:** una superficie pubblica su cui gli utenti possano contare senza breaking change.

- [x] <!-- id:v100-api-freeze --> **API congelata**: superficie pubblica documentata su pkg.go.dev.
- [x] <!-- id:v100-contributing --> **Contributi e rilasci**: `CONTRIBUTING.md`, changelog, release automatiche da tag.
- [x] <!-- id:v100-coverage --> **Copertura di test** sulle funzioni pubbliche.
- [ ] <!-- id:v100-release --> **Rilascio v1.0.0**: tag `v1.0.0` dopo la v0.4.0, quando streaming, chat e retry hanno confermato il design; da lì `gorelease` in CI blocca
      ogni cambiamento incompatibile dell'API esportata.

> **Avanzamento 2026-09-29**: preparazione completa. API rivista (21 simboli, tutti documentati, esempi per
> `New`, `GenerateContent`, `Generate`, `APIError`, `WithHTTPClient`); costanti che cambiano valore
> (`version`, `defaultModel`) non più esportate, perché `apidiff` le tratta come incompatibili; gate CI su copertura
> (≥ 90%, oggi 96,6%) e su `gorelease`; `CONTRIBUTING.md`, `SECURITY.md`, workflow `release.yml` dal changelog.

## Storico

- **v0.1.0** — scheletro del client Bard (cookie `__Secure-1PSID`, token `SNlM0e`); `GetAnswer()` mai implementato.
- **v0.2.0** (2026-09-29) — client per l'API ufficiale, modulo senza dipendenze, package Bard rimossi. Vedi `CHANGELOG.md`.
- **v0.1.x** (2026-09-29) — audit, `CLAUDE.md`, sito GitHub Pages statico, resty v2.17.2 / `x/net` v0.59.0,
  floor Go 1.26, CI con vet/race/govulncheck.
- Le milestone precedenti (stabilizzare l'integrazione Bard, estrarre `SNlM0e`) sono **superate** dalla
  scelta dell'API ufficiale.
