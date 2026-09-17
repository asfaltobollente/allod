# Modulo Condivisioni (Samba Windows Network Shares)

Il modulo `shares` abilita la condivisione file ad alta velocità tramite protocollo SMB/CIFS sulla tua rete locale (LAN).

## Rilevamento di Rete e Cartelle

* **Rilevamento Automatico**: Demone `wsdd2` integrato per la visualizzazione immediata in "Rete" di Windows ed esplora risorse macOS/Linux.
* **Cartella Pubblica**: `\\<IP-NODO>\public` (mappata su `/mnt/allod-storage/shares/public`, accessibile da tutti i dispositivi di casa).
* **Cartelle Personali**: `\\<IP-NODO>\<utente>` (mappata su `/mnt/allod-storage/shares/<utente>`, protetta da password personale, permessi `0770`).
