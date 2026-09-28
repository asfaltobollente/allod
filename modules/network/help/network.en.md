# Network Module (Remote Access & Sovereign Mesh)

The `network` module manages secure remote access to Immich, Jellyfin, Nextcloud, Samba, and the Allod Web Dashboard from smartphones, laptops, and tablets worldwide without opening incoming ports on your home router using [NetBird](https://netbird.io).

---

## Active Operational Levels

### `cloud` (NetBird European Cloud — 40 MB Allocated RAM — Recommended)
* **Hosted in Frankfurt, Germany**: 100% compliant with European GDPR regulations.
* **Zero Infrastructure Overhead**: Connects to NetBird's managed signaling service using an ephemeral Setup Key (`NB_SETUP_KEY`).
* **Automated WebRTC NAT Traversal**: Connects through CGNAT, satellite/FWA connections, and mobile firewalls with zero open router ports.
* **Direct Encrypted WireGuard P2P**: Heavy traffic (Jellyfin 4K streaming, Immich camera sync, Samba transfers) flows directly peer-to-peer between devices.

### `selfhosted` (Sovereign Management Server — 40 MB Allocated RAM)
* **100% Data & Control Plane Sovereignty**: Connects to your private self-hosted NetBird management server (e.g. `https://mesh.yourdomain.com:443`).
* **Zero Reliance on External Cloud**: Complete ownership of the peer registry, ACLs, and routing policies.
* **Enterprise Identity**: Support for custom OIDC/SAML providers (Keycloak, Authentik).

---

## Security & Privacy Model

1. **Zero Open Inbound Ports**:
   * No port forwarding rules (such as 80, 443, or 22) are required on your home router.
   * Eliminates exposure to internet port scanners, brute-force bots, and DDoS attacks.
2. **Encrypted Secret Storage**:
   * Setup keys and management URLs are saved exclusively to `network/secrets/netbird.env` with restricted `0600` permissions and never inlined in container definition files.
3. **P2P WireGuard Encryption**:
   * All application traffic between devices is end-to-end encrypted with state-of-the-art Noise protocol cryptography.

---

## 🌟 Recommended Setup: One IP Address Inside & Outside Home (Network Route)

To avoid changing server URLs in mobile apps (such as Immich and Jellyfin) when moving between home Wi-Fi and mobile 4G/5G:
* **Why not use `.local` over 4G**: Android and iOS strictly filter out mDNS `*.local` queries over mobile VPN connections according to RFC 6762.
* **The Seamless Solution (NetBird Network Route)**:
  1. In your NetBird dashboard, navigate to **Network Routes** ➔ **Add Route**.
  2. Set the *Network Range* to your Allod server's local LAN IP with `/32` (e.g. `192.168.1.50/32` or your full subnet `192.168.1.0/24`).
  3. Select your Allod node as the *Routing Peer* and *Distribution Groups*: `All`.
  4. In your mobile apps, configure the local IP directly (e.g. `http://192.168.1.50:2283` for Immich): it will connect seamlessly both at home on Wi-Fi and globally over 4G!

