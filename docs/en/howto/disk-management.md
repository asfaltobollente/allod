# How to Manage Disks and Configure Btrfs RAID 1 in Allod

Allod is designed as a **Safe Harbor**: it gives any user, even those without prior Linux or storage administration experience, complete enterprise-grade data security with zero complicated commands.

---

## 🎯 1. Recommended Hardware Setup

For a resilient home or office NAS, the recommended layout is:
* **Disk 1 (SSD / NVMe)**: ~120–250 GB for Ubuntu Server OS (mount `/` and `/home`).
* **Disk 2 (HDD / SSD)**: 1 TB–16 TB for NAS Storage Pool (`/mnt/allod-storage`).
* **Disk 3 (HDD / SSD)**: Identical size to Disk 2 for **RAID 1 Mirroring**.

---

## 🔍 2. Step 1: Inspect Physical Disks

Run the automated inspection tool:

```bash
allod storage disks
```

Example output:
```text
=== Dischi Fisici & Pool Storage Allod ===
Topologia Attuale: Btrfs RAID 1 Mirroring (2 Dischi Dati Dedicati - Ridondanza Completa)

  • /dev/sdc      [111 GB, SSD] (Disco di Sistema OS - root '/') Modello: Crucial BX500
  • /dev/sda      [931 GB, HDD] (Candidato Pool NAS #1) Modello: WDC WD1003FBYX
  • /dev/sdb      [931 GB, HDD] (Candidato Pool NAS #2) Modello: WDC WD1002FBYS
```

Allod automatically shields your OS drive (`sdc`) and identifies data disk candidates (`sda` and `sdb`).

---

## ⚡ 3. Step 2: Initialize Btrfs RAID 1 (One-Command Setup)

To format and unite both disks into a resilient Btrfs RAID 1 pool:

```bash
sudo allod storage init sda1 sdb1
```

### What Allod does under the hood:
1. **Auto-Unmounts** any old temporary mountpoints.
2. **Creates Btrfs RAID 1**:
   `mkfs.btrfs -d raid1 -m raid1 -f /dev/sda1 /dev/sdb1`
   * `-d raid1`: Every data block is duplicated across both physical disks.
   * `-m raid1`: Every piece of filesystem metadata is duplicated across both disks.
   * **CRC32c Checksums**: Every read validates checksums and automatically heals bit rot using the healthy mirror.
3. **Mounts the storage pool** at `/mnt/allod-storage`.
4. **Creates dedicated subvolumes** for services:
   * `/mnt/allod-storage/cloud` (Nextcloud & PostgreSQL)
   * `/mnt/allod-storage/photos` (Immich & database)
   * `/mnt/allod-storage/shares` (Samba network folders)
   * `/mnt/allod-storage/backup` (Encrypted peer backups)
5. **Sets rootless permissions** for your non-root user.

---

## 🔒 4. Step 3: Make the Mount Persistent on Boot

To ensure Ubuntu automatically mounts your RAID 1 pool on every reboot:

```bash
UUID=$(sudo blkid -s UUID -o value /dev/sda1)
echo "UUID=$UUID /mnt/allod-storage btrfs defaults,noatime,compress=zstd:1 0 0" | sudo tee -a /etc/fstab
```

Verify the `/etc/fstab` configuration without errors:
```bash
sudo systemctl daemon-reload
sudo mount -a
```

---

## 📊 5. How to Verify RAID 1 Health and Allocation

You can inspect the exact real-time RAID status at any time:

```bash
sudo btrfs filesystem usage /mnt/allod-storage
```

Look for:
* **Data, RAID1**: Indicates both disks are actively mirroring user data.
* **Metadata, RAID1**: Indicates filesystem trees are fully mirrored.

You can also view devices in the pool:
```bash
sudo btrfs device usage /mnt/allod-storage
```

---

## 🛡️ 6. What Happens if a Hard Drive Fails? (Disaster Recovery)

If Disk 1 or Disk 2 experiences hardware death:
1. Your server stays online without data loss.
2. Replace the broken physical disk with a new drive.
3. Run the live replace command:
   ```bash
   sudo btrfs replace start <devid-guasto> /dev/sdX /mnt/allod-storage
   ```
4. Btrfs reconstructs the mirror on the fly while all services remain active!
