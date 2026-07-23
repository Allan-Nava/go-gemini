# CLAUDE.md — go-gemini

Repo Go per l'interfaccia Google Bard; questo documento definisce le linee guida operative per lavorare con questo progetto.

## Regole di lavoro (SEMPRE)

- **MAI `git push` automatico**: il push lo esegue sempre l'utente. L'agente può suggerire i comandi ma non deve mai eseguirli.
- **MAI `Co-Authored-By` nei commit**: l'assistente mantiene le modifiche come suggerimenti, non come coautore.
- **Test e formattazione**: ogni modifica al codice deve considerare `gofmt ./...`, `go test ./...` e `go mod tidy` se si cambia `go.mod`/`go.sum`.
- **Documentare sempre**: se aggiungi o modifichi una funzionalità, aggiorna il `README.md` o aggiungi un documento in `docs/` se esiste. La documentazione deve descrivere l'uso, l'autenticazione e le eventuali variabili d'ambiente.
- **Niente segreti nel repository**: non inserire mai cookie, token o credenziali (`__Secure-1PSID`, chiavi API, ecc.) nei file, nei commenti o negli esempi.
- **Cambiamenti chiari**: ogni suggerimento deve includere quali file modificare, come verificare il risultato e quale problema risolve.

## Pattern di sviluppo

1. **Verifica locale**: prima di proporre una modifica, eseguire i comandi locali:
   - `gofmt -w .`
   - `go test ./...`
   - `go vet ./...`
2. **Aggiornare la documentazione**:
   - se si modifica l'interfaccia pubblica o l'autenticazione, aggiornare `README.md` sotto la sezione `Authentication` o creare una nuova sezione di esempi.
   - se si aggiunge una nuova funzione pubblica, documentarne l'utilizzo con un esempio concreto.
3. **Evitare rumore**:
   - non proporre modifiche non necessarie a file non correlati.
   - non aggiungere strumenti o dipendenze senza motivo valido.

## Trappole note / regole tecniche

- Il repository è un SDK Go: non usare file di configurazione esterni o tecnologie non richieste dal progetto.
- Non includere esempi di cookie o sessioni reali; scrivi esempi generici e sicuri.
- Non modificare la struttura principale del repository (`go.mod`, `go.sum`, `README.md`) senza esplicitare il motivo e l'impatto.
- Se si propone una modifica alla configurazione di autenticazione, spiegare chiaramente il flusso e le eventuali implicazioni di sicurezza.

## Puntatori

- `README.md` è il riferimento principale per l'uso e l'autenticazione.
- Se serve una guida più estesa su contributi e PR, suggerire la creazione di un `CONTRIBUTING.md`.
- Per modifiche feature/bugfix, indicare sempre i comandi di verifica e i file interessati.
