# Reference: Root Helper Socket API (`allod-helperd`)

The `allod-helperd` root daemon listens on a local UNIX domain socket (`/run/allod/helper.sock`, mode `0660`, group `allod`) and accepts JSON-encoded requests for privileged administrative actions.

## Security & Peer Authentication

1. **UNIX Socket Permissions (`0660`)**: The socket file `/run/allod/helper.sock` is strictly owned by `root:allod` with file mode `0660` (`-rw-rw----`). Unprivileged processes that are not members of group `allod` cannot open or write to the socket (receiving `EACCES` permission denied).
2. **Kernel-Level Caller Verification (`SO_PEERCRED`)**: On Linux, every incoming socket connection is checked at the kernel level via `SO_PEERCRED` (`unix.GetsockoptUcred`). The helper verifies that the caller's effective UID is `0` (`root`) or that the caller's UID belongs to group `allod`. Calls from unauthorized UIDs are rejected with `{"ok":false,"error":"caller not in group allod"}` and logged to stdout/journal.
3. **No Unauthenticated Fallbacks**: There is no unauthenticated TCP fallback. Communication strictly occurs over the authenticated UNIX domain socket.

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
