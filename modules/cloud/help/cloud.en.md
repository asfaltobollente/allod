# Cloud Module (Nextcloud Hub)

The `cloud` module provides Nextcloud Hub for file sync, WebDAV, Calendar (CalDAV), Contacts (CardDAV), and collaborative editing.

## Endpoints & Credentials

* **Web UI Port**: `8080` (HTTP).
* **Database Engine**: Dedicated PostgreSQL 16 Alpine container (`cloud-postgres`).
* **Database Credentials**: Dynamically generated secure secrets stored in `~/.config/allod/secrets/cloud-db.env` (`0600` permissions).
* **Data Storage**: Stored in `/mnt/allod-storage/cloud/data` (or `~/.local/share/allod/cloud/data`).
