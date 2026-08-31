# Allod — Piano PoC in 7 Tappe

Ogni tappa produce qualcosa di utilizzabile e ha un criterio di uscita verificabile.
Lo sforzo è in «sessioni» di tre o quattro ore.
Il totale realistico è fra le 30 e le 40 sessioni, cioè 4-6 mesi di serate non continuative.

## M0 — Le due interfacce che non potrai più cambiare (2 sessioni)

- [ ] `module.schema.json`: lo schema formale del manifest
- [ ] Specifica API dell'helper: elenco chiuso delle azioni
- [ ] Tre manifest: `storage`, `shares`, `photos`

**Criterio di uscita:** i tre manifest validano contro lo schema, e la descrizione del modulo `photos` con i suoi tre livelli sta interamente nel file senza bisogno di codice speciale.

## M1 — CLI minima che genera Quadlet (5 sessioni)

- [ ] `allod plan` e `allod apply` su tre moduli
- [ ] `config.yaml` come unica fonte di verità; `state.db` per lo stato applicato
- [ ] Pacchetto `.deb` grezzo e repository apt firmato

**Criterio di uscita:** da una VM Ubuntu vuota, con la sola CLI, si arriva a un archivio condiviso funzionante e al backup foto dal telefono. Un secondo `apply` non cambia nulla.

## M2 — Livelli, preflight, doctor (4 sessioni)

- [ ] Verifica preventiva delle risorse
- [ ] `allod set photos=full` rifiutato con messaggio motivato
- [ ] `allod doctor` con link alla pagina giusta

**Criterio di uscita:** su una macchina con 8 GB, `allod set photos=full` viene rifiutato con messaggio chiaro; con `--accept-risk` passa e appare il badge.

## M3 — Pannello e helper (8 sessioni)

- [ ] Helper root con lista chiusa e validazione
- [ ] Pannello Go + SQLite + asset incorporati
- [ ] Generatore cloud-init
- [ ] Unità di ripristino del pannello

**Criterio di uscita:** una persona non tecnica completa l'installazione senza aiuto.

## M4 — Backup verso i peer, due nodi (5 sessioni + seeding)

- [ ] Backup restic append-only verso un peer
- [ ] Ripristino verificato: hash coincidono
- [ ] `restic forget` con credenziali del server **fallisce**

**Criterio di uscita:** append-only verificato con prova reale.

## M5 — Monitoraggio e aggiornamenti (6 sessioni)

- [ ] Modulo `watch` in due confezioni + suite di conformità
- [ ] Macchina a stati degli aggiornamenti con rollback
- [ ] Digest positivo giornaliero

**Criterio di uscita:** allarme entro 26 ore dal timer disattivato; rollback automatico da immagine rotta; digest puntuale per 3 mattine.

## M6 — Il gruppo, tre nodi (6 sessioni)

- [ ] `allod ring simulate --remove <membro>` con piano corretto
- [ ] Ogni dataset critico con 2 repliche verificate
- [ ] Ingresso del terzo membro senza riconfigurare i primi due

## M7 — Pubblicazione (4 sessioni)

- [ ] Documentazione in inglese, struttura obbligatoria cap. 11
- [ ] AGPL-3.0, CLA, SECURITY.md, `allod sbom`
- [ ] Verifica finale del nome, domini, repository pubblico
- [ ] Pagina per i partecipanti

> **Se il tempo è poco:** M0, M1 e M4 (solo documenti) danno la parte irrinunciabile in ~10 sessioni.
