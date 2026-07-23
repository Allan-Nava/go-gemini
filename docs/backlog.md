---
layout: default
title: Backlog
nav_order: 3
description: Backlog operativo per migliorare l'SDK go-gemini
last_modified_date: 2026-07-23T12:00:00+0000
---

# Backlog

Elenco delle attività proposte per migliorare `go-gemini`.

## 1. Stabilità del client Bard

- [ ] Implementare `GetAnswer()` in `gogemini/gemini.go`.
- [ ] Estrarre `SNlM0e` dalla pagina di Bard o da risposta API in modo robusto.
- [ ] Gestire gli header di sessione e cookie in modo configurabile, evitando hardcode non necessari.
- [ ] Documentare il flusso di autenticazione in `README.md` e `docs/index.md`.

## 2. Test e copertura

- [ ] Aggiungere test unitari per `gogemini.restyPost` e `restyGet`.
- [ ] Aggiungere test per `configuration.GetConfiguration()` con variabili d'ambiente.
- [ ] Aggiungere test di integrazione simulata per il flusso di richiesta/resposta.
- [ ] Verificare `go test ./...` su tutto il repository.

## 3. Documentazione

- [ ] Creare `CONTRIBUTING.md` con linee guida per i contributi.
- [ ] Aggiungere esempi di utilizzo nel `README.md`.
- [ ] Documentare le variabili di ambiente supportate e le modalità di esecuzione dei test.
- [ ] Aggiungere una sezione `Docs` nel sito Jekyll con la roadmap e il backlog.

## 4. Qualità del codice

- [ ] Applicare `gofmt -w .` al repository.
- [ ] Introdurre `go vet ./...` come requisito di build.
- [ ] Valutare l'adozione di `golangci-lint` o simili.
- [ ] Rifattorizzare il package `configuration` per chiarezza e sicurezza.

## 5. Esperienza sviluppatore

- [ ] Aggiungere supporto per `IS_DEBUG`, base URL personalizzata e chiavi API in `configuration.Configuration`.
- [ ] Rendere l'interfaccia `IGoGemini` più completa e documentata.
- [ ] Aggiungere helper per l'inizializzazione del client e per i retry.

## Note

- Il backlog deve essere aggiornato man mano che emergono nuove funzionalità o bug.
- Le attività più importanti rimangono quelle che consentono all'SDK di funzionare con Bard in modo affidabile e testabile.
