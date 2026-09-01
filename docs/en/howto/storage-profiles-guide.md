# 🧭 Storage Profiles Selection Guide — Allod

> How to organize your data on the RAID pool for maximum speed, simplicity, and safety—without duplicating files.

---

## 🎯 The Core Principle: Single Source of Truth

In Allod, your files exist **only once** on the Btrfs RAID 1 pool, and different services (`cloud`, `shares`, `photos`) look into it as dedicated views.

To ensure stability and prevent index desynchronization or corrupted databases, Allod adopts a golden rule:

> **"One data pool. One owner per subtree. N views with dedicated permissions."**

Depending on how you use your computer, your smartphone, and your daily workflow, Allod supports **3 configuration profiles**. Here is the guide to help you choose the best fit for your needs.

---

## ⚡ Quick 10-Second Selector

```
Do you need to share public links with external people (clients, family) without VPN access?
│
├── 🟢 YES ──→  PROFILE A (Nextcloud Monolith) or PROFILE C (Hybrid)
│
└── 🔴 NO  ──→  PROFILE B (Samba + Immich)  ⭐ [Recommended for 90% of home/LAN users]
```

---

## ☁️ Profile A — "Cloud" (Nextcloud Alone)

**One single service, one index, one unified app for everything.**

In this profile, you only activate the `cloud` module (Nextcloud Hub 30 + PostgreSQL). `shares` and `photos` remain disabled (`off`).

```
/mnt/allod-storage/
├── data/              ← Entirely managed by Nextcloud (documents, photos, notes)
└── system-cloud/      ← PostgreSQL 16 DB, appdata, previews
```

### 👤 Who it is for
* Users who want **maximum simplicity**: a single application on phone and PC.
* Users working remotely who frequently send **password-protected public download links with expiration dates** to friends, clients, or colleagues.
* Users who want synced contacts, calendars, and notes across all devices.

### 👍 Pros
* **Total management simplicity**: 1 active service, 1 database, zero permission conflicts.
* **Access from anywhere**: Native HTTPS design and full mobile apps.
* **Desktop On-Demand (Virtual Files)**: Nextcloud Desktop client for Windows/Mac displays all Terabytes in File Explorer while taking 0 Bytes locally.
* **Complete integrated suite**: Calendar (CalDAV), Contacts (CardDAV), Notes, and sharing.
* **Low resource usage**: ~600 MB RAM for the entire system.

### 👎 Cons & Limitations
* **Slow for massive files**: All file operations pass through PHP and PostgreSQL. Moving a 50 GB file or folder with tens of thousands of files is noticeably slower than raw Samba wire-speed.
* **Basic photo management**: Nextcloud Photos is a simple chronological grid without rapid AI face recognition or semantic search.

---

## 📁📸 Profile B — "Archive" (Samba + Immich) — ⭐ Recommended

**Pure LAN speed on PC + AI-powered photo gallery. Zero index conflicts.**

In this profile, you activate `shares` (Samba) for computers and `photos` (Immich) for smartphones and tablets. `cloud` remains disabled (`off`).

```
/mnt/allod-storage/
├── data/                         ← Root of Samba share (\\allod\shares)
│   ├── Documents/                ← Writer: Samba (Windows/Mac PC)
│   ├── Projects/                 ← Writer: Samba (Windows/Mac PC)
│   └── images/
│       ├── Archive/              ← Writer: Samba (Historical/DSLR photos)
│       │                            Reader: Immich (External library indexed by AI)
│       └── Phone/                ← Writer: Immich (Automatic mobile backup)
│                                    Reader: Samba (Read-only for safety)
└── system-photos/                ← pgvector DB, Valkey, thumbnails, and AI models
```

### 👤 Who it is for
* Users who access the NAS primarily from home or via VPN (WireGuard/Tailscale).
* Users working with large files (4K video editing, CAD, raw archives, ISOs, music) who want **maximum LAN transfer speeds (115–280 MB/s)**.
* Users who prioritize their photo/video memories and want an experience on par with **Google Photos / Apple Photos**.

### 👍 Pros
* **Maximum native speed**: Work on Windows/Mac as if the NAS were an internal drive, without heavy sync clients.
* **State-of-the-art AI (Immich)**: Automatic family face clustering, semantic search (*"sunset on the beach"*), interactive map, instant video scrub.
* **Zero locking conflicts**: Samba writes documents, Immich writes phone photos and reads archive. The two never collide.
* **Clean and rock-solid topology**: Minimal maintenance, zero cache drift.

### 👎 Cons & Limitations
* **No direct public links**: Cannot generate public download links for external people (except Immich photo album links).
* **Remote access requires VPN**: Opening files outside the home requires connecting to your personal secure VPN (e.g., WireGuard).
* **RAM usage**: Requires ~1.5–2 GB RAM for Immich Machine Learning models.

---

## 🔀 Profile C — "Hybrid / Custom" (Nextcloud + Samba + Immich)

**Everything unified on the same storage pool. Maximum power with explicit rules.**

In this profile, you activate all three services (`cloud` + `shares` + `photos`) pointing to `/mnt/allod-storage/data/`.

```
/mnt/allod-storage/
├── data/                         ← UNIFIED PHYSICAL ARCHIVE
│   ├── Documents/                ← Samba (R/W) + Nextcloud External Storage (R/W)
│   ├── images/
│   │   ├── Archive/              ← Samba (R/W) + Immich External (RO) + Nextcloud (R/W)
│   │   └── Phone/                ← Immich Backup (R/W) + Samba (RO)
│   └── Projects/
├── system-cloud/                 ← Nextcloud metadata and DB
└── system-photos/                ← Immich metadata, AI DB, and cache
```

### 👤 Who it is for
* Advanced users / Prosumers who want **all features simultaneously**: Samba speed on PC, Nextcloud public links and mobile files, and Immich AI photo gallery.

### 👍 Pros
* **360-degree flexibility**: Use the best tool for each context and device.
* **True Single Source of Truth**: Edit via Samba, see it synced on Nextcloud, and indexed on Immich.

### 👎 Cons & Required Safeguards
* **Index Divergence**: Changes made via Samba must be reconciled into Nextcloud and Immich databases via background reconciliation daemon (`files:scan` / inotify).
* **Strict Permission Discipline**: Requires consistent permissions (`keep-id` on containers and `allod-data` group with SGID `2770`).
* **Resource Consumption**: Requires at least **8–16 GB RAM** to concurrently run two PostgreSQL databases, Valkey cache, Immich AI microservices, and Nextcloud PHP-FPM.

---

## 📊 Summary Comparison Table

| Feature | ☁️ Profile A (Cloud) | 📁📸 Profile B (Archive) ⭐ | 🔀 Profile C (Custom) |
| :--- | :---: | :---: | :---: |
| **LAN Speed on Large Files** | ⚖️ Moderate (WebDAV) | ⚡ **Max (SMB 115-280 MB/s)** | ⚡ **Max (SMB)** |
| **Photo Gallery with AI (Faces, Maps)** | ❌ Basic | 🧠 **Excellent (Immich)** | 🧠 **Excellent (Immich)** |
| **Public Links to External Users** | 🔗 **Yes (Nextcloud)** | ❌ Photo albums only | 🔗 **Yes (Nextcloud)** |
| **Calendar, Contacts, and Notes** | 📅 **Included** | 🪶 Optional (Radicale/Baïkal) | 📅 **Included** |
| **Automatic Mobile Backup** | 📱 Nextcloud Sync | 📸 **Immich Native Backup** | 📸 **Immich Native Backup** |
| **Remote Access Outside Home** | 🌐 Direct HTTPS | 🔒 Via WireGuard VPN | 🌐 Both |
| **Managed Containers / Services** | 🪶 Few (Nextcloud+DB) | 🪶 Clean (Immich+Samba) | 📦 Complete (All) |
| **Index Desync Risk** | 🟢 Zero | 🟢 Zero | 🟡 Requires Watcher Daemon |
| **Recommended RAM** | 4–8 GB | 6–8 GB | 8–16 GB |

---

## 💡 Allod's Recommendation
* For a **home and family NAS**: Choose **Profile B (Samba + Immich)**. It is the fastest, smoothest, and most robust combination.
* For a **remote office and client file sharing**: Choose **Profile A (Nextcloud)**.
* For **all features without compromises** (with 8–16 GB RAM): Choose **Profile C (Custom)**.