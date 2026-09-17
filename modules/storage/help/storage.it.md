# Modulo Storage (Gestione Pool Btrfs)

Il modulo `storage` gestisce i dischi fisici (HDD, SSD, NVMe) sfruttando le funzionalità avanzate del filesystem Btrfs.

## Modalità Supportate e Subvolumi

* **RAID 1 (Consigliato)**: Mirroring su due dischi con checksum automatico dei dati e autoriparazione (*self-healing*).
* **Single / RAID 0**: Singolo disco o striping per installazioni senza ridondanza hardware.
* **Punto di Mount**: Montato su `/mnt/allod-storage` con opzione `compress=zstd:1` per la compressione trasparente in tempo reale.
* **Subvolumi Isolati**:
  * `/mnt/allod-storage/cloud`
  * `/mnt/allod-storage/photos`
  * `/mnt/allod-storage/shares`
  * `/mnt/allod-storage/backup`
