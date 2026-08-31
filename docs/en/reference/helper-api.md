# Reference: Root Helper Socket API (`allod-helperd`)

The `allod-helperd` root daemon listens on a local UNIX socket (`/run/allod/helper.sock`, mode `0660`, group `allod`) and accepts JSON-encoded requests for a closed list of 9 administrative actions.

---

## Request & Response Format

### Request
```json
{
  "action": "shares.apply",
  "plan": false,
  "args": {
    "name": "documents",
    "path": "/data/documents"
  }
}
```

### Response
```json
{
  "ok": true,
  "applied": true,
  "plan": [
    "systemctl stop smb-documents",
    "configure share documents at /data/documents",
    "systemctl start smb-documents"
  ]
}
```

---

## Closed Actions Table

| Action | Required Arguments | Description |
| :--- | :--- | :--- |
| `shares.apply` | `name` (regex), `path` (abs path) | Configures and restarts a Samba user share. |
| `users.create` | `username` (regex) | Creates a Linux user for network share access. |
| `users.passwd` | `username` (regex) | Updates credentials for a user. |
| `firewall.apply` | none | Reloads nftables firewall rules. |
| `snapshots.create` | `subvolume` (regex) | Creates a read-only btrfs subvolume snapshot. |
| `snapshots.prune` | none | Prunes expired btrfs subvolume snapshots. |
| `smart.read` | `disk` (serial ID) | Reads SMART health status from physical drive. |
| `service.restart` | `unit` (regex) | Restarts a specified system service unit. |
| `storage.init` | `serial` (serial ID) | Wipes and formats a proven-empty disk. |
