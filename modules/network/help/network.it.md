# Modulo Network (Accesso Remoto & Mesh)

Il modulo `network` gestisce la connettività sicura fuori casa per accedere a Immich, Jellyfin e alla Web Dashboard senza aprire alcuna porta sul router.

## Modalità Disponibili

1. **`hybrid` (Headscale + Cloudflare Shield - Consigliata)**:
   * Control plane Headscale privato, dispositivi illimitati.
   * Tunnel Cloudflare come scudo per la segnalazione (zero porte aperte).
   * Dati e streaming video P2P diretti WireGuard fuori da Cloudflare.
2. **`wireguard` (Sovrana Pura)**:
   * WireGuard nativo nel kernel con generazione di QR Code.
   * Zero terze parti, zero account cloud.
3. **`tailscale` (Zero-Click Cloud)**:
   * Connessione al coordinatore ufficiale di Tailscale.
