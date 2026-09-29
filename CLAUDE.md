# CLAUDE.md — go-gemini

SDK Go (`github.com/Allan-Nava/go-gemini`, package `gogemini`) nato come client non ufficiale di Google Bard (cookie `__Secure-1PSID` + token `SNlM0e`). **Stato: scheletro, non funzionante** — `GetAnswer()` ritorna `nil`, nessun test. Bard è diventato Gemini (02/2024), ma l'endpoint web `StreamGenerate` **risponde ancora, anche senza cookie** (verifica audit §0). Sito: GitHub Pages `https://allan-nava.github.io/go-gemini/` (HTML statico in `docs/`, deploy via `.github/workflows/jekyll-gh-pages.yml`). Stato dettagliato: `docs/audit-2026-09-29.md`.

## Regole di lavoro (SEMPRE)

- **MAI `git push`** — lo fa sempre l'utente. MAI `Co-Authored-By` né footer di attribuzione in commit/PR. Commit solo se richiesto.
- **Repo PUBBLICO**: prima di ogni commit `git grep` + `git log -p` per segreti e nomi interni. ⛔ Mai cookie/token Google (`__Secure-1PSID`, `SNlM0e`, API key) in codice, test, esempi, `env/.env.*`, log di debug (`IS_DEBUG=true` fa loggare a resty **gli header**, cookie inclusi → mai incollare quell'output). Esempi solo con placeholder (`YOUR_API_KEY`).
- **Verifica prima di dire "fatto"**: `gofmt -l .` (vuoto), `go vet ./...`, `go test ./...` **senza `APP_ENV`** (oggi fallisce: vedi trappole), `go mod tidy -diff` se tocchi le dipendenze, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` se aggiorni moduli.
- **Documentare SEMPRE**: modifica all'API pubblica o all'autenticazione → `README.md` + `docs/index.html` (quickstart e tabella API) nello stesso commit. Todo → `docs/backlog.md` (non TODO sparsi nel codice). Audit/analisi → `docs/<tema>-<YYYY-MM-DD>.md` con schema ASCII.
- **Allineare tutto**: nome del progetto (go-gemini, non go-bard), variabili d'ambiente, versione Go minima: `go.mod`, matrice CI, README, sito e CLAUDE.md devono dire la stessa cosa.
- **Decisione aperta, non anticiparla**: la direzione (API ufficiale Gemini con API key vs scraping del web client) è dell'utente — vedi audit §1. Non implementare lo scraping del cookie senza conferma esplicita.

## Pattern per modifiche all'SDK

1. **Baseline**: build/vet/test prima di toccare nulla, per distinguere i fallimenti nuovi da quelli esistenti.
2. **Errori, non panic**: il codice di libreria ritorna `error` (sentinel esportati, `errors.Is`); niente `panic`, `log.Fatal`, `log.Println` nei package importabili.
3. **Test senza rete**: `httptest.Server` + `WithBaseURL`; nessun test deve chiamare Google. `Example…` in `_test.go` per ogni metodo pubblico (finisce su pkg.go.dev).
4. **Chiusura**: README + sito + backlog aggiornati, `gofmt`/`vet`/`test` verdi, comandi di verifica nel messaggio all'utente.

## Trappole note / regole tecniche

- **Milestone GitHub = `docs/milestone.md`**: il workflow `milestone-sync.yml` (script `.github/scripts/milestone_sync.py`, solo stdlib) crea/aggiorna milestone e issue con label `milestone-sync` a ogni push su `main`; sulle PR fa solo `--check`. ⛔ Non modificare titolo/corpo di quelle issue su GitHub: il sync li sovrascrive. Per chiudere un item: `[x]` nel file (chiuderla solo su GitHub lascia un warning, il sync non la riapre). Ogni item vuole `<!-- id:… -->` univoco e **stabile** (rinominarlo = issue nuova + la vecchia chiusa *not planned*). Prima di applicare a mano: `python3 .github/scripts/milestone_sync.py --dry-run`. ⚠️ Mai filtrare le issue per label nello script: quell'elenco è in ritardo di qualche secondo sulle scritture e un secondo giro ravvicinato ha creato duplicati (#46, #47 il 2026-09-29).

- **Prove dal vivo contro Google**: solo su richiesta esplicita, una richiesta alla volta, mai in CI né nei test del repo. La risposta di `StreamGenerate` contiene la **località stimata dall'IP**: non salvarla in file tracciati, log o issue. Senza sessione la pagina **non** contiene `SNlM0e`.

- **`go test ./...` in locale muore** con `Error loading .env file`: `test/a_main_test.go` imposta `APP_ENV=test` e `env.Load()` cerca `../env/.env.test`, che non esiste. In CI passa solo perché il workflow setta `APP_ENV=runner`. Non "risolvere" creando `.env.test` con valori veri.
- `configuration.GetConfiguration()` fa **`panic`** se `env.Parse` fallisce; la variabile è `_BARD_API_KEY` (underscore iniziale) ma il campo contiene un cookie, non una API key.
- Il client resty imposta a mano l'header `Host` e un `Content-Type` form-urlencoded **globale**, mentre `restyPost` manda un body JSON: incoerente con l'endpoint `StreamGenerate` (form `f.req`/`at`). Nessun timeout configurato → una richiesta può restare appesa.
- **Floor Go 1.26** (`go 1.26.0`): lo impone `golang.org/x/net` v0.59 (via resty v2.17). Matrice CI = floor + `stable`; se alzi il floor aggiorna `go.mod`, entrambi i workflow, `Dockerfile` e `docs/index.html` (Quickstart).
- **GitHub Pages**: build type *workflow*, pubblica `./docs` così com'è (`.nojekyll`, nessun Jekyll). I `.md` in `docs/` si leggono su GitHub, non come pagine del sito: linkarli con URL `github.com/.../blob/main/docs/...`.
- `Dockerfile` copia `/app/main` ma il repo è una libreria senza `main` → la build Docker fallisce. Non usarlo come riferimento.
- Aggiornamenti dipendenze: attivi **sia** Dependabot **sia** Renovate (doppie PR); Dependabot punta anche a `/tests`, che non esiste.

## Puntatori

- Audit e priorità: `docs/audit-2026-09-29.md` · Backlog: `docs/backlog.md` · Milestone: `docs/milestone.md` (fonte di verità delle milestone/issue GitHub, vedi trappole)
- Sito: `docs/index.html` (+ `docs/404.html`), stesso stile di `Allan-Nava/MistServer-go-sdk` (token colore su `:root`, dark/light, niente dipendenze JS).
- Regole per altri agenti: `AGENTS.md` (tenerlo coerente con questo file).
- API ufficiale Gemini: https://ai.google.dev/api — SDK Go ufficiale `google.golang.org/genai`.
