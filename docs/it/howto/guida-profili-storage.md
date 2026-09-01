# 🧭 Guida alla Scelta dei Profili di Storage — Allod

> Come organizzare i tuoi dati sul pool RAID per ottenere la massima velocita, semplicita e sicurezza, senza mai duplicare i file.

---

## 🎯 Il Principio Fondamentale: Single Source of Truth

In Allod i tuoi file esistono **una sola volta** sul pool Btrfs RAID 1, e i vari servizi (`cloud`, `shares`, `photos`) vi si affacciano sopra come viste dedicate.

Per garantire stabilita ed evitare file corrotti o database disallineati, Allod adotta una regola d'oro:

> **« Un dato. Un proprietario per cartella. N viste con privilegi dedicati. »**

In base a come usi il tuo computer, il tuo smartphone e alle tue abitudini quotidiane, Allod supporta **3 profili di configurazione**. Ecco la guida per scegliere quello perfetto per te.

---

## ⚡ Come scegliere in 10 secondi

```
Ti serve condividere link pubblici con persone esterne (clienti, parenti) senza farli entrare in VPN?
│
├── 🟢 SI  ──→  PROFILO A (Nextcloud Monolite) oppure PROFILO C (Ibrido)
│
└── 🔴 NO  ──→  PROFILO B (Samba + Immich)  ⭐ [Consigliato per il 90% degli utenti home/LAN]
```

---

## ☁️ Profilo A — "Cloud" (Solo Nextcloud)

**Un solo servizio, un solo indice, un'unica app per tutto.**

In questo profilo attivi solo il modulo `cloud` (Nextcloud Hub 30 + PostgreSQL). Tieni disattivati `shares` e `photos`.

```
/mnt/allod-storage/
├── data/              ← Interamente gestito da Nextcloud (documenti, foto, note)
└── system-cloud/      ← Database PostgreSQL 16, appdata, anteprime
```

### 👤 Per chi e perfetto
* Chi vuole la **massima semplicita**: una sola applicazione sul telefono e sul PC.
* Chi lavora molto da fuori casa e ha bisogno di inviare **link pubblici protetti da password e scadenza** ad amici, clienti o colleghi.
* Chi vuole contatti, calendari e note sincronizzati tra tutti i dispositivi.

### 👍 Vantaggi (Pros)
* **Semplicita totale di gestione**: 1 solo servizio attivo, 1 solo database, zero conflitti di permessi.
* **Accesso da ovunque**: Progettato per HTTPS nativo e client mobile completi.
* **Desktop On-Demand (Virtual Files)**: Il client Nextcloud per Windows/Mac mostra tutti i Terabyte del NAS in Esplora Risorse occupando 0 Byte sul disco locale.
* **Suite completa integrata**: Calendario (CalDAV), Rubrica (CardDAV), Note e condivisioni.
* **Consumo risorse ridotto**: ~600 MB di RAM per l'intero sistema.

### 👎 Svantaggi & Limiti (Cons)
* **Velocita su file giganti**: Le operazioni passano dall'interprete PHP e dal database. Lo spostamento di file da 50 GB o cartelle con decine di migliaia di file e sensibilmente piu lento rispetto alla velocita di rete pura di Samba.
* **Gestione foto base**: L'app Nextcloud Photos visualizza le foto in una griglia temporale semplice, ma non ha l'Intelligenza Artificiale rapida, il riconoscimento facciale istantaneo o la ricerca semantica di livello commerciale.

---

## 📁📸 Profilo B — "Archivio" (Samba + Immich) — ⭐ Consigliato

**Velocita pura sul PC + Intelligenza Artificiale per le foto. Nessun conflitto di indici.**

In questo profilo attivi `shares` (Samba) per i computer e `photos` (Immich) per smartphone e tablet. Tieni disattivato `cloud`.

```
/mnt/allod-storage/
├── data/                         ← Root della condivisione Samba (\\allod\shares)
│   ├── Documenti/                ← Scrittura: Samba (PC Windows/Mac)
│   ├── Progetti/                 ← Scrittura: Samba (PC Windows/Mac)
│   └── images/
│       ├── Archivio/             ← Scrittura: Samba (Foto storiche/reflex)
│       │                            Lettura: Immich (Libreria esterna indicizzata con AI)
│       └── Telefono/             ← Scrittura: Immich (Backup automatico smartphone)
│                                    Lettura: Samba (Sola lettura per sicurezza)
└── system-photos/                ← Database pgvector, Valkey, miniature e modelli AI
```

### 👤 Per chi e perfetto
* Chi usa il NAS principalmente da casa o tramite VPN (WireGuard/Tailscale).
* Chi lavora su file pesanti (montaggio video 4K, grafica, archivi, ISO, musica) e vuole la **massima velocita di trasferimento LAN (115–280 MB/s)**.
* Chi tiene moltissimo alla propria libreria fotografica e vuole un'esperienza identica a **Google Foto / Apple Foto**.

### 👍 Vantaggi (Pros)
* **Velocita massima nativa**: Lavori su Windows/Mac come se il NAS fosse un hard disk interno, senza installare client pesanti.
* **Intelligenza Artificiale al top (Immich)**: Riconoscimento volti di tutta la famiglia, ricerca semantica (*"tramonto sulla neve"*), mappa geografica, video scrubber istantaneo.
* **Zero conflitti di locking**: Samba scrive sui documenti, Immich scrive sulle foto del telefono e legge l'archivio. I due mondi non si intralciano mai.
* **Topologia pulita ed elegante**: Minima manutenzione, zero disallineamenti di cache.

### 👎 Svantaggi & Limiti (Cons)
* **Niente link pubblici diretti**: Non puoi generare un link web per far scaricare un file a una persona esterna (a meno di non usare la condivisione album di Immich per le sole foto).
* **Accesso remoto richiede VPN**: Per aprire i file da fuori casa ti colleghi alla tua VPN personale protetta (es. WireGuard).
* **Consumo RAM**: Richiede circa 1.5–2 GB di RAM per i modelli di Machine Learning di Immich.

---

## 🔀 Profilo C — "Libero / Ibrido" (Nextcloud + Samba + Immich)

**Tutto insieme sullo stesso archivio unificato. Massima potenza con regole chiare.**

In questo profilo attivi tutti e tre i servizi (`cloud` + `shares` + `photos`) che puntano allo stesso pool dati `/mnt/allod-storage/data/`.

```
/mnt/allod-storage/
├── data/                         ← ARCHIVIO FISICO UNICO
│   ├── Documenti/                ← Samba (R/W) + Nextcloud External Storage (R/W)
│   ├── images/
│   │   ├── Archivio/             ← Samba (R/W) + Immich External (RO) + Nextcloud (R/W)
│   │   └── Telefono/             ← Immich Backup (R/W) + Samba (RO)
│   └── Progetti/
├── system-cloud/                 ← Metadati e DB Nextcloud
└── system-photos/                ← Metadati, DB AI e cache Immich
```

### 👤 Per chi e perfetto
* Utenti esperti / Prosumer che vogliono **tutte le funzionalita contemporaneamente**: velocita Samba su PC, interfaccia e link pubblici di Nextcloud, e galleria AI di Immich.

### 👍 Vantaggi (Pros)
* **Flessibilita totale a 360°**: Usi lo strumento migliore a seconda di dove ti trovi e del dispositivo che stai usando.
* **Dato unico reale**: Modifichi un file da Samba e lo ritrovi sincronizzato su Nextcloud e indicizzato su Immich.

### 👎 Svantaggi & Accortezze Necessarie (Cons)
* **Divergenza degli Indici**: Quando crei o sposti un file via Samba, Nextcloud e Immich devono aggiornare i rispettivi database tramite riconciliazione automatica (`files:scan` / inotify).
* **Gestione Permessi Rigorosa**: Richiede l'uso di permessi standardizzati (`keep-id` nei container e gruppo `allod-data` con SGID `2770`).
* **Consumo Risorse**: Richiede almeno **8–16 GB di RAM** sul server per far girare contemporaneamente i due database relazionali (PostgreSQL), la cache Valkey, i microservizi AI di Immich e PHP-FPM di Nextcloud.

---

## 📊 Tabella Comparativa di Sintesi

| Caratteristica | ☁️ Profilo A (Cloud) | 📁📸 Profilo B (Archivio) ⭐ | 🔀 Profilo C (Libero) |
| :--- | :---: | :---: | :---: |
| **Velocita LAN su file giganti** | ⚖️ Media (WebDAV) | ⚡ **Massima (SMB 115-280 MB/s)** | ⚡ **Massima (SMB)** |
| **Galleria Foto con AI (Volti, Mappa)** | ❌ Base | 🧠 **Eccellente (Immich)** | 🧠 **Eccellente (Immich)** |
| **Condivisione Link Pubblici a terzi** | 🔗 **Si (Nextcloud)** | ❌ Solo album foto | 🔗 **Si (Nextcloud)** |
| **Calendario, Rubrica e Note** | 📅 **Inclusi** | 🪶 Opzionale (Radicale/Baïkal) | 📅 **Inclusi** |
| **Backup Automatico Smartphone** | 📱 Nextcloud Sync | 📸 **Immich Backup nativo** | 📸 **Immich Backup nativo** |
| **Accesso da Fuori Casa** | 🌐 HTTPS Diretto | 🔒 Via VPN WireGuard | 🌐 Entrambi |
| **Servizi e Container da gestire** | 🪶 Pochi (Nextcloud+DB) | 🪶 Puliti (Immich+Samba) | 📦 Completi (Tutti) |
| **Rischio Disallineamento Indici** | 🟢 Zero | 🟢 Zero | 🟡 Richiede Daemon Watcher |
| **RAM Consigliata** | 4–8 GB | 6–8 GB | 8–16 GB |

---

## 💡 Il Consiglio di Allod
* Se il tuo NAS e per la **tua casa e la tua famiglia**: scegli il **Profilo B (Samba + Immich)**. E la combinazione piu veloce, piacevole da usare e a prova di bomba.
* Se usi il NAS principalmente come **ufficio remoto e scambio file con clienti**: scegli il **Profilo A (Nextcloud)**.
* Se vuoi **tutto senza compromessi** e hai almeno 8–16 GB di RAM: scegli il **Profilo C (Libero)**.