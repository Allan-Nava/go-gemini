# Milestone

Roadmap di `go-gemini`, dalla più vicina. Contesto e priorità: [`audit-2026-09-29.md`](audit-2026-09-29.md);
audit: [`audit-v0.2.0-2026-09-29.md`](audit-v0.2.0-2026-09-29.md).

> **Questo file è la fonte di verità** per le [milestone](https://github.com/Allan-Nava/go-gemini/milestones) e le
> issue con label `milestone-sync`: il workflow *Milestone sync* le riallinea a ogni push su `main`.
> Ogni `## vX.Y.Z — Titolo` è una milestone, ogni `- [ ] <!-- id:… -->` una issue (`[x]` = chiusa;
> item tolto = chiusa come *not planned*). Gli `id` sono stabili: non rinominarli, o nasce una issue nuova.

```
  v0.2.0 ─► v0.3.0 ─► v0.4.0 ─► v1.0.0 (rilasciata) ─► v1.1.0 (rilasciata) ────────► v1.2.0
  client    hardening  streaming  API congelata        immagini, sicurezza, JSON,     tool calling
  ufficiale            chat, retry                     thinking, chat stream, token
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

## v0.3.0 — Hardening sicurezza e pulizia (rilasciata, 2026-09-29)

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
- [x] <!-- id:v021-release --> **Rilascio v0.3.0**: release GitHub di `v0.2.0` recuperata (R1, fatto); prova reale con una chiave;
      tag `v0.3.0` — la release la crea il workflow `release.yml` dal changelog.

> **Prova reale 2026-09-29**, eseguita a mano dall'utente con `examples/generate`: `pong` da `gemini-3.5-flash-lite`
> (9 token) con `User-Agent` e regola sui redirect della v0.3.0. `gorelease` in CI: `DefaultModel` rimosso, suggerita v0.3.0.

## v0.4.0 — Esperienza SDK (rilasciata, 2026-09-29)

**Obiettivo:** coprire i casi d'uso comuni oltre la singola domanda: streaming, conversazioni, parametri, retry.

- [x] <!-- id:v030-streaming --> **Streaming**: `streamGenerateContent` (SSE) esposto come iteratore.
- [x] <!-- id:v030-chat --> **Chat multi-turno**: storico `contents` con ruoli `user`/`model`.
- [x] <!-- id:v030-generation-config --> **Parametri di generazione**: `temperature`, `maxOutputTokens`, system instruction.
- [x] <!-- id:v030-retry --> **Retry con backoff** su 429/5xx, configurabile; `APIError` espone i `details` di Google (`RetryInfo.retryDelay`, `ErrorInfo.reason`) — audit v0.2.0 A8.
- [x] <!-- id:v030-lint-deps --> **Lint e dipendenze**: `golangci-lint` in CI; tenere una sola tra Renovate e Dependabot.
- [x] <!-- id:v040-release --> **Rilascio v0.4.0**: prova reale a mano (`examples/generate` con e senza `-stream`), changelog
      con data, `version` = `0.4.0`, tag `v0.4.0` — la release la crea il workflow.

> **Prova reale 2026-09-29**, eseguita a mano dall'utente: `-stream` su `gemini-3.5-flash-lite` → `1`…`5` in più chunk,
> uso token dall'ultimo evento (22). Sul default `gemini-3.8-flash` di nuovo `503 UNAVAILABLE` (alta domanda): il client
> ha ritentato 4 volte con backoff e poi restituito l'ultimo `*APIError`, come previsto.

> **Avanzamento 2026-09-29**: `GenerateContentStream`/`GenerateStream` (`iter.Seq2`, SSE), `NewChat`/`Send`/`History`,
> `GenerationConfig` + `SystemInstruction` con default del client, `WithRetry`/`RetryPolicy` (default 4 tentativi,
> backoff con jitter, `RetryInfo` rispettato), `APIError.Reason`/`RetryDelay`/`Retryable()`. Tutto additivo (nessun
> cambiamento incompatibile); cambia il comportamento di default: ora i 429/5xx vengono ritentati. `golangci-lint` in
> CI, solo Renovate. 14 mutazioni del codice nuovo, tutte intercettate dai test (il mutex della chat con `-race`).

## v1.0.0 — API stabile (rilasciata, 2026-09-29)

**Obiettivo:** una superficie pubblica su cui gli utenti possano contare senza breaking change.

- [x] <!-- id:v100-api-freeze --> **API congelata**: superficie pubblica documentata su pkg.go.dev.
- [x] <!-- id:v100-contributing --> **Contributi e rilasci**: `CONTRIBUTING.md`, changelog, release automatiche da tag.
- [x] <!-- id:v100-coverage --> **Copertura di test** sulle funzioni pubbliche.
- [x] <!-- id:v100-release --> **Rilascio v1.0.0**: tag `v1.0.0` dopo la v0.4.0, quando streaming, chat e retry hanno confermato il design; da lì `gorelease` in CI blocca
      ogni cambiamento incompatibile dell'API esportata.

> **Revisione finale 2026-09-29** ([`api-review-v1.0.0-2026-09-29.md`](api-review-v1.0.0-2026-09-29.md)): trovato e corretto
> il taglio degli stream lunghi (`http.Client.Timeout` comprende la lettura del body); timeout ora per tentativo via context,
> default 5 minuti, `DefaultTimeout` non più esportata. Modello di default confermato `gemini-3.8-flash`. Resta: prova reale
> (normale e `-stream`), poi `version` = `1.0.0`, changelog con data, tag.
>
> **Prova reale 2026-09-29**, eseguita a mano dall'utente su `gemini-3.5-flash-lite`: `pong` (9 token) e uno stream di
> dieci righe (120 token) arrivato intero con il nuovo timeout per tentativo. Tag `v1.0.0`.

> **Avanzamento 2026-09-29**: preparazione completa. API rivista (21 simboli, tutti documentati, esempi per
> `New`, `GenerateContent`, `Generate`, `APIError`, `WithHTTPClient`); costanti che cambiano valore
> (`version`, `defaultModel`) non più esportate, perché `apidiff` le tratta come incompatibili; gate CI su copertura
> (≥ 90%, oggi 96,6%) e su `gorelease`; `CONTRIBUTING.md`, `SECURITY.md`, workflow `release.yml` dal changelog.

## v1.1.0 — Multimodale, sicurezza e output strutturato (rilasciata, 2026-09-30)

**Obiettivo:** coprire i casi d'uso più richiesti dopo il testo semplice (immagini e file, filtri di sicurezza, risposte JSON con schema, conteggio dei token), solo con aggiunte compatibili.

Vincolo: dalla v1.0.0 l'API è congelata. Ogni item **aggiunge** campi, tipi, metodi o opzioni; `api-compat` in CI
blocca qualsiasi cambiamento incompatibile. Nessuna dipendenza nuova.

- [x] <!-- id:v110-inline-data --> **Immagini e file inline**: `Part.InlineData` (`*Blob{MIMEType, Data []byte}`, base64 via `encoding/json`) e
      `Part.FileData` (`*FileData{MIMEType, FileURI}`); costruttori `TextPart`, `InlineDataPart`, `FileDataPart`, `UserContent`;
      `Chat.SendParts`; esempio con un'immagine. Test sul JSON inviato.
- [x] <!-- id:v110-safety --> **Impostazioni e valutazioni di sicurezza**: `GenerateContentRequest.SafetySettings` (`category`, `threshold`) e
      opzione `WithSafetySettings` come default del client; `SafetyRatings` su `Candidate`. Non su `PromptFeedback`: una slice
      la renderebbe non confrontabile, cambiamento incompatibile (trovato da `gorelease`).
- [x] <!-- id:v110-structured-output --> **Output strutturato**: `GenerationConfig.ResponseFormat` (`responseFormat.text` = `mimeType`
      `APPLICATION_JSON` + `schema` JSON Schema) e helper `JSONResponse(schema)`; esempio che decodifica la risposta in una struct.
      `responseSchema`/`responseJsonSchema` sono deprecati nell'API (riferimento del 2026-09-30): non si implementano.
- [x] <!-- id:v110-thinking --> **Ragionamento dei modelli**: `GenerationConfig.ThinkingConfig` (`IncludeThoughts`, `ThinkingBudget`,
      `ThinkingLevel` `MINIMAL`…`HIGH`), `Part.Thought` e `Part.ThoughtSignature` (la chat la rimanda nei turni successivi, altrimenti
      `MISSING_THOUGHT_SIGNATURE`); `Response.Text()` salta le parti di ragionamento (oggi non arrivano, quindi nessun cambiamento per chi non le chiede),
      `Response.Thoughts()` le restituisce.
- [x] <!-- id:v110-chat-stream --> **Chat in streaming**: `Chat.SendStream(ctx, text)` → `iter.Seq2[*Response, error]`; a fine stream la risposta
      completa entra nella storia, su errore o `break` la storia resta com'era.
- [x] <!-- id:v110-count-tokens --> **Conteggio dei token**: `Client.CountTokens(ctx, req) (*CountTokensResponse, error)` su `:countTokens`
      con `generateContentRequest` (verificato: risposta `totalTokens`, `cachedContentTokenCount`); una struct e non un `int`, per poterla
      estendere. Stesso percorso di invio, retry e timeout.
- [x] <!-- id:v110-release --> **Rilascio v1.1.0**: `gorelease` senza cambiamenti incompatibili, prova reale a mano (testo, immagine, stream
      della chat), `version` = `1.1.0`, changelog con data, tag — la release la crea il workflow.

> **Avanzamento 2026-09-30**: sei item su sette nel codice. Nomi dei campi verificati sul riferimento ufficiale scaricato
> (lì `responseSchema`/`responseJsonSchema` risultano deprecati → `responseFormat`). `gorelease` contro `v1.0.0` ha trovato due
> rotture di confrontabilità (`Client`, `PromptFeedback`), corrette prima del commit: ora solo aggiunte, suggerita v1.1.0.
> 11 mutazioni del codice nuovo, tutte intercettate. `Chat` ora rimanda le `thoughtSignature`.
>
> **Prova reale 2026-09-30**, eseguita a mano dall'utente su `gemini-3.5-flash-lite`: `CountTokens` → 8 token; un PNG di prova
> generato apposta (cerchio rosso su bianco, nessun dato dell'utente) descritto correttamente, 1111 token. Tag `v1.1.0`.

## v1.2.0 — Tool calling

**Obiettivo:** far chiamare al modello funzioni Go dichiarate dal chiamante, con il giro domanda → chiamata → risultato → risposta gestito dall'SDK.

- [ ] <!-- id:v120-tools --> **Dichiarazione degli strumenti**: `GenerateContentRequest.Tools` (`FunctionDeclarations` con `Name`, `Description`,
      schema dei parametri) e `ToolConfig` (`functionCallingConfig.mode`: `AUTO`, `ANY`, `NONE`, `VALIDATED`).
- [ ] <!-- id:v120-function-parts --> **Chiamate e risultati**: `Part.FunctionCall` (`Name`, `Args`) e `Part.FunctionResponse` (`Name`, `Response`);
      `Response.FunctionCalls()` per leggerle.
- [ ] <!-- id:v120-chat-tools --> **Giro automatico in chat**: handler registrati per nome; `Chat.Send` esegue le chiamate, rimanda i risultati e
      restituisce la risposta finale, con un limite al numero di giri e il `ctx` del chiamante passato agli handler.
- [ ] <!-- id:v120-release --> **Rilascio v1.2.0**: `gorelease` senza cambiamenti incompatibili, prova reale con una funzione di esempio, tag.

## Storico

- **v0.1.0** — scheletro del client Bard (cookie `__Secure-1PSID`, token `SNlM0e`); `GetAnswer()` mai implementato.
- **v1.1.0** (2026-09-30) — immagini e file, output JSON con schema, sicurezza, thinking, chat in streaming, `CountTokens`. Vedi `CHANGELOG.md`.
- **v1.0.0** (2026-09-29) — prima versione stabile: API congelata, timeout per tentativo (gli stream lunghi non vengono più tagliati). Vedi `CHANGELOG.md` e `api-review-v1.0.0-2026-09-29.md`.
- **v0.4.0** (2026-09-29) — streaming, chat, parametri di generazione, retry con backoff, `golangci-lint`. Vedi `CHANGELOG.md`.
- **v0.3.0** (2026-09-29) — hardening dall'audit v0.2.0 (la chiave resta sull'host dell'API), preparazione della v1.0.0. Vedi `CHANGELOG.md`.
- **v0.2.0** (2026-09-29) — client per l'API ufficiale, modulo senza dipendenze, package Bard rimossi. Vedi `CHANGELOG.md`.
- **v0.1.x** (2026-09-29) — audit, `CLAUDE.md`, sito GitHub Pages statico, resty v2.17.2 / `x/net` v0.59.0,
  floor Go 1.26, CI con vet/race/govulncheck.
- Le milestone precedenti (stabilizzare l'integrazione Bard, estrarre `SNlM0e`) sono **superate** dalla
  scelta dell'API ufficiale.
