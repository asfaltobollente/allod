# Modulo Passwords (Vaultwarden)

Vaultwarden è un'implementazione autonoma, leggera e ad alte prestazioni dell'API Bitwarden scritta in Rust. Integrata in Allod, permette a tutta la famiglia di custodire password, codici 2FA/TOTP, note di testo sicure, carte di pagamento e allegati cifrati su hardware sovrano, con pieno supporto per le estensioni browser ufficiali e le applicazioni mobili per iOS e Android.

---

## Caratteristiche Principali

* **Crittografia a Conoscenza Zero (Zero-Knowledge)**: Tutti i dati vengono cifrati e decifrati direttamente sul dispositivo dell'utente tramite algoritmi standard (AES-256 e PBKDF2/Argon2id). Sul server Allod risiedono esclusivamente blob cifrati: nessuno, nemmeno l'amministratore di sistema o l'utente root, può accedere alle credenziali memorizzate.
* **Compatibilità Completa Bitwarden**: Funziona direttamente con le app ufficiali Bitwarden (estensioni Chrome, Firefox, Safari, Edge, app Android, iOS, Windows, macOS, Linux e CLI).
* **Consumo Risorse Ultra-Leggero**: Il demone Rust compilato in Alpine Linux consuma tipicamente solo 30-50 MB di RAM, con un tetto massimo di sistema configurato a 256 MB.
* **Autenticazione a Due Fattori (2FA)**: Supporta generatori TOTP/autenticatori e chiavi hardware di sicurezza FIDO2/WebAuthn.
* **Condivisione Organizzazioni**: Possibilità di creare raccolte condivise tra membri della famiglia per account e accessi comuni (Wi-Fi, streaming, bollette).

---

## Livelli di Risorsa

* **`off`**: Servizio disattivato, container fermato, 0 MB RAM allocati.
* **`standard` (256 MB RAM allocati, 5 GB storage consigliato)**:
  * Esecuzione container rootless Podman su porta locale 8088 (`PublishPort=8088:80`).
  * Database SQLite integrato e file di vault memorizzati nel pool Btrfs `/mnt/allod-storage/passwords/data`.
  * Supporto WebSocket abilitato per sincronizzazione immediata tra browser e telefoni.

---

## Requisito HTTPS (WebCrypto API)

I moderni browser web (Chrome, Firefox, Safari) e le app Bitwarden richiedono rigorosamente una connessione sicura **HTTPS** per abilitare l'API crittografica nativa `window.crypto.subtle` (WebCrypto API).

Per accedere a Vaultwarden dall'esterno o dai dispositivi mobili:
1. **NetBird Sovereign Mesh (Consigliato)**: Connetti i dispositivi alla rete mesh privata Allod. L'indirizzo IP mesh garantisce il trasporto cifrato end-to-end WireGuard.
2. **Reverse Proxy Locale**: Se utilizzi un dominio personale con certificato Let's Encrypt (Caddy o Nginx), punta il reverse proxy alla porta interna `http://127.0.0.1:8088`.
3. **Localhost**: Sul server locale è possibile accedere all'interfaccia web direttamente su `http://localhost:8088`.

---

## Sicurezza e Backup

* **Dataset di Backup Allod**: La cartella `/mnt/allod-storage/passwords/data` è inclusa come dataset prioritario selezionabile nella cassaforte crittografica di backup Allod (modulo `backup`).
* **File di Configurazione Segreti**: Le chiavi e le impostazioni del server risiedono in `/mnt/allod-storage/passwords/secrets/vaultwarden.env` con permessi di accesso stretti `0600` (leggibile unicamente dall'utente allod).
* **Registrazioni Pubbliche**: La registrazione libera è disattivata per impostazione predefinita (`SIGNUPS_ALLOWED=false`). Le iscrizioni per familiari avvengono su invito gestito dall'amministratore (`INVITATIONS_ALLOWED=true`).
