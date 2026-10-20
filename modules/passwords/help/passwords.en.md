# Passwords Module (Vaultwarden)

Vaultwarden is a lightweight, high-performance Bitwarden-compatible server written in Rust. Integrated into Allod, it provides self-hosted password and secrets management for your family and home lab, with seamless support for official Bitwarden browser extensions and mobile apps.

---

## Key Features

* **Zero-Knowledge Encryption**: All vault data (passwords, notes, TOTP seeds, cards) is encrypted and decrypted locally on the client device using AES-256 and PBKDF2/Argon2id. The server only stores encrypted ciphertext.
* **Full Bitwarden Ecosystem**: Compatible with official Bitwarden apps across iOS, Android, Windows, macOS, Linux, and all major browser extensions.
* **Minimal Resource Footprint**: The compiled Rust daemon in Alpine Linux typically uses only 30-50 MB RAM, with a system ceiling configured at 256 MB.
* **Two-Factor Authentication**: Built-in support for authenticator apps (TOTP) and FIDO2/WebAuthn hardware security keys.
* **Family Organizations**: Securely share Wi-Fi passwords, streaming accounts, and family documents in shared collections.

---

## Resource Levels

* **`off`**: Service disabled, container stopped, 0 MB RAM allocated.
* **`standard` (256 MB RAM allocated, 5 GB recommended storage)**:
  * Runs rootless Podman container on local host port 8088 (`PublishPort=8088:80`).
  * Embedded SQLite database and vault items stored on Btrfs pool at `/mnt/allod-storage/passwords/data`.
  * Real-time WebSocket synchronization enabled for instant updates across devices.

---

## HTTPS Requirement (WebCrypto API)

Modern web browsers and Bitwarden clients strictly enforce **HTTPS** to expose the cryptographic WebCrypto API (`window.crypto.subtle`).

To connect to your Vaultwarden instance:
1. **NetBird Sovereign Mesh (Recommended)**: Connect client devices to your private Allod NetBird mesh. WireGuard transport provides secure encryption.
2. **Local Reverse Proxy**: Forward incoming HTTPS traffic from Caddy or Nginx with a valid TLS certificate to `http://127.0.0.1:8088`.
3. **Localhost**: Local testing and initial configuration can be performed on the server via `http://localhost:8088`.

---

## Security & Backups

* **Allod Backup Dataset**: The directory `/mnt/allod-storage/passwords/data` is tracked as a critical dataset in Allod's Ring backup selector (`backup` module).
* **Secrets Configuration**: Environment settings and generated admin tokens are stored at `/mnt/allod-storage/passwords/secrets/vaultwarden.env` with `0600` permissions.
* **Restricted Signups**: Public self-registration is disabled by default (`SIGNUPS_ALLOWED=false`). New users are onboarded via administrator invitations (`INVITATIONS_ALLOWED=true`).
