# Network Module (Remote Access & Mesh)

The `network` module manages secure remote access to Immich, Jellyfin, and the Allod Web Dashboard without opening any firewall ports on your home router.

## Available Modes

1. **`hybrid` (Headscale + Cloudflare Shield - Recommended)**:
   * Private Headscale control plane with unlimited nodes.
   * Outbound Cloudflare Tunnel protecting the coordination endpoint.
   * Direct P2P WireGuard data plane bypassing Cloudflare for 4K video and photo syncing.
2. **`wireguard` (Pure Sovereign)**:
   * Native kernel WireGuard with instant cryptographic QR Code pairing.
   * Zero third parties, zero cloud accounts.
3. **`tailscale` (Zero-Click Cloud)**:
   * Connection to the hosted Tailscale control server.
