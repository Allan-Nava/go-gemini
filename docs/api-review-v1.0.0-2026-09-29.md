# Revisione dell'API prima della v1.0.0 — 2026-09-29

Dalla v1.0.0 l'API esportata di `gogemini` non cambia più in modo incompatibile (il job `api-compat`
lo blocca). Questa è la revisione fatta sulla superficie della v0.4.0 prima del congelamento.

## Superficie

```
New(opts...) ──► *Client ──┬─ GenerateContent(ctx, prompt)        (*Response, error)
                           ├─ Generate(ctx, *GenerateContentRequest)
                           ├─ GenerateContentStream(ctx, prompt)  iter.Seq2[*Response, error]
                           ├─ GenerateStream(ctx, req)
                           ├─ NewChat(history...) ──► *Chat ── Send(ctx, text) · History()
                           └─ Model()

Option: WithAPIKey · WithModel · WithBaseURL · WithHTTPClient · WithTimeout
        WithRetry(RetryPolicy) · WithSystemInstruction · WithGenerationConfig
Tipi:   Content · Part · GenerateContentRequest · GenerationConfig · Response · Candidate
        PromptFeedback · UsageMetadata · RetryPolicy · APIError (Reason, RetryDelay, Retryable)
Errori: ErrMissingAPIKey · ErrInvalidBaseURL · ErrInvalidTimeout · ErrEmptyModel
        ErrInvalidRetryPolicy · ErrEmptyRequest · ErrRedirectOtherHost
Altro:  Ptr[T] · DefaultRetryPolicy() · DefaultBaseURL · APIKeyEnv
```

## Criterio

Per ogni simbolo: si potrà estendere senza rompere? Aggiungere un campo a una struct, una funzione,
un'opzione o un errore è compatibile. Non lo sono: rimuovere o rinominare, cambiare una firma,
**cambiare il valore di una costante esportata** (verificato con `apidiff`).

## Trovato e corretto prima del tag

| # | Problema | Correzione |
|---|---|---|
| 1 | **Gli stream venivano interrotti al timeout** (60 s): `http.Client.Timeout` comprende la lettura del body. Riprodotto: con `WithTimeout(250ms)` uno stream di 6 chunk si fermava a 3. | Il client di default non ha più `http.Client.Timeout`. L'SDK limita ogni tentativo con il context: per `Generate` richiesta e risposta intera, per uno stream solo l'avvio. Un tentativo scaduto viene ritentato; la scadenza del ctx del chiamante no. |
| 2 | `DefaultTimeout` era una costante esportata: dopo la v1.0.0 il valore non si sarebbe più potuto cambiare. | Non esportata (`defaultTimeout`), come `defaultModel` e `version`. |
| 3 | 60 s per tentativo è poco per i modelli che ragionano a lungo. | Default 5 minuti (decisione dell'utente). |

## Esaminato e tenuto

- **Import path** `github.com/Allan-Nava/go-gemini/gogemini`: la ripetizione è brutta, ma lo stesso
  schema di `MistServer-go-sdk`; spostare il package alla radice romperebbe ogni import.
- **`Option func(*Client)`**: una funzione sui campi non esportati; non espone niente. La validazione
  resta in `New`, che è dove un errore di configurazione deve emergere.
- **`NewChat(history ...Content)`**: senza opzioni per chat. Per istruzioni di sistema diverse si crea
  un altro `Client` (costa poco e condivide il transport se si passa lo stesso `http.Client`).
  Aggiungere opzioni più avanti richiederebbe un costruttore nuovo, accettato.
- **`Part` solo testo**, `Candidate` senza `SafetyRatings`, richiesta senza `SafetySettings`/`Tools`:
  campi che si aggiungono in 1.x senza rompere.
- **`Response.Text()`** restituisce il testo del primo candidato: semantica documentata e stabile.
- **Modello di default** `gemini-3.8-flash` (decisione dell'utente): due `503` in due prove, ma è quello
  che Google consiglia; il retry assorbe i picchi e il default può cambiare in una minor.
- **`DefaultBaseURL`, `APIKeyEnv`**: valori stabili, restano costanti esportate.

## Dopo il congelamento (v1.1.0, 2026-09-30)

Il primo lavoro sulla 1.x ha mostrato un caso che la revisione non aveva considerato: **aggiungere
un campo slice a una struct esportata confrontabile è incompatibile** (`gorelease`: "old is
comparable, new is not"). È successo con `Client` (campo privato: risolto con un puntatore) e con
`PromptFeedback.SafetyRatings` (campo esportato: non aggiunto).

## Verifiche

- Test con `-race`: copertura 98,0%; cinque mutazioni della logica del timeout, tutte intercettate.
- `gorelease` contro `v0.4.0`: rimozione di `DefaultTimeout` segnalata come incompatibile, ammessa
  nel passaggio da 0.x a 1.0.0.
