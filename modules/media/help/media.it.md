# Modulo Media (Jellyfin Media Server)

Il modulo `media` integra **Jellyfin** per lo streaming di film, serie TV, video personali e musica verso Smart TV, smartphone, tablet e browser.

## Cartelle e Librerie Multimediali

I file multimediali caricati tramite la condivisione Samba (`\\allod\shares`) sono disponibili direttamente dentro Jellyfin in due percorsi:

* **`/shares`**: Visualizza l'intera condivisione di rete (film, musica, serie TV o qualsiasi cartella personalizzata).
* **`/media`**: Percorso per la cartella multimediale dedicata (`/shares/media`).

### Struttura Consigliata delle Cartelle
```text
\\allod\shares\
├── film\         -> da selezionare in Jellyfin come libreria "Film"
├── musica\       -> da selezionare in Jellyfin come libreria "Musica"
├── serie\        -> da selezionare in Jellyfin come libreria "Serie TV"
└── media\
    ├── movies\
    ├── tv\
    └── music\
```

## Permessi delle Cartelle
Allod assicura automaticamente permessi completi di lettura ed esecuzione (`0777`) tra Samba e Jellyfin. Se hai creato cartelle manualmente o da utente non root, puoi riallineare i permessi con:
```bash
sudo chmod -R 0777 /mnt/allod-storage/shares
```
