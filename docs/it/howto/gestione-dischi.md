# Guida: Gestione Dischi e Configurazione Btrfs RAID 1 in Allod

Allod è progettato come un **Porto Sicuro (Safe Harbor)**: permette a chiunque, anche senza alcuna esperienza pregressa di sistemi Linux o storage RAID, di avere una sicurezza dei dati di livello enterprise con zero comandi complicati.

---

## 🎯 1. Configurazione Hardware Consigliata

Per un server o NAS domestico robusto, la configurazione raccomandata è:
* **Disco 1 (SSD / NVMe)**: ~120–250 GB per il sistema operativo Ubuntu Server (root `/` e `/home`).
* **Disco 2 (HDD / SSD)**: 1 TB–16 TB per il Pool Dati NAS (`/mnt/allod-storage`).
* **Disco 3 (HDD / SSD)**: Stessa dimensione del Disco 2 per il **Mirroring RAID 1**.

---

## 🔍 2. Passo 1: Ispezione dei Dischi Fisici

Esegui il rilevamento automatico di Allod:

```bash
allod storage disks
```

Esempio di output:
```text
=== Dischi Fisici & Pool Storage Allod ===
Topologia Attuale: Btrfs RAID 1 Mirroring (2 Dischi Dati Dedicati - Ridondanza Completa)

  • /dev/sdc      [111 GB, SSD] (Disco di Sistema OS - root '/') Modello: Crucial BX500
  • /dev/sda      [931 GB, HDD] (Candidato Pool NAS #1) Modello: WDC WD1003FBYX
  • /dev/sdb      [931 GB, HDD] (Candidato Pool NAS #2) Modello: WDC WD1002FBYS
```

Allod isola e protegge in automatico il disco del tuo sistema operativo (`sdc`) e identifica i dischi candidati per il NAS (`sda` e `sdb`).

---

## ⚡ 3. Passo 2: Inizializzazione Btrfs RAID 1 in un Solo Comando

Per formattare e unire entrambi i dischi nel pool Btrfs RAID 1:

```bash
sudo allod storage init sda1 sdb1
```

### Cosa fa Allod dietro le quinte:
1. **Smonta in automatico** eventuali vecchi punti di montaggio temporanei.
2. **Crea il pool Btrfs RAID 1**:
   `mkfs.btrfs -d raid1 -m raid1 -f /dev/sda1 /dev/sdb1`
   * `-d raid1`: Ogni blocco di dati viene duplicato su entrambi i dischi fisici.
   * `-m raid1`: Tutti i metadati dell'albero filesystem sono duplicati.
   * **Checksum CRC32c & Auto-Healing**: Ad ogni lettura, il kernel valida i checksum e, se un blocco si corrompe, lo ripara all'istante dalla copia speculare sana.
3. **Monta il pool storage** su `/mnt/allod-storage`.
4. **Crea le cartelle dedicate ai servizi**:
   * `/mnt/allod-storage/cloud` (Nextcloud e PostgreSQL)
   * `/mnt/allod-storage/photos` (Immich e database foto)
   * `/mnt/allod-storage/shares` (Cartelle condivise Samba)
   * `/mnt/allod-storage/backup` (Cassaforte backup cifrata per gli amici)
5. **Applica i permessi non privilegiati** per il tuo utente rootless.

---

## 🔒 4. Passo 3: Montaggio Automatico al Riavvio (FSTAB)

Per fare in modo che Ubuntu rimonti il pool RAID 1 ad ogni riavvio:

```bash
UUID=$(sudo blkid -s UUID -o value /dev/sda1)
echo "UUID=$UUID /mnt/allod-storage btrfs defaults,noatime,compress=zstd:1 0 0" | sudo tee -a /etc/fstab
```

Verifica che non ci siano errori:
```bash
sudo systemctl daemon-reload
sudo mount -a
```

---

## 📊 5. Come Verificare lo Stato del RAID 1

Puoi controllare in tempo reale l'allocazione e lo stato del mirroring:

```bash
sudo btrfs filesystem usage /mnt/allod-storage
```

Voci da verificare:
* **Data, RAID1**: Conferma che i file sono duplicati su entrambi i dischi.
* **Metadata, RAID1**: Conferma che i metadati del filesystem sono in mirroring.

Per visualizzare i due hard disk fisici membri del pool:
```bash
sudo btrfs device usage /mnt/allod-storage
```

---

## 🛡️ 6. Cosa Succede se un Hard Disk si Rompe Fisicamente?

Se uno dei due dischi subisce un guasto meccanico:
1. Il tuo server continua a funzionare normalmente senza alcuna perdita di dati.
2. Sostituisci il disco rotto con uno nuovo.
3. Esegui il comando di sostituzione a caldo:
   ```bash
   sudo btrfs replace start <id-disco-guasto> /dev/sdX /mnt/allod-storage
   ```
4. Btrfs ricostruisce il mirror istantaneamente mentre tutti i servizi (Nextcloud, foto, backup) restano attivi!
