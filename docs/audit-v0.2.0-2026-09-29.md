# Audit go-gemini v0.2.0 — 2026-09-29

Secondo audit, sul tag `v0.2.0` (`f2957e1`), dopo la riscrittura sull'API ufficiale Gemini.
Il primo audit (stato pre-rilascio, client Bard) è [`audit-2026-09-29.md`](audit-2026-09-29.md).
Le correzioni sono pianificate nella milestone **v0.2.1** di [`milestone.md`](milestone.md).

## Sintesi

| Area | Stato | Nota |
|---|---|---|
| Build, `go vet`, `gofmt`, `staticcheck` | ✅ | nessuna segnalazione |
| Test (`-race`) | ✅ | offline, funzioni pubbliche quasi tutte al 100% |
| Dipendenze | ✅ | zero (solo stdlib), `govulncheck` 0 vulnerabilità |
| Chiamata reale | ✅ | `pong` da `gemini-3.5-flash-lite`; `503` temporaneo sul default |
| Sicurezza della chiave | ❌ | **inoltrata a un altro host sui redirect** (A1) |
| Validazione opzioni/input | ⚠️ | timeout 0 = nessun timeout, modello vuoto e richiesta `nil` arrivano all'API |
| Rilascio | ⚠️ | tag e proxy Go ok; **release GitHub non creata**; pkg.go.dev non ancora indicizzato |
| Igiene repo | ⚠️ | `Dockerfile` rotto, `.vscode/`, backlog obsoleto, doppio bot dipendenze |

## Flusso di una chiamata e punti deboli

```
 New(opts...) ─► opzioni in ordine ─► key: WithAPIKey | $GEMINI_API_KEY ─► http.Client{Timeout}
     │              A2: base URL non validato (http:// in chiaro)          A3: Timeout 0 = infinito
     │              A4: model "" accettato
     ▼
 GenerateContent(ctx, prompt) ─► Generate(ctx, req)       A4: req nil / Contents vuoti inviati
     │
     ▼
 POST {base}/v1beta/models/{model}:generateContent
     header x-goog-api-key ───────────────┐
     │                                     │  A1: 3xx verso altro host →
     │                                     └─ net/http ricopia gli header custom → chiave all'altro host
     ▼
 2xx ─► json.Decode (body non drenato, nessun limite)        A6
 non-2xx ─► *APIError{StatusCode, Status, Message}           A5: Message fino a 1 MiB
                                                              A8: nessun dettaglio (RetryInfo) per il retry
```

## Findings — codice

### A1. [ALTO] La chiave API viene inoltrata sui redirect verso un altro host — confermato

`net/http`, su un redirect verso un host diverso, toglie solo `Authorization`, `Www-Authenticate`,
`Cookie`, `Cookie2`, `Proxy-Authorization` e `Proxy-Authenticate` (`net/http/client.go`, Go 1.27.1). Gli header custom vengono ricopiati, **compreso `x-goog-api-key`**.

Riproduzione (test temporaneo, non committato): un server risponde `307` verso un secondo server
`httptest`; il secondo riceve `x-goog-api-key: secret-key` e la chiamata termina **senza errore**.
Chi controlla la base URL (proxy compromesso, `WithBaseURL` sbagliato) o riesce a far rispondere un
redirect ottiene la chiave.

Fix: il client di default rifiuta i redirect verso un host diverso (l'API Gemini non ne usa);
per un `*http.Client` passato con `WithHTTPClient` senza `CheckRedirect` si usa una copia con la
stessa regola. Test di regressione con due server `httptest`.

### A2. [MEDIO] `WithBaseURL` accetta qualsiasi URL, anche `http://`

Con `http://` la chiave viaggia in chiaro; con un URL malformato l'errore arriva solo alla prima
chiamata. Fix: `New` valida l'URL e rifiuta `http://` tranne che per host di loopback
(`127.0.0.1`, `::1`, `localhost`), che è il caso dei test.

### A3. [MEDIO] `WithTimeout(0)` (o negativo) disattiva il timeout

`http.Client{Timeout: 0}` significa "nessun timeout": una richiesta può restare appesa per sempre
senza che il chiamante lo sappia. Fix: `New` rifiuta durate `<= 0` con un errore.

### A4. [BASSO] Input non validati prima di chiamare la rete

- `WithModel("")` → richiesta a `/v1beta/models/:generateContent` → 404 poco chiaro.
- `Generate(ctx, nil)` → body `null` → `400 INVALID_ARGUMENT` da Google (verificato).
- `Contents` vuoto o prompt vuoto → 400 da Google.

Fix: errori locali (`ErrEmptyModel`, `ErrEmptyRequest`) senza consumare una richiesta né quota.

### A5. [BASSO] `APIError.Message` può contenere fino a 1 MiB

Se un proxy risponde con una pagina HTML, l'intero corpo finisce nel messaggio d'errore e nei log.
Fix: troncare il messaggio (≈1 KiB, con `…`).

### A6. [BASSO] Corpo della risposta 2xx non drenato né limitato

`json.Decoder` si ferma alla fine del valore JSON: il resto del corpo non viene letto, quindi la
connessione può non essere riusata. Nessun limite di dimensione. Fix: `io.LimitReader` generoso e
drain del resto prima di `Close`.

### A7. [BASSO] Nessun `User-Agent` dell'SDK

Le richieste escono come `Go-http-client/1.1`: impossibile distinguerle lato quota o supporto.
Fix: `User-Agent: go-gemini/<versione>`.

### A8. [INFO → v0.3.0] `APIError` non espone i dettagli di Google

Le risposte `429`/`503` contengono `details` (`RetryInfo.retryDelay`, `ErrorInfo.reason`) che servono al
retry con backoff. Rientra in `v030-retry`.

## Findings — repository e processo

| ID | Gravità | Finding | Azione |
|---|---|---|---|
| R1 | MEDIO | Release GitHub `v0.2.0` non creata (solo tag) | `gh release create` con le note del changelog |
| R2 | MEDIO | Durante la prova reale una chiave è comparsa in chiaro nell'output (file `.env` senza prefisso passato a `source`) | revocare la chiave; README: flusso `.env` sicuro con controllo del formato |
| R3 | BASSO | `Dockerfile` rotto (copia `/app/main`, che non esiste in una libreria) | rimuovere |
| R4 | BASSO | `.vscode/launch.json` tracciato | rimuovere, ignorare `.vscode/` |
| R5 | BASSO | `govulncheck@latest` non fissato in CI | fissare la versione (Renovate la aggiorna) |
| R6 | BASSO | `docs/backlog.md` obsoleto (item Bard), duplicato di `milestone.md` | archiviare nello storico, lasciare `milestone.md` unica fonte |
| R7 | BASSO | `AGENTS.md` non allineato a `CLAUDE.md` (API ufficiale, changelog, milestone sync) | riallineare |
| R8 | BASSO | Workflow Pages si chiama ancora `jekyll-gh-pages.yml` | rinominare in `pages.yml` |
| R9 | BASSO | Dependabot e Renovate attivi insieme; Dependabot su `/tests` inesistente | già in `v030-lint-deps` |
| R10 | INFO | pkg.go.dev non ha ancora la pagina di `v0.2.0` (il proxy sì) | si indicizza da solo; eventualmente "Request" dalla pagina |

## Cosa va bene

- Modulo senza dipendenze, `go.sum` vuoto: superficie di attacco da supply chain nulla lato runtime.
- Chiave in header, mai in URL; un test verifica che non compaia nei messaggi d'errore.
- `Client` immutabile dopo `New`: sicuro in concorrenza.
- `context.Context` rispettato (test con deadline), errori tipizzati con `errors.Is`/`errors.As`.
- CI: vet, `-race`, `go mod tidy -diff`, `govulncheck`; milestone e issue generate dal file.

## Comandi usati

```bash
gofmt -l . && go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test -race -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test -run TestProbeRedirectLeak ./gogemini/   # test temporaneo per A1, poi rimosso
curl -s https://proxy.golang.org/github.com/!allan-!nava/go-gemini/@v/list
```
