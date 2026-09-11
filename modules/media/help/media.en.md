# Media Module (Jellyfin Media Server)

The `media` module integrates **Jellyfin** for streaming movies, TV shows, and music to Smart TVs, phones, tablets, and web browsers.

## Media Folders & Libraries

Media files copied over the public Samba share (`\\allod\public`) are accessible inside Jellyfin in two dedicated locations:

* **`/media`**: Dedicated public media directory (`/mnt/allod-storage/shares/public`).
* **`/shares/public`**: Direct mount of the public share.
* **`/shares`**: Root of shared storage.

### Recommended Directory Structure
```text
\\allod\public\          (Freely accessible to all devices on the LAN without credentials)
├── film\                 -> select in Jellyfin as "Movies" library
├── musica\               -> select in Jellyfin as "Music" library
├── serie\                -> select in Jellyfin as "Shows" library
├── movies\
├── tv\
└── music\
```

## Permissions
Allod automatically applies `0777` permissions to Samba and Jellyfin shared storage. If directories were created with restricted permissions, realign them with:
```bash
sudo chmod -R 0777 /mnt/allod-storage/shares/public
```
