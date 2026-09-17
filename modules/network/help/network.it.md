# Modulo Network (Accesso Remoto & Sovereign Mesh)

Il modulo `network` gestisce la connettività sicura da remoto per accedere a Immich, Jellyfin, Nextcloud, Samba e alla Web Dashboard di Allod da smartphone, computer e tablet in tutto il mondo senza aprire porte in ingresso sul router di casa.

---

## Livello Risorse Attivo

### `hybrid` (Headscale + Cloudflare Shield — 120 MB RAM Allocata)
* **Control Plane Privato Sovrano**: Esegue un server di coordinamento [Headscale](https://headscale.net) direttamente sul tuo nodo Allod con database SQLite integrato. Mantieni il controllo totale su registro utenti, chiavi crittografiche e liste di accesso (ACL) con dispositivi illimitati e zero limitazioni commerciali.
* **Scudo Tunnel in Uscita**: Sfrutta un Cloudflare Tunnel in uscita (`cloudflared`) per esporre in modo sicuro l'endpoint di coordinamento di Headscale. Funziona istantaneamente dietro CGNAT, Starlink, connessioni mobili 4G/5G e firewall complessi con zero porte aperte sul router.
* **Piano Dati Crittografato P2P Diretto**: Cloudflare è utilizzato esclusivamente per il traffico leggero di segnalazione JSON (autenticazione e peer discovery). I flussi di dati pesanti — streaming video 4K Jellyfin, backup foto/video con Immich, sync Nextcloud e trasferimenti file Samba — viaggiano direttamente peer-to-peer (P2P) tra smartphone e server tramite un tunnel WireGuard crittografato end-to-end, completamente all'esterno di Cloudflare.
* **100% Conforme ai ToS di Cloudflare**: Poiché i video e i file non transitano mai per i server proxy di Cloudflare, la configurazione rispetta integralmente i Termini di Servizio di Cloudflare con banda gigabit illimitata.
* **Accoppiamento Semplice da Smartphone**: Si connette con le app ufficiali e gratuite di Tailscale per iOS, Android, macOS, Windows e Linux selezionando "Change server" e inserendo una chiave pre-autenticata monouso valida 1 ora, generata con 1 clic dal pannello di Allod.

---

## Livelli Pianificati (Roadmap Futura)

* **`wireguard` (WireGuard Sovrano Puro — 📋 planned)**: WireGuard nativo nel kernel Linux con associazione istantanea tramite QR code crittografico, pensato per connessioni con IP pubblico o port forwarding UDP manuale.
* **`tailscale` (Zero-Click Cloud — 📋 planned)**: Connessione diretta del client al server di controllo hosted commerciale di Tailscale (`tailscale.com`).

---

## Modello di Sicurezza e Privacy

1. **Zero Porte Aperte in Ingresso**:
   * Nessuna regola di port forwarding (es. porte 80, 443 o 22) necessaria sul router.
   * Elimina l'esposizione a port scanner su Internet, tentativi di brute force e attacchi DDoS.
2. **Archiviazione Segreti Protetti**:
   * I token del Cloudflare Tunnel sono salvati esclusivamente in `network/secrets/cloudflared.env` con permessi restrittivi `0600` e mai inseriti in chiaro nei file di configurazione dei container.
3. **Crittografia P2P WireGuard**:
   * Tutto il traffico applicativo tra i dispositivi è cifrato end-to-end tramite crittografia a curva ellittica moderna (protocollo Noise).
