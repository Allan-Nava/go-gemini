# Controllo della documentazione — 2026-09-30

Verifica della documentazione su `main` dopo il commit di rilascio della v1.2.0 (`f87a13f`, prima del tag).
Non una rilettura: ogni punto è stato controllato con uno strumento.

```
 README.md ─┐                      ┌─► 11 snippet Go ──► compilati + go vet (modulo con replace locale)
 docs/*.html ├─► estrazione ────────┼─► 3 blocchi del sito ──► idem
 *.md       ─┘                      ├─► link relativi ──► il file esiste?
                                    ├─► 23 link esterni ──► HTTP 200?
 gogemini (go doc) ─► 74 simboli ───┼─► commento (staticcheck ST1000/1020/1021/1022)
                                    └─► citati nel README / nel sito?
```

## Risultati

| Controllo | Prima | Dopo |
|---|---|---|
| Snippet Go del README (11, di cui 1 programma completo) | ✅ compilano | ✅ |
| Blocchi di codice del sito (3) | ✅ compilano | ✅ |
| Link relativi | ❌ 2 rotti in `docs/archive/backlog-bard.md` (spostato senza aggiornarli) | ✅ nessuno |
| Link esterni (23) | ✅ 200, tranne 3 attesi¹ | ✅ |
| Godoc dei 74 simboli esportati | ✅ tutti commentati | ✅ |
| Simboli citati nel README | ❌ 14 tipi mai nominati (`Part`, `Blob`, `Tool`, `FunctionCall`, `UsageMetadata`, …) | ✅ tabella "API at a glance" con tutti |
| Tabella API del sito | ⚠️ descritta come "per una singola chiamata"; mancavano tool calling, tipi di base, costanti | ✅ completata, rimanda al README per l'elenco intero |
| Layout della tabella API | ❌ prima colonna `nowrap`: con i nomi lunghi le descrizioni uscivano a destra | ✅ va a capo negli spazi; nessuno scroll orizzontale a 375 px |
| Sezione Status del sito | ❌ ferma alla v0.2.0 ("A real call", card sul client Bard rimosso) | ✅ riscritta su cosa fa l'SDK oggi |
| Sezione Roadmap del sito | ⚠️ "What comes next" con solo versioni già uscite | ✅ "Release history", voce di menu "Releases" |
| README: link "latest audit" | ⚠️ puntava all'audit della v0.2.0 | ✅ revisione dell'API della v1.0.0 + Go reference |
| AGENTS.md | ⚠️ nessuna regola sul congelamento dell'API | ✅ aggiunta, con le rotture sottili |
| CONTRIBUTING.md | ⚠️ non citava le rotture sottili né il controllo locale | ✅ costanti, struct confrontabili, `gorelease` prima del push |
| CLAUDE.md | ⚠️ citava un solo esempio e due flag | ✅ entrambi gli esempi, tutti i flag, obbligo di aggiornare la tabella del README |

¹ `fonts.googleapis.com` e `fonts.gstatic.com` sono domini di `preconnect`, non pagine; la release `v1.2.0`
non esiste finché il tag non viene pubblicato.

## Cercati e non trovati

Nei documenti attuali (esclusi audit, archivio e sezioni storiche del changelog) nessun riferimento
superato a: `DefaultModel`, `DefaultTimeout` (salvo "Upgrading from v0.4.0"), timeout di 60 s, API del
client Bard (salvo "Upgrading from v0.1.0"), Dependabot, Jekyll, resty, "pre-release",
`responseSchema`/`responseJsonSchema`, `PromptFeedback.SafetyRatings`.

## Come ripetere il controllo

Gli snippet si compilano estraendo i blocchi ` ```go ` del README e i `<pre><code>` del sito (tolto l'HTML
della colorazione), ciascun frammento in una funzione che riceve `ctx`, `client` e `req`, in un modulo
con `replace github.com/Allan-Nava/go-gemini => <checkout>`. I link relativi si risolvono rispetto alla
cartella del file. Per i simboli: `go doc -short ./gogemini` più costanti e metodi da `go doc -all`.
