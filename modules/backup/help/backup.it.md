# Modulo Backup (Restic & rest-server)

Il modulo `backup` gestisce il backup federato tra i nodi peer fidati del tuo Allod Ring utilizzando Restic e `rest-server`.

## Architettura e Porte

* **Motore Backend**: `docker.io/restic/rest-server:0.12.1`.
* **Porta**: `8000` (protocollo HTTP REST per client restic).
* **Percorso Storage**: Subvolume dedicato su `/mnt/allod-storage/backup` (o `~/.local/share/allod/backup`).

## Sicurezza e Limitazioni Note

> [!WARNING]
> **Nessuna Autenticazione nell'immagine v0.12.1**: Il container upstream `rest-server:0.12.1` opera in modalità `--no-auth` senza isolamento per singolo peer tramite `.htpasswd`. Tutti i peer con accesso alla porta 8000 possono accedere al repository.

### Misure di protezione raccomandate:
1. **Uso esclusivo su Rete Mesh Privata**: Esponi la porta 8000 solo sull'overlay NetBird sovereign mesh (`100.64.0.0/10` / `10.42.0.0/16`). Non effettuare port forwarding della porta 8000 sul router di casa.
2. **Crittografia lato Client**: Restic crittografa ogni snapshot lato client con AES-256 prima dell'invio sulla rete.
3. **Roadmap di sviluppo**: L'autenticazione per-peer con htpasswd e la pianificazione automatica degli snapshot tramite systemd timer sono in fase di sviluppo.
