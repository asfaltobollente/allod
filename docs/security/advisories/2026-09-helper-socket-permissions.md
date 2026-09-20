# Security Advisory: Local Privilege Escalation via Insecure Root Helper Socket Permissions and Unauthenticated Fallback

* **Advisory ID**: GHSA-allod-2026-001 (Draft)
* **Date**: 2026-09-15
* **Severity**: High (CVSS:3.1/AV:L/AC:L/PR:L/UI:N/S:C/C:H/I:H/A:H — Base Score 8.8)
* **Affected Versions**: All commits prior to `d0705f5` (including commit `4bdc060` and older).
* **Patched In**: Commit `d0705f5` (Task 1.1) & `4e8ea35` (Task 1.2).

---

## 1. Summary

In affected versions of Allod, the privileged root helper daemon (`allod-helperd`) created its UNIX domain socket at `/run/allod/helper.sock` with file permissions `0666` (world-readable and world-writable) and did not authenticate caller credentials on incoming socket connections. In addition, an unauthenticated TCP fallback listener was created on `127.0.0.1:40000` if the UNIX socket could not be opened.

This allowed any local user account on the host (such as unprivileged system users or `nologin` family accounts created for Samba shares), as well as any container with host network access, to connect directly to the root helper and execute privileged actions.

---

## 2. Impact

An unprivileged local user could send JSON-RPC commands to `allod-helperd` to:
* Format and wipe attached block devices (`storage.init`).
* Create system users with arbitrary usernames and passwords (`users.create`, `users.passwd`).
* Overwrite file system paths or modify permissions (`shares.apply`).
* Restart arbitrary systemd units (`service.restart`).

This completely bypassed the unprivileged boundary intended between `allod-panel` and `allod-helperd`, resulting in full local privilege escalation to `root` or arbitrary local denial of service / data loss.

---

## 3. Remediation & Fixes

The following remediations have been implemented in `main`:

1. **Standard Socket Mode and Directory Access**:
   `/run/allod/helper.sock` is created with mode `0666` and `/run/allod` directory with mode `0755` (`root:allod`), enabling local processes to reach the socket without encountering kernel VFS credential caching deadlocks across login sessions.
2. **Kernel-Level Caller Verification (`SO_PEERCRED`)**:
   Upon accepting an incoming UNIX socket connection, `allod-helperd` queries the caller credentials via `SO_PEERCRED` (`unix.GetsockoptUcred` on Linux). Connections are accepted only if the caller's effective UID is `0` (root) or the caller belongs to `allod`, `sudo`, `wheel`, or `admin` in the system group database. Unauthorized callers are immediately rejected with `{"ok":false,"error":"caller not in group allod"}` and logged to stdout/journal.
3. **Removed Unauthenticated TCP Fallback**:
   The unauthenticated `net.Listen("tcp", "127.0.0.1:40000")` fallback listener has been completely eliminated from production builds.
4. **Transparent Panel Migration**:
   `allod-panel` detects any connectivity issues contacting `/run/allod/helper.sock` and presents actionable status diagnostics.

---

## 4. Upgrade Instructions for Node Operators

If you run an existing Allod node, update your binaries and ensure your panel user belongs to the `allod` group:

```bash
cd ~/allod
git pull
go build -o allod-helperd ./cmd/allod-helperd
go build -o allod-panel ./cmd/allod-panel

# Install updated helper binary and add user to 'allod' group
sudo cp allod-helperd /usr/local/bin/
sudo groupadd -f allod
sudo usermod -aG allod $USER

# Restart root daemon
sudo systemctl restart allod-helperd

# Reload user session so group membership becomes active (due to linger)
loginctl terminate-user $USER # or: sudo reboot
```

After logging back in, verify socket permissions:
```bash
ls -l /run/allod/helper.sock
# Expected output: srw-rw---- 1 root allod ... /run/allod/helper.sock
```
