# Allod

**Cloud personale in piena proprietà: modulare, senza abbonamenti, senza porte aperte.**
Utile con un nodo solo; se c'è un amico, diventa un gruppo di backup reciproco che protegge i dati di entrambi da un disastro.

*Documento di concetto e piano di Proof of Concept — versione 2.1 — 18 agosto 2026. Sostituisce la versione 1.0. Sul nome e sulle verifiche ancora da fare, vedi la sezione 1.5.*

> I tuoi dati in piena proprietà, con la stessa comodità di prima. *Your data, held in full.*

| Voce              | Sintesi                                                                                                    |
|-------------------|------------------------------------------------------------------------------------------------------------|
| Cos'è             | Uno strato di orchestrazione su Podman: moduli dichiarativi, pannello web, documentazione integrata        |
| Cosa non è        | Non è un sistema operativo, non è un servizio, non ha account centrali né telemetria                       |
| Baseline del core | Intel Core i3 4ª gen, 8 GB RAM, 2 dischi — deve girare qui, non solo su hardware nuovo                     |
| Piattaforme       | x86-64 e ARM64 (Raspberry Pi 5, mini-PC Rockchip)                                                          |
| Licenza           | AGPL-3.0, pubblicato su Git. Ricavi: solo donazioni libere senza controprestazione                         |
| Installazione     | Pacchetto `.deb` da repository firmato più file cloud-init generato: un amico non tecnico ce la fa da solo |

## I tre livelli di servizio

| Livello   | RAM    | Cosa comprende                                                                                                                                                 |
|-----------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **CORE**  | 1,5 GB | Archivio condiviso, backup automatico delle foto dal telefono, snapshot locali, accesso da fuori casa, pannello. Gira su un i3 del 2013 e su un Raspberry Pi 5 |
| **PLUS**  | 4 GB   | Aggiunge media server con transcodifica hardware, accesso web ai file e il backup reciproco con gli amici. Richiede una iGPU con encoder                       |
| **ULTRA** | 16 GB  | Aggiunge ricerca semantica e riconoscimento volti sulle foto, più utenti in parallelo, gruppi ampi. Richiede AVX2, GPU opzionale                               |

I livelli non sono edizioni commerciali: sono la stessa installazione con moduli diversi accesi. Si sale e si scende con un comando, e il sistema rifiuta ciò che l'hardware non regge spiegando perché.
