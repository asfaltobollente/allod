# How-to: Restore from a Peer after Ransomware or Hardware Loss

If your local server is completely destroyed, stolen, or encrypted by ransomware, your data is safe on your peers' nodes because all remote backups are stored in **immutable append-only mode**.

---

## 1. Why Ransomware Cannot Delete Remote Backups

When your node backs up to Marco or Sara, the remote rest-server runs with `--append-only`. If ransomware infects your node and sends `DELETE` commands to destroy remote backups, the peer's server immediately rejects the request with `403 Forbidden`.

---

## 2. Restore Steps on a New Node

### Step 1: Set up a replacement node
Follow the [Tutorial: First Node](../tutorial/first-node.md) on new hardware.

### Step 2: Connect to your peer's rest-server
Export your backup encryption password and connect to your friend's node over the mesh:

```bash
export RESTIC_PASSWORD="<your-secret-password>"
export RESTIC_REPOSITORY="rest:http://100.64.0.2:8000/"
```

### Step 3: Verify and list historical snapshots
List all snapshots stored on Marco's node:

```bash
restic snapshots
```

### Step 4: Restore to target directory
Restore the latest clean snapshot:

```bash
restic restore latest --target /data/
```

All your photos, documents, and subvolumes will be restored with exact SHA-256 hash matching.
