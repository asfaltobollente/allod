# Modulo Network (Accesso Remoto & Sovereign Mesh)

Il modulo `network` gestisce la connettività sicura da remoto per accedere a Immich, Jellyfin, Nextcloud, Samba e alla Web Dashboard di Allod da smartphone, computer e tablet in tutto il mondo senza aprire porte in ingresso sul router di casa tramite [NetBird](https://netbird.io).

---

## Livelli Operativi Attivi

### `cloud` (NetBird Cloud Europeo — 40 MB RAM Allocata — Consigliato)
* **Server a Francoforte, Germania**: Conforme al 100% al GDPR europeo e alla sovranità dei dati.
* **Zero Manutenzione Infrastrutturale**: Si collega al control plane gestito di NetBird con una semplice Setup Key (`NB_SETUP_KEY`).
* **NAT Traversal WebRTC Automatico**: Connessione istantanea dietro CGNAT, connessioni satellitari/FWA, reti mobili 4G/5G con zero porte aperte sul router.
* **Flussi Dati WireGuard P2P Diretti**: Streaming 4K Jellyfin, backup automatico foto Immich e cartelle Samba viaggiano in P2P diretto ad altissima velocità.

### `selfhosted` (Server di Gestione Sovrano — 40 MB RAM Allocata)
* **Sovranità Totale su Control Plane e Dati**: Si collega al tuo server di gestione NetBird self-hosted dedicato (es. `https://mesh.tuodominio.it:443`).
* **Zero Dipendenze da Terze Parti**: Possesso integrale del registro peer, delle ACL e delle regole di instradamento.
* **Integrazione Identity**: Supporto per provider di autenticazione proprietari (Keycloak, Authentik via OIDC).

---

## Modello di Sicurezza e Privacy

1. **Zero Porte Aperte in Ingresso**:
   * Nessuna regola di port forwarding (es. porte 80, 443 o 22) necessaria sul router.
   * Elimina l'esposizione a port scanner su Internet, tentativi di brute force e attacchi DDoS.
2. **Archiviazione Segreti Protetti**:
   * Le credenziali di pairing NetBird sono salvate esclusivamente in `network/secrets/netbird.env` con permessi restrittivi `0600`.
3. **Crittografia P2P WireGuard**:
   * Tutto il traffico applicativo tra i dispositivi è cifrato end-to-end tramite crittografia moderna basata su WireGuard e protocollo Noise.
