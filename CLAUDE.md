# CLAUDE.md — go-gemini

SDK Go (`github.com/Allan-Nava/go-gemini`, package `gogemini`) per l'**API ufficiale Gemini**: `gogemini.New(opts...)` → `Client.GenerateContent`, chiave nell'header `x-goog-api-key`. **Modulo senza dipendenze** (solo stdlib, `go.sum` vuoto). **Stato: v0.2.0 completa nel codice, non ancora taggata**; il vecchio client Bard (cookie `__Secure-1PSID` + token `SNlM0e`) è stato rimosso — breaking change, vedi `CHANGELOG.md`. Prova manuale: `go run ./examples/generate "…"` con `GEMINI_API_KEY`. Sito: GitHub Pages `https://allan-nava.github.io/go-gemini/` (HTML statico in `docs/`, deploy via `.github/workflows/jekyll-gh-pages.yml`). Contesto storico: `docs/audit-2026-09-29.md`.

## Regole di lavoro (SEMPRE)

- **MAI `git push`** — lo fa sempre l'utente, come tag e release. MAI `Co-Authored-By` né footer di attribuzione in commit/PR. Commit solo se richiesto.
- **Repo PUBBLICO**: prima di ogni commit `git grep` + `git log -p` per segreti e nomi interni. ⛔ Mai chiavi API o cookie Google in codice, test, esempi, file `.env*` (ignorati da git), log o issue. Esempi solo con placeholder (`YOUR_API_KEY`).
- **Verifica prima di dire "fatto"**: `gofmt -l .` (vuoto), `go vet ./...`, `go test -race ./...` (nessuna variabile d'ambiente), `go mod tidy -diff`, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
- **Documentare SEMPRE**: modifica all'API pubblica o all'autenticazione → `README.md` + `docs/index.html` (Quickstart e tabella API) + `CHANGELOG.md` (*Unreleased*, Keep a Changelog, in inglese) nello stesso commit. Lavoro pianificato → `docs/milestone.md` (non TODO sparsi nel codice). Audit/analisi → `docs/<tema>-<YYYY-MM-DD>.md` con schema ASCII.
- **Allineare tutto**: nome del progetto (go-gemini, non go-bard), variabili d'ambiente, modello di default, versione Go minima: `go.mod`, matrice CI, README, sito, CHANGELOG e CLAUDE.md devono dire la stessa cosa.
- **Direzione decisa (2026-09-29)**: API ufficiale Gemini (audit §1, opzione A). ⛔ Non reintrodurre lo scraping del web client (cookie, `SNlM0e`, `StreamGenerate`).
- ⛔ **Cancellazioni di file/package**: solo su richiesta esplicita dell'utente (il primo `git rm` dei package legacy è stato bloccato dai permessi finché l'utente non ha chiesto di chiudere la milestone). Mai aggirarle svuotando i file.

## Pattern per modifiche all'SDK

1. **Baseline**: build/vet/test prima di toccare nulla, per distinguere i fallimenti nuovi da quelli esistenti.
2. **Errori, non panic**: il codice di libreria ritorna `error` (sentinel esportati, `errors.Is`/`errors.As`); niente `panic`, `log.Fatal`, `log.Println` nei package importabili. ⛔ **Niente dipendenze nuove**: il modulo è solo stdlib; aggiungerne una va motivato e documentato.
3. **Test senza rete**: `httptest.Server` + `WithBaseURL`; nessun test deve chiamare Google. `Example…` in `_test.go` per ogni funzione pubblica (finisce su pkg.go.dev).
4. **Chiusura**: README + sito + CHANGELOG + milestone aggiornati, verifiche verdi, comandi di verifica nel messaggio all'utente.

## Trappole note / regole tecniche

- **Milestone GitHub = `docs/milestone.md`**: il workflow `milestone-sync.yml` (script `.github/scripts/milestone_sync.py`, solo stdlib) crea/aggiorna milestone e issue con label `milestone-sync` a ogni push su `main`; sulle PR fa solo `--check`. ⛔ Non modificare titolo/corpo di quelle issue su GitHub: il sync li sovrascrive. Per chiudere un item: `[x]` nel file (chiuderla solo su GitHub lascia un warning, il sync non la riapre); una milestone si chiude quando tutti i suoi item sono `[x]`. Ogni item vuole `<!-- id:… -->` univoco e **stabile** (rinominarlo = issue nuova + la vecchia chiusa *not planned*). Prima di applicare a mano: `python3 .github/scripts/milestone_sync.py --dry-run`. ⚠️ Mai filtrare le issue per label nello script: quell'elenco è in ritardo di qualche secondo sulle scritture e un secondo giro ravvicinato ha creato duplicati (#46, #47 il 2026-09-29).
- **Client** (`gogemini/client.go`, `generate.go`): opzioni applicate in ordine, `WithAPIKey` batte `GEMINI_API_KEY`; `WithTimeout` vale solo per il client HTTP di default (ignorato con `WithHTTPClient`). Errori non-2xx → `*APIError` (body letto max 1 MiB). La chiave non deve mai finire in URL o messaggi d'errore: c'è un test che lo verifica. Modello di default `gemini-3.8-flash` (consigliato dai doc Google al 2026-09-29; la serie 2.5 ha accesso limitato) — se cambia, aggiornare anche `ExampleNew`, README, sito.
- **Prove dal vivo contro Google**: solo su richiesta esplicita, una richiesta alla volta, mai in CI né nei test del repo. Con una chiave reale le lancia l'utente (`examples/generate`). Il vecchio endpoint web `StreamGenerate` risponde anche senza cookie (audit §0) e la risposta contiene la **località stimata dall'IP**: non salvarla in file tracciati, log o issue.
- ⚠️ **Script Python che riscrivono file**: mai `open(p, 'w').write(open(p).read())` — l'apertura in scrittura tronca il file prima della lettura. È successo a questo file il 2026-09-29 (commit `60f1640`, ripristinato prima del push).
- **Floor Go 1.26** (`go 1.26.0`): scelto con l'aggiornamento di `x/net` e mantenuto dopo la rimozione di tutte le dipendenze (versioni supportate: 1.26, 1.27). Matrice CI = floor + `stable`; se alzi il floor aggiorna `go.mod`, entrambi i workflow, `Dockerfile`, README e `docs/index.html` (Quickstart).
- **GitHub Pages**: build type *workflow*, pubblica `./docs` così com'è (`.nojekyll`, nessun Jekyll). I `.md` in `docs/` si leggono su GitHub, non come pagine del sito: linkarli con URL `github.com/.../blob/main/docs/...`.
- `Dockerfile` copia `/app/main` ma la libreria non ha un `main` alla radice → la build Docker fallisce. Non usarlo come riferimento (rimozione proposta nell'audit §8).
- Aggiornamenti dipendenze: attivi **sia** Dependabot **sia** Renovate (item `v030-lint-deps`); Dependabot punta anche a `/tests`, che non esiste.

## Puntatori

- Milestone: `docs/milestone.md` (fonte di verità delle milestone/issue GitHub) · Changelog: `CHANGELOG.md` · Audit: `docs/audit-2026-09-29.md` · Backlog storico: `docs/backlog.md`
- Esempio eseguibile: `examples/generate/main.go` (flag `-model`, `-timeout`).
- Sito: `docs/index.html` (+ `docs/404.html`), stesso stile di `Allan-Nava/MistServer-go-sdk` (token colore su `:root`, dark/light, niente dipendenze JS).
- Regole per altri agenti: `AGENTS.md` (tenerlo coerente con questo file).
- API ufficiale Gemini: https://ai.google.dev/api · modelli: https://ai.google.dev/gemini-api/docs/models
