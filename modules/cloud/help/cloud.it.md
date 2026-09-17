# Modulo Cloud (Nextcloud Hub)

Il modulo `cloud` integra Nextcloud Hub per la sincronizzazione file, WebDAV, Calendario (CalDAV), Rubrica (CardDAV) e collaborazione su documenti.

## Porte e Credenziali

* **Porta Web UI**: `8080` (HTTP).
* **Database**: Container PostgreSQL 16 Alpine dedicato (`cloud-postgres`).
* **Credenziali Database**: Generate dinamicamente e salvate nel secret `~/.config/allod/secrets/cloud-db.env` (permessi `0600`).
* **Archiviazione Dati**: Salvati in `/mnt/allod-storage/cloud/data` (o `~/.local/share/allod/cloud/data`).
