# Reference: Root Helper Socket API (`allod-helperd`)

The `allod-helperd` root daemon listens on a local UNIX domain socket (`/run/allod/helper.sock`, mode `0660`, group `allod`) and accepts JSON-encoded requests for privileged administrative actions.

## Security & Peer Authentication

1. **UNIX Socket Permissions (`0660`)**: The socket file `/run/allod/helper.sock` is strictly owned by `root:allod` with file mode `0660` (`-rw-rw----`). Unprivileged processes that are not members of group `allod` cannot open or write to the socket (receiving `EACCES` permission denied).
2. **Kernel-Level Caller Verification (`SO_PEERCRED`)**: On Linux, every incoming socket connection is checked at the kernel level via `SO_PEERCRED` (`unix.GetsockoptUcred`). The helper verifies that the caller's effective UID is `0` (`root`) or that the caller's UID belongs to group `allod`. Calls from unauthorized UIDs are rejected with `{"ok":false,"error":"caller not in group allod"}` and logged to stdout/journal.
3. **Audit Logging**: Every incoming request is logged to systemd journal with structured fields: `time`, `uid`, `action`, `args_hash` (truncated SHA-256 of parameters), `ok`, and `applied`.
4. **No Unauthenticated Fallbacks**: There is no unauthenticated TCP fallback. Communication strictly occurs over the authenticated UNIX domain socket.

---

## Request & Response Format

### Request
```json
{
  "action": "shares.apply",
  "plan": false,
  "args": {
    "name": "documents",
    "path": "/mnt/allod-storage/shares"
  }
}
```

### Response
```json
{
  "ok": true,
  "applied": true,
  "plan": [
    "mkdir -p /mnt/allod-storage/shares && chmod -R 0777 /mnt/allod-storage/shares",
    "configure share [documents] at /mnt/allod-storage/shares in /etc/samba/smb.conf",
    "systemctl restart smbd"
  ]
}
```

---

## Closed Actions Whitelist (16 Actions)

The root helper operates exclusively on a closed whitelist of 16 actions defined in `internal/helper/server.go` (`AllowedActions`) and mirrored in `schemas/helper-api.schema.json`:

| Action | Purpose | Arguments | What Runs as Root |
| :--- | :--- | :--- | :--- |
| `shares.apply` | Configure Samba share and directory permissions | `name` (regex), `path` (allowed path), `enabled` (bool) | `mkdir -p`, `chmod`, modifies `/etc/samba/smb.conf`, restarts `smbd` |
| `shares.bind_photos` | Bind-mount Immich photo library into Samba shares | `username` (optional regex), `enabled` (bool) | `mkdir -p`, `mount --bind`, `umount`, `chmod` |
| `shares.set_password` | Configure Samba password and private share for user | `username` (regex), `password` (string) | `useradd` (if missing), `smbpasswd -a -s`, `chmod`, `chown`, `systemctl reload smbd` |
| `users.create` | Create unprivileged Linux user for share authentication | `username` (regex) | `useradd -M -s /usr/sbin/nologin <user>`, `mkdir -p`, `chmod 0770` |
| `users.passwd` | Alias for `shares.set_password` | `username` (regex), `password` (string) | `useradd` (if missing), `smbpasswd -a -s`, `chmod`, `chown`, `systemctl reload smbd` |
| `firewall.apply` | Reload host firewall configuration | *(none)* | `nftables reload /etc/allod/nftables.conf` |
| `snapshots.create` | Create read-only Btrfs subvolume snapshot | `subvolume` (optional regex, default: `data`) | `btrfs subvolume snapshot /data/<subvol> /data/.snapshots/<subvol>` |
| `snapshots.prune` | Delete expired Btrfs subvolume snapshots | *(none)* | `btrfs subvolume delete` on expired snapshots |
| `smart.read` | Query SMART health status of a physical storage drive | `disk` (regex device / ID) | `smartctl -H /dev/disk/by-id/<disk>` |
| `service.restart` | Restart an allowed Allod systemd service | `unit` (regex from `AllowedServiceUnits`) | `systemctl restart <unit>` (special update logic for `allod-helperd`) |
| `storage.init` | Format disks and create initial Btrfs pool structure | `disks` (list/string of regex devices), `mode` (`single`\|`raid1`), `mount` (allowed path), `user` (optional regex) | `umount`, `mkfs.btrfs`, `mount`, `mkdir -p`, `chmod`, `chown` |
| `storage.diagnostics` | Collect Btrfs filesystem usage and device stats | `mount` (optional allowed path) | `btrfs filesystem usage`, `btrfs device stats`, `btrfs filesystem df` |
| `network.headscale_cli` | Execute controlled Headscale coordination commands | `command` (`preauthkey_create`\|`nodes_list`\|`users_list`) | `podman exec <container> headscale ...` (or host binary fallback) |
| `network.preauthkey_create` | Direct action alias: generate Headscale pre-auth key | *(none)* | `podman exec <container> headscale preauthkeys create -u default ...` |
| `network.nodes_list` | Direct action alias: query Headscale registered nodes | *(none)* | `podman exec <container> headscale nodes list --output json` |
| `network.users_list` | Direct action alias: list Headscale mesh users | *(none)* | `podman exec <container> headscale users list --output json` |

---

## Threat Notes & Validation per Action

### 1. `shares.apply`
* **Input & Validation**: `name` must match `^[a-zA-Z0-9_-]{1,64}$` (defaults to `"shares"`). `path` must be within `/mnt/allod-storage`, `/data`, or the configured `ALLOD_STORAGE_DIR`, strictly disallowing directory traversal (`..`).
* **Compromised Panel Impact**: An attacker controlling `allod-panel` could alter Samba share definitions or permissions within `/mnt/allod-storage`. They cannot access arbitrary system paths (`/etc`, `/root`, `/bin`).
* **Mitigations**: Strict path allowlist validation; system binary paths like `/usr/local/bin` are protected and prevented from being exposed as Samba shares.
* **Idempotency**: Yes, reapplying identical share settings leaves configuration and filesystem in a consistent state.
* **Support for `plan: true`**: Yes, outputs planned commands without filesystem or Samba mutations.

### 2. `shares.bind_photos`
* **Input & Validation**: `username` (if non-empty) is checked against `^[a-zA-Z0-9_-]{1,64}$`.
* **Compromised Panel Impact**: Could bind or unbind photo library mount points under `/mnt/allod-storage/shares`. Cannot mount arbitrary host devices or external paths.
* **Mitigations**: Source and target paths are hardcoded under `/mnt/allod-storage/photos` and `/mnt/allod-storage/shares`.
* **Idempotency**: Yes, checks existing `/proc/mounts` and unmounts prior targets before applying new bind mounts.
* **Support for `plan: true`**: Yes.

### 3. `shares.set_password` & 5. `users.passwd`
* **Input & Validation**: `username` must match `^[a-zA-Z0-9_-]{1,64}$`. `password` cannot be empty.
* **Compromised Panel Impact**: Could change Samba credentials for an existing share user or create a new user.
* **Mitigations**: Linux user created with `/usr/sbin/nologin` (no interactive shell or SSH login). Passwords are provided via standard input to `smbpasswd`, preventing command-line argument leakage in process listings (`ps`).
* **Idempotency**: Yes, updates existing passwords or registers users transparently.
* **Support for `plan: true`**: Yes.

### 4. `users.create`
* **Input & Validation**: `username` must match `^[a-zA-Z0-9_-]{1,64}$`.
* **Compromised Panel Impact**: Could create unprivileged Linux accounts.
* **Mitigations**: Created strictly as system accounts without home directory (`-M`) and with `/usr/sbin/nologin`.
* **Idempotency**: Yes, succeeds if the user already exists.
* **Support for `plan: true`**: Yes.

### 6. `firewall.apply`
* **Input & Validation**: No input parameters accepted.
* **Compromised Panel Impact**: Could trigger a firewall reload.
* **Mitigations**: Reloads strictly `/etc/allod/nftables.conf`, preventing arbitrary firewall rules from being injected via socket.
* **Idempotency**: Yes.
* **Support for `plan: true`**: Yes.

### 7. `snapshots.create`
* **Input & Validation**: `subvolume` must match `^[a-zA-Z0-9_-]{1,64}$` (defaults to `"data"`).
* **Compromised Panel Impact**: Could generate snapshot subvolumes consuming disk space.
* **Mitigations**: Traversal is prevented; snapshot creation targets are restricted to `/data/.snapshots/<subvol>`.
* **Idempotency**: Yes (new snapshots receive timestamps or unique target identifiers).
* **Support for `plan: true`**: Yes.

### 8. `snapshots.prune`
* **Input & Validation**: No input arguments.
* **Compromised Panel Impact**: Could prune expired snapshot retention sets.
* **Mitigations**: Operates only on managed snapshot directories according to retention policy.
* **Idempotency**: Yes.
* **Support for `plan: true`**: Yes.

### 9. `smart.read`
* **Input & Validation**: `disk` must match valid device pattern (`^[a-zA-Z0-9_-]{2,64}$`).
* **Compromised Panel Impact**: Read-only query; an attacker could inspect SMART telemetry for disks.
* **Mitigations**: Strictly passes sanitized identifier to `smartctl -H /dev/disk/by-id/<disk>`.
* **Idempotency**: Yes (read-only).
* **Support for `plan: true`**: Yes.

### 10. `service.restart`
* **Input & Validation**: `unit` must match `^[a-zA-Z0-9_.-]{1,64}$` AND exist in `AllowedServiceUnits` (`allod-helperd`, `allod-panel`, `smbd`, `smb`, `network`, `network-headscale`, `network-cloudflared`, `cloud`, `cloud-postgres`, `photos`, `photos-postgres`, `photos-valkey`, `media`, `backup`, `storage`, `nftables`).
* **Compromised Panel Impact**: Could restart whitelisted Allod services, causing temporary service interruption.
* **Mitigations**: Closed allowlist of units; an attacker cannot restart arbitrary host services (e.g., `ssh`, `systemd-journald`, `login`).
* **Idempotency**: Yes.
* **Support for `plan: true`**: Yes.

### 11. `storage.init`
* **Input & Validation**: `disks` must be a list or comma-separated string of identifiers matching `validDeviceRegex`. `mode` must strictly be `"single"` or `"raid1"`. `mount` must satisfy `isAllowedPath`. `user` must match `validNameRegex`.
* **Compromised Panel Impact**: Reformatting storage pool. This is the most destructive action in the system.
* **Mitigations**: Protected by `allod` group authorization, kernel `SO_PEERCRED`, strict regex on disk paths, and confirmation barriers in the UI. Disk identifiers are validated to avoid touching root OS partitions.
* **Idempotency**: Yes, re-initializes storage to a known clean state.
* **Support for `plan: true`**: Yes, details the wipe and mkfs commands without executing them.

### 12. `storage.diagnostics`
* **Input & Validation**: `mount` must satisfy `isAllowedPath` (defaults to `/mnt/allod-storage`).
* **Compromised Panel Impact**: Read-only inspection of Btrfs filesystem usage and error counters.
* **Mitigations**: Read-only execution of `btrfs filesystem usage`, `device stats`, and `df`.
* **Idempotency**: Yes (read-only).
* **Support for `plan: true`**: Yes.

### 13. `network.headscale_cli`, 14. `network.preauthkey_create`, 15. `network.nodes_list`, 16. `network.users_list`
* **Input & Validation**: `command` is strictly restricted to an enum: `"preauthkey_create"`, `"nodes_list"`, or `"users_list"`. Direct action aliases require no arguments.
* **Compromised Panel Impact**: Could generate a 1-hour single-use pre-auth key for joining the mesh or list connected mesh nodes and users.
* **Mitigations**: Pre-auth keys are configured as single-use with an expiration of 1 hour. No arbitrary Headscale commands (such as deleting nodes or altering coordination settings) can be executed.
* **Idempotency**: `nodes_list` and `users_list` are read-only. `preauthkey_create` creates a single ephemeral key.
* **Support for `plan: true`**: Yes.
