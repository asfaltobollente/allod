# Backup Module (Restic & rest-server)

The `backup` module orchestrates federated backups across trusted peer nodes in your Allod Ring using Restic and `rest-server`.

## Architecture & Endpoints

* **Backend Engine**: `docker.io/restic/rest-server:0.12.1`.
* **Port**: `8000` (HTTP REST protocol for restic clients).
* **Storage Location**: Dedicated subvolume at `/mnt/allod-storage/backup` (or `~/.local/share/allod/backup`).

## Security & Known Limitations

> [!WARNING]
> **No Authentication in v0.12.1 Image**: The upstream `rest-server:0.12.1` container runs in `--no-auth` mode without per-peer `.htpasswd` isolation. All peers reachable on port 8000 can append and read snapshots.

### Mitigations:
1. **Private Mesh Overlay Only**: Only expose port 8000 over NetBird sovereign mesh (`100.64.0.0/10` / `10.42.0.0/16`). Do not forward port 8000 on WAN routers.
2. **Client-Side Encryption**: Restic encrypts all repository snapshots client-side using AES-256 before transferring data over the network.
3. **Roadmap**: Federated peer authentication with per-peer htpasswd secrets and automated snapshot scheduling is actively under development.
