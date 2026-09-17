# Network Module (Remote Access & Sovereign Mesh)

The `network` module manages secure remote access to Immich, Jellyfin, Nextcloud, Samba, and the Allod Web Dashboard from smartphones, laptops, and tablets worldwide without opening incoming ports on your home router.

---

## Active Resource Level

### `hybrid` (Headscale + Cloudflare Shield — 120 MB Allocated RAM)
* **Private Control Plane**: Runs a self-hosted [Headscale](https://headscale.net) coordination server directly on your Allod node with embedded SQLite. You maintain complete ownership of user registries, cryptographic keys, and access control lists with unlimited devices and zero license restrictions.
* **Outbound Tunnel Shield**: Uses an outbound Cloudflare Tunnel (`cloudflared`) to securely expose Headscale's coordination endpoint. Operates seamlessly behind Carrier-Grade NAT (CGNAT), Starlink, 4G/5G mobile carriers, and campus firewalls with zero open router ports.
* **Direct Encrypted P2P Data Plane**: Cloudflare is used strictly for signaling JSON (authentication and peer discovery). Heavy data streams — 4K Jellyfin streaming, Immich photo/video camera backups, Nextcloud sync, and Samba transfers — flow directly peer-to-peer (P2P) between your client and your server over an end-to-end encrypted WireGuard tunnel, completely outside Cloudflare.
* **100% Cloudflare ToS Compliant**: Because media streaming and large file transfers never pass through Cloudflare edge servers, your connection complies fully with Cloudflare's Terms of Service with uncapped bandwidth.
* **Zero-Hassle Client Pairing**: Pairs with the free, official Tailscale apps on iOS, Android, macOS, Windows, and Linux via "Change server" and a single-use 1-hour pre-auth key generated in 1 click from the Allod Dashboard.

---

## Planned Levels (Future Roadmap)

* **`wireguard` (Pure Sovereign WireGuard — 📋 planned)**: Native kernel WireGuard with cryptographic QR code pairing, designed for setups with a public IP or manual UDP port forwarding.
* **`tailscale` (Zero-Click Cloud — 📋 planned)**: Direct client connection to Tailscale's hosted commercial control plane (`tailscale.com`).

---

## Security & Privacy Model

1. **Zero Open Inbound Ports**:
   * No port forwarding rules (such as 80, 443, or 22) are required on your router.
   * Eliminates exposure to public internet port scanners, brute-force bots, and DDoS attacks.
2. **Encrypted Secret Storage**:
   * Cloudflare Tunnel tokens are saved exclusively to `network/secrets/cloudflared.env` with restricted `0600` permissions and never inlined in container definition files.
3. **P2P WireGuard Encryption**:
   * All application traffic between your devices is end-to-end encrypted with state-of-the-art Noise protocol cryptography.
