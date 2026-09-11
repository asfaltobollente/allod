# Modulo Media (Jellyfin Media Server)

Il modulo `media` integra **Jellyfin** per lo streaming di film, serie TV, video personali e musica verso Smart TV, smartphone, tablet e browser.

## Cartelle e Librerie Multimediali

I file multimediali caricati tramite la condivisione Samba pubblica (`\\allod\public`) sono disponibili direttamente dentro Jellyfin in due percorsi dedicati:

* **`/media`**: Cartella pubblica multimediale predefinita (`/mnt/allod-storage/shares/public`).
* **`/shares/public`**: Percorso diretto alla condivisione pubblica.
* **`/shares`**: Visualizzazione dell'intero storage condiviso.

### Struttura Consigliata delle Cartelle
```text
\\allod\public\          (Accessibile liberamente da tutti in rete locale senza password)
├── film\                 -> da selezionare in Jellyfin come libreria "Film"
├── musica\               -> da selezionare in Jellyfin come libreria "Musica"
├── serie\                -> da selezionare in Jellyfin come libreria "Serie TV"
├── movies\
├── tv\
└── music\
```

## Permessi delle Cartelle
Allod assicura automaticamente permessi completi di lettura ed esecuzione (`0777` per le cartelle, `0666` per i file) tra Samba e Jellyfin. Se hai creato cartelle manualmente o da utente non root, puoi riallineare i permessi con:
```bash
sudo chmod -R 0777 /mnt/allod-storage/shares/public
```
