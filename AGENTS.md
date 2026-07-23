# AGENTS.md — go-gemini

Questo file definisce le regole operative per gli agenti AI (Copilot, Claude, altri tool AI) quando interagiscono con il repository `go-gemini`.

## Regole di lavoro (SEMPRE)

- **MAI `git push`**: l'agente suggerisce modifiche, ma non esegue il push. Lo user decide quando eseguire il push.
- **MAI `Co-Authored-By`** nei commit: l'agente non dovrebbe aggiungere se stesso come autore.
- **Non creare commit automatici**: fornire patch e suggerimenti strutturati, non commettere direttamente.
- **Documentare le scelte**: ogni modifica proposta deve includere il motivo, i file modificati e come testarli.
- **Nessun segreto nel codice**: non generare o inserire cookie, token, password o dati sensibili nel repository.
- **Verifiche**: indicare sempre i comandi di verifica locali, come `go test ./...`, `gofmt` e `go vet ./...`.

## Pattern operativi

- **Modifiche al package**:
  - aggiornare `README.md` quando cambia l'uso pubblico o l'autenticazione.
  - mantenere gli esempi coerenti con l'API esposta.
- **Bugfix e miglioramenti**:
  - spiegare cosa causa il problema e come la soluzione lo risolve.
  - non introdurre modifiche a file non necessari.
- **Struttura del repository**:
  - il repository è un SDK Go, quindi la logica deve restare leggera e focalizzata.
  - non aggiungere tool di deployment, pipeline o infrastruttura non rilevanti al progetto.

## Cosa evitare

- Non proporre `git push` o qualsiasi azione che alteri il repo remoto.
- Non generare file con dati sensibili o esempi reali di sessione Google Bard.
- Non cambiare le dipendenze in `go.mod` senza una valida ragione e relativa documentazione.
- Non suggerire l'aggiunta di librerie pesanti se il problema può essere risolto con il codice esistente.

## Puntatori

- `README.md` è il documento di riferimento per l'utente finale.
- `CLAUDE.md` descrive le regole generali di lavoro con il repository.
- Se serve una guida di contributo più ampia, proporre l'aggiunta di `CONTRIBUTING.md` e/o un file `docs/` dedicato.
