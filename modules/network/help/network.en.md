# Network Module (Remote Access & Sovereign Mesh)

The `network` module manages secure remote access to Immich, Jellyfin, Nextcloud, Samba, and the Allod Web Dashboard from smartphones, laptops, and tablets worldwide without opening incoming ports on your home router using [NetBird](https://netbird.io).

---

## Active Operational Levels

### `cloud` (NetBird European Cloud — 40 MB Allocated RAM — Recommended)
* **Hosted in Frankfurt, Germany**: 100% compliant with European GDPR regulations.
* **Zero Infrastructure Overhead**: Connects to NetBird's managed signaling service using an ephemeral Setup Key (`NB_SETUP_KEY`).
* **Automated WebRTC NAT Traversal**: Connects through CGNAT, Starlink, and mobile firewalls with zero open router ports.
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
