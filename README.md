# Allod — Personal Cloud & Federated Peer Backup Orchestrator

**Full data ownership: modular, subscription-free, with zero exposed ports.**

> *"I tuoi dati in piena proprietà, con la stessa comodità di prima."*  
> *Your data, held in full ownership, with the simplicity you expect.*

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](go.mod)
[![Target OS](https://img.shields.io/badge/Target%20OS-Ubuntu%20Server%2024.04%20LTS-E95420.svg)](docs/en/tutorial/first-node.md)
[![Architecture](https://img.shields.io/badge/Arch-x86--64%20%7C%20ARM64-brightgreen.svg)](docs/en/explanation/architecture.md)

---

## 📖 Overview

**Allod** is a lightweight, declarative personal cloud orchestrator built on top of **Podman Quadlets** and **systemd**. It transforms any mini PC, old laptop, or Raspberry Pi 5 into a resilient home server that can federate with friends over an encrypted WireGuard mesh to provide reciprocal, ransomware-proof backups.

### Core Principles
1. **Full Data Ownership**: You own the hardware, the encryption keys, and the storage. No third-party SaaS accounts required.
2. **Immutable Append-Only Backups**: Peer backups operate in strict append-only mode (`rest-server --append-only`). If your local server suffers a ransomware attack, the attacker cannot delete or tamper with historical backups stored on your peers' servers.
3. **Decentralized 2-Replica Ring**: In a federation group of 3+ nodes, every critical dataset automatically maintains 2 distinct remote replicas with anti-affinity placement.
4. **Strict Privilege Boundary**: The web dashboard is 100% rootless; administrative tasks are delegated over a local UNIX socket to a minimal root helper with a closed action whitelist (see [Root Helper Socket API](docs/en/reference/helper-api.md)).
5. **No Open Firewall Ports**: Natively integrates with modern WireGuard overlay mesh networking ([NetBird](https://netbird.io)) with zero open ports, automated WebRTC NAT traversal, and granular per-port access control lists (ACLs).

---

## 🏛️ Architecture

```text
  ┌─────────────────────────────────────────────────────────────┐
  │                   UNPRIVILEGED USER SPACE                   │
  │                                                             │
  │   ┌───────────────────────────┐   ┌─────────────────────┐   │
  │   │  allod-panel (Web UI)     │   │  allod CLI          │   │
  │   │  - Embedded SPA dashboard │   │  - plan / apply     │   │
  │   │  - Preflight validator    │   │  - doctor / ring    │   │
  │   │  - Live service controls  │   │  - start / stop     │   │
  │   └─────────────┬─────────────┘   └──────────┬──────────┘   │
  │                 │                            │              │
  │                 │    UNIX Domain Socket      │              │
  │                 │    /run/allod/helper.sock  │              │
  │                 │    (Closed Action Whitelist)              │
  └─────────────────┼────────────────────────────┼──────────────┘
                    ▼                            ▼
  ┌─────────────────────────────────────────────────────────────┐
  │                    ROOT PRIVILEGE SPACE                     │
  │                                                             │
  │   ┌─────────────────────────────────────────────────────┐   │
  │   │  allod-helperd (Root Daemon)                        │   │
  │   │  - Samba share reload    - SMART health checks      │   │
  │   │  - btrfs snapshots       - safe storage format      │   │
  │   └─────────────────────────────────────────────────────┘   │
  └─────────────────────────────────────────────────────────────┘
```

---

## 📦 Open Source Technology Stack (Transparency First)

Allod orchestrates best-in-class, audited open-source technologies. No black boxes, no vendor lock-in:

| Module | Open Source Technology | Role & Purpose | Direct Web Port |
| :--- | :--- | :--- | :---: |
| **`cloud`** | **[Nextcloud Hub 30](https://nextcloud.com)** | Personal file sync, mobile backup & link sharing | `8443` |
| **`photos`** | **[Immich](https://immich.app)** + PostgreSQL + Valkey | Photo/video timeline & mobile camera auto-sync | `2283` |
| **`backup`** | **[rest-server](https://github.com/restic/rest-server)** + **[Restic](https://restic.net)** | Zero-knowledge, append-only encrypted peer backups | Mesh only |
| **`shares`** | **[Samba (SMB/CIFS)](https://www.samba.org)** | High-speed LAN shared folders for Windows, Mac & Linux | `445` |
| **`storage`** | **[Btrfs](https://btrfs.readthedocs.io)** + **smartmontools** | Hardware-safe RAID 1, instant snapshots & S.M.A.R.T. health | Native |
| **`media`** | **[Jellyfin](https://jellyfin.org)** | Personal streaming server for movies, series & music | `8096` |
| **`network`** | **[NetBird](https://netbird.io)** | Sovereign WireGuard mesh, remote access & zero open router ports (Cloud EU / Self-Hosted) | Mesh only |
| **`watch`** | **Allod Watch Sentinel** + **Telegram** | Remote blackout & recovery alerts, Telegram notifications & daily morning digest with weather | Standalone / Mesh |

---

## 🧭 Project Status

> **Status Snapshot**: September 2026 (`main` branch)

| Component / Feature | Category | State & Description |
| :--- | :--- | :--- |
| **Allod CLI Orchestrator** | **Funzionante oggi** *(Working Today)* | Full declarative lifecycle (`plan`, `apply`, `destroy`, `doctor`, `preflight`, `sbom`, `ring`, `purge`, `version`). Rootless Quadlet generation. |
| **Privileged Helper Daemon** | **Funzionante oggi** *(Working Today)* | Root socket at `/run/allod/helper.sock` with `allod` group ownership, strict 16-action whitelist, and full argument audit logging. |
| **Multi-Tenancy & Storage** | **Funzionante oggi** *(Working Today)* | Multi-user Linux provisioning with nologin shells, Samba `0770`/`0777` shares, Btrfs RAID 1/Single pool detection and auto-healing. |
| **Web Panel & Dashboard** | **Funzionante oggi** *(Working Today)* | Embedded SPA with live service controls, hardware preflight, speedtest, self-update, hardware telemetry vitals pill, and streamlined navigation. |
| **Web Panel Auth Gate** | **Funzionante oggi** *(Working Today)* | Dedicated session authentication (`/login`, `/setup`), PBKDF2-SHA256, cryptographically secure memory session tokens, and CLI emergency recovery. |
| **Family User Self-Service Portal** | **Funzionante oggi** *(Working Today)* | Dedicated family dashboard (`/portal`) with private Samba paths (`\\allod\<user>`), personal app launchpad, and autonomous password change synced via `allod-helperd`. |
| **Telemetry & Performance History** | **Funzionante oggi** *(Working Today)* | Continuous background time-series telemetry (CPU temp, CPU load, RAM, storage) in local SQLite WAL with interactive HTML5 `<canvas>` charts (1h, 24h, 7d, 30d) and zero external JS libraries. |
| **Wake-on-LAN Hub** | **Funzionante oggi** *(Working Today)* | Multi-subnet broadcast Magic Packet sender (`allod wol wake <mac>`, Web UI manager) for waking workstations and homelab nodes without SSH. |
| **External Watch Sentinel** | **Funzionante oggi** *(Working Today)* | Dedicated daemon (<15MB RAM) for cloud VPS/free-tier with instant outage & recovery Telegram alerts, and daily morning digest with weather. |
| **NetBird Sovereign Mesh** | **Funzionante oggi** *(Working Today)* | Native WireGuard overlay mesh with automated WebRTC NAT traversal, zero open router ports, EU Cloud (Frankfurt) & Self-Hosted sovereign modes. |
| **Media & Photos Modules** | **In sviluppo** *(In Development)* | Immich standard/full and Jellyfin container orchestration functional; automated mobile client integration in refinement. |
| **Cloud Module (Nextcloud)** | **In sviluppo** *(In Development)* | Nextcloud 30 with dedicated PostgreSQL 16 Alpine and dynamic secret management; automated WebDAV setup in refinement. |
| **Federated Backup Engine** | **In sviluppo** *(In Development)* | `rest-server` 0.12.1 rootless Quadlet provisioned; federated snapshot client orchestration, automated timer scheduling, and per-peer htpasswd auth in progress. |
| **Release Cryptographic Signing** | **Pianificato** *(Planned)* | Cosign / Minisign keyless artifact and SBOM signing for production release binaries. |
| **Automated Ring Snapshot & Restore** | **Pianificato** *(Planned)* | One-click snapshot scheduling across federated peers and interactive disaster recovery restore CLI. |


---

## 🚀 Quickstart & Installation Guide

Set up a sovereign Allod node on **Ubuntu Server 24.04 LTS** with this 5-step walkthrough:

### 1. System Requirements & Dependencies
Ensure your host is running **Ubuntu Server 24.04 LTS** (or compatible Debian/Ubuntu derivative) with **Go >= 1.25** and rootless Podman:
```bash
sudo apt update && sudo apt install -y git golang podman btrfs-progs smartmontools
```

### 2. Clone & Compile Binaries
Clone the official repository and compile the CLI, web panel, and privileged helper daemon:
```bash
cd ~
git clone https://github.com/asfaltobollente/allod.git
cd allod
go build -o allod ./cmd/allod
go build -o allod-panel ./cmd/allod-panel
go build -o allod-helperd ./cmd/allod-helperd
```

### 3. Setup Privileged Helper & User Group
Install the root helper daemon, configure socket permissions, and add your user to the `allod` group:
```bash
# Install binary and systemd service
sudo install -m 0755 allod-helperd /usr/local/bin/allod-helperd
sudo install -m 0644 configs/allod-helperd.service /etc/systemd/system/allod-helperd.service

# Create group and add current user
sudo groupadd -f allod
sudo usermod -aG allod $USER

# Start the helper daemon
sudo systemctl daemon-reload
sudo systemctl enable --now allod-helperd

# Reload user group membership (or re-login via SSH)
newgrp allod
```

### 4. Configure & Enable 24/7 Autostart (Linger)
Enable systemd user linger so that your rootless Podman containers run 24/7 even after SSH logout:
```bash
# Enable 24/7 background execution
loginctl enable-linger $USER

# Prepare local configuration
cp configs/config.example.yaml configs/config.yaml

# Register and start Web Dashboard as a persistent user service
mkdir -p ~/.config/systemd/user
cp configs/allod-panel.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now allod-panel
```

### 5. Access the Web Dashboard
Open your browser and navigate to:  
👉 **`http://<SERVER-IP>:8080`**

From the Web GUI, you can:
* Initialize your physical storage pool (Btrfs RAID 1) with 1 click.
* Start modules (**Photos / Immich**, **Shares / Samba**, **Cloud / Nextcloud**).
* Run speedtests, Podman sweeper, and self-updates directly from the browser.

---

## 💾 Storage Architecture & Physical Disk Management (A Safe Harbor for Everyone)

Allod is designed from the ground up as a **"Storage First"** system: personal cloud files, photos, and shared directories live on a dedicated storage pool and **never fill up your operating system drive**.

Allod automatically inspects your physical hardware and adapts to 3 deployment tiers:

```text
                     ┌──────────────────────────────────────┐
                     │ Linux Block Device Detection (lsblk) │
                     └──────────────────┬───────────────────┘
                                        │
             ┌──────────────────────────┼──────────────────────────┐
             ▼                          ▼                          ▼
   ┌───────────────────┐      ┌───────────────────┐      ┌───────────────────┐
   │    2+ Data Disks  │      │    1 Data Disk    │      │    0 Data Disks   │
   │  (Dedicated NAS)  │      │ (Simple Storage)  │      │  (OS Drive Only)  │
   └─────────┬─────────┘      └─────────┬─────────┘      └─────────┬─────────┘
             ▼                          ▼                          ▼
   ┌───────────────────┐      ┌───────────────────┐      ┌───────────────────┐
   │   FULL NAS MODE   │      │  SINGLE DISK NAS  │      │   WITNESS VAULT   │
   │  Btrfs RAID 1     │      │   Btrfs Single    │      │  Remote Backup    │
   │  Hardware Mirror  │      │  ⚠️ Warning Banner│      │  Preserves OS Disk│
   │  & Auto-Healing   │      │  Relies on Ring   │      │  (No Heavy Apps)  │
   └───────────────────┘      └───────────────────┘      └───────────────────┘
```

### 1. Dual-Disk NAS (Full RAID 1 Redundancy — Recommended)
* **Setup**: 1x OS Drive (SSD) + 2x Dedicated Data Drives (HDDs/SSDs).
* **How it works**: Uses **Btrfs RAID 1** data & metadata mirroring. If one physical hard drive experiences mechanical failure, your server continues operating with zero data loss. Btrfs continuously validates CRC32c checksums to detect and **automatically heal bit rot and silent corruption**.

### 2. Single-Disk NAS (Simple Home Server)
* **Setup**: 1x OS Drive + 1x Data Drive.
* **How it works**: Formatted as Btrfs Single. Allod displays a clear dashboard warning that local hardware mirroring is absent, reminding you that disaster recovery is provided by your encrypted peer replicas in the Ring.

### 3. Witness / Remote Backup Vault (Zero Dedicated Data Disks)
* **Setup**: A single disk containing the Ubuntu OS.
* **How it works**: Protects the OS drive from getting exhausted by disabling heavy local storage services and operating purely as a lightweight **Encrypted Backup Witness** to safeguard your friends' remote replicas.

---

## 🧩 Modules, Levels & Safe Runtime Transitions

Allod abandons primitive binary switches in favor of **hardware-aware resource levels**. This ensures you can run powerful modern applications (like Immich or Jellyfin) tailored to your hardware without crashing your server due to Out-Of-Memory (OOM) events.

### 📊 Level Comparison & System Requirements

| Module | Level | RAM Allocated | Minimum System Specs | Features & Capabilities |
| :--- | :--- | :--- | :--- | :--- |
| **`photos`** *(Immich)* | **`standard`** | **1.5 GB** | 8 GB System RAM, SSE4.2 | Full iOS/Android auto-backup, chronological timeline, shared albums, GPS map, EXIF metadata. Ultra-lightweight and battery-friendly. |
| **`photos`** *(Immich)* | **`full`** | **4.0 GB** | 16 GB System RAM, AVX2 | Everything in `standard` + **Facial Recognition AI** (clusters people automatically) and **Semantic AI Search** (e.g., search *"dog on a sunny beach"* without manual tags). |
| **`shares`** *(Samba)* | **`basic`** | **50 MB** | 4 GB System RAM | Unified network share `\\<SERVER-IP>\shares` for Windows, Mac, and Linux with rapid credential setup from the GUI. |
| **`shares`** *(Samba)* | **`custom`** | **100 MB** | 4 GB System RAM | Granular multi-share permissions and independent user/group Access Control Lists (ACLs). |
| **`media`** *(Jellyfin)* | **`basic`** | **500 MB** | 4 GB System RAM | 4K/1080p direct streaming of movies, TV shows, and music to Smart TVs, phones, and web browsers. |
| **`media`** *(Jellyfin)* | **`full`** | **1000 MB** | 8 GB System RAM, GPU | Everything in `basic` + **Hardware Transcoding** via Intel QuickSync / AMD VA-API (`/dev/dri/renderD128`). |
| **`network`** *(NetBird Cloud)* | **`cloud`** | **40 MB** | 4 GB System RAM | **Recommended**: NetBird European Cloud (Frankfurt, Germany). 100% GDPR, zero open router ports, automated NAT traversal, direct P2P WireGuard. |
| **`network`** *(NetBird Self-Hosted)* | **`selfhosted`** | **40 MB** | 4 GB System RAM | Connect to your private self-hosted NetBird management server. 100% control plane and signaling sovereignty. |

### 🛡️ What Happens When You Change a Level on an Active Service?

1. **Zero Data Loss (`safe` transitions)**:  
   Your personal photos, databases (PostgreSQL, SQLite), video collections, and configuration files reside on the persistent Btrfs RAID 1 pool (`/mnt/allod-storage`), **never inside ephemeral containers**. Changing a level never touches your stored files.
2. **Atomic Quadlet Update**:  
   When you change a level, Allod's preflight engine validates memory and hardware requirements, updates `config.yaml`, regenerates the systemd Quadlet container files with updated `MemoryMax` limits and flags, and executes a clean `systemctl --user daemon-reload`.
3. **Graceful Restart (~5 seconds)**:  
   The containers reload cleanly with the new resource allocation. When online, all your existing accounts, timeline, and libraries are exactly as you left them.
4. **Production Lock Protection**:  
   To prevent accidental level switches on live production databases, active cards are automatically marked with **`🟢 PROTECTED (In Production)`**. The level dropdown is disabled until you intentionally click **"Unlock"**.

### 📁 Clean Storage Isolation (Media vs Config)

Allod enforces strict boundaries between internal service databases and user-accessible files:
* **Internal System Data (`/config`, `db/`)**: Hidden from the network, preventing accidental deletion or corruption of SQLite/Postgres databases and transcode caches.
* **User Media Libraries (`/media`, `shares/`)**: Cleanly linked to Samba LAN shares:
  * Drop video files into `\\allod\shares\media\movies` or `tv` from Windows Explorer for instant Jellyfin streaming.
  * Browse original photos directly in `\\allod\shares\photos` with zero clutter from machine-learning thumbnail caches.

---

### 🌐 Remote Access & Mobile Mesh: Sovereign NetBird (Zero Open Ports)

Accessing your home cloud from outside the home usually presents an impossible dilemma:
1. **The Dangerous Route**: Open ports on your home router and expose your server to automated port-scanners, brute-force bots, and zero-day exploits.
2. **The Commercial Subscription Trap**: Pay monthly SaaS fees to proprietary cloud relays that track your traffic, enforce bandwidth caps, and require central corporate accounts.

Allod solves this by integrating **[NetBird](https://netbird.io)**, the open-source WireGuard overlay mesh (BSD-3 licensed). NetBird combines native kernel WireGuard performance with automated WebRTC (ICE, STUN, TURN) NAT traversal — keeping all incoming router ports closed, bypassing strict Carrier-Grade NAT (CGNAT), satellite/FWA providers, and 4G/5G mobile carriers, and connecting seamlessly from iOS, Android, macOS, Windows, and Linux.

```text
               ┌─────────────────────────────────────────────────────────┐
               │              ALLOD SERVER (At Home / Office)             │
               │                                                         │
               │   ┌─────────────────────────────────────────────────┐   │
               │   │ NetBird Client Container (Network=host)         │   │
               │   │ - Kernel WireGuard (wt0 interface)              │   │
               │   │ - Automated WebRTC ICE / STUN / TURN            │   │
               │   └────────────────────────┬────────────────────────┘   │
               │                            │                            │
               │   ┌────────────────────────▼────────────────────────┐   │
               │   │ Services: Immich, Jellyfin, Nextcloud, Samba    │   │
               │   └────────────────────────▲────────────────────────┘   │
               └────────────────────────────┼────────────────────────────┘
                                            │
               CONTROL PLANE (Signaling)    │    DATA PLANE (P2P WireGuard)
               Zero router ports needed     │    Direct encrypted pipe:
               Bypasses CGNAT & ISP firewalls    High-speed 4K streaming & photos
               100% Open-Source BSD-3       │    Native Samba (SMB TCP 445) support!
                                            │
                       ┌────────────────────┴───────────────────┐
                       ▼                                        ▼
             ┌──────────────────┐                     ┌──────────────────┐
             │ NetBird Control  │                     │ Remote Client    │
             │ Plane (EU Cloud  │                     │ (Phone / Laptop) │
             │ or Self-Hosted)  │                     │ Official NetBird │
             └─────────▲────────┘                     └────────▲─────────┘
                       │                                       │
                       └───────────────────────────────────────┘
                              Signaling & Peer Discovery
```

### The 2 Sovereign NetBird Operational Levels

#### 1. Level: `cloud` (NetBird European Cloud — 40 MB RAM — ⭐ Recommended)
* **Hosted in Frankfurt, Germany**: Fully compliant with European GDPR regulations. The control plane runs in ISO-certified German datacenters.
* **Generous Free Tier**: Free for up to 100 connected peer devices and 5 users with zero credit card required.
* **Zero Infrastructure Overhead**: No need to maintain a separate VPS or signaling server. Connects instantly with an ephemeral Setup Key (`NB_SETUP_KEY`).
* **Direct Encrypted P2P**: The signaling server only handles peer discovery and cryptographic handshakes. Once paired, all data (4K Jellyfin streaming, Immich camera sync, Samba file transfers) travels direct peer-to-peer over WireGuard.

#### 2. Level: `selfhosted` (Sovereign Management Instance — 40 MB RAM)
* **100% Data & Control Plane Sovereignty**: Connects to your own self-hosted NetBird management server running on a private VPS or bare-metal machine.
* **Zero Reliance on External Infrastructure**: You own both the signaling control plane and the encrypted data plane.
* **Enterprise Features**: Unlimited peers, custom identity providers (OIDC/SAML like Keycloak or Authentik), and fine-grained network routing rules.

---

### ⚖️ NetBird Operational Modes Comparison

| Feature / Metric | Option 1: `cloud` (⭐ Recommended) | Option 2: `selfhosted` (100% Sovereign) |
| :--- | :---: | :---: |
| **Control Plane Location** | NetBird Managed Cloud (Frankfurt, Germany) | Self-Hosted Server / VPS |
| **Data Privacy & GDPR** | **100% GDPR Compliant (Germany)** | **100% Sovereign (Your Hardware)** |
| **Router Ports to Open** | **Zero (0) Open Ports** | **Zero (0) Open Ports** (on Allod) |
| **NAT Traversal (CGNAT/4G)** | **Automated WebRTC ICE/STUN/TURN** | **Automated WebRTC ICE/STUN/TURN** |
| **Data Plane (Traffic)** | Direct P2P Encrypted WireGuard | Direct P2P Encrypted WireGuard |
| **Samba (SMB TCP 445)** | **Full Native Support** | **Full Native Support** |
| **Jellyfin 4K Direct Streaming**| **Full Native Support (Uncapped)** | **Full Native Support (Uncapped)** |
| **Mobile Application** | Official NetBird App (iOS / Android) | Official NetBird App (iOS / Android) |
| **Memory Footprint** | **~40 MB RAM** | **~40 MB RAM** |
| **Setup Time** | **1 Minute (Paste Setup Key)** | 5 Minutes (Provide Management URL + Key) |

---

### 🚀 Smart Launchpad: Seamless 1-Click LAN ↔ Mesh Switcher

To make remote access effortless in daily life, the Allod Web Dashboard features an integrated **Launchpad** with an intelligent network switcher:

* **`🏠 LAN (Casa)` Mode**: Automatically displays local network URLs (e.g., `http://192.168.1.50:2283` for Immich, `\\192.168.1.50\shares` for Samba).
* **`🌍 WireGuard Mesh` Mode**: Instantly updates all application cards, direct links, and Samba paths to use the node's encrypted Mesh IP (e.g., `http://100.64.0.1:2283`).

When you leave home, open the Allod Dashboard over WireGuard, toggle the switch to **`🌍 WireGuard Mesh`**, and tap any application icon to launch Immich, Jellyfin, or Nextcloud directly from your mobile browser without memorizing IPs or modifying bookmarks!

---

### 🛰️ Carrier-Grade NAT (CGNAT) & IPv6 Direct Peering

When using residential internet providers behind **Carrier-Grade NAT (CGNAT)** — such as satellite, cellular 4G/5G, or fixed-wireless access (FWA):
* **Why UPnP Fails**: Routers report `No valid IGD` because the WAN port receives a private `100.64.0.0/10` address rather than a public IPv4. Manual IPv4 port forwarding is similarly blocked upstream by the ISP.
* **Relayed Fallback**: When both ends are behind symmetric CGNAT, NetBird routes traffic through European public relays, capping throughput to ~10 Mbps with ~90 ms latency.
* **The IPv6 Solution**: Modern fiber, satellite, and broadband providers allocate dynamic public IPv6 prefix delegations (e.g. `/56` or `/64` via DHCPv6-PD). By enabling **DHCPv6 Prefix Delegation** on your router/firewall and SLAAC on your LAN, Allod acquires a global IPv6. NetBird automatically establishes an **uncapped Direct WireGuard P2P connection** over IPv6 with zero open ports! (See [How-to Guide](docs/en/howto/remote-access-hybrid.md)).

---

## ⚡ Prerequisites & Requirements


* **Operating System**: Ubuntu Server 24.04 LTS (recommended) or any Debian 12+ system (x86-64 or ARM64 / Raspberry Pi 5).
* **Hardware**: Minimum 4 GB RAM (8 GB recommended for AI photo indexing), 1x or 2x disks for storage.
* **System Packages**:
  ```bash
  sudo apt update && sudo apt install -y podman btrfs-progs git
  ```
* **Go Compiler (≥ 1.25)**:
  Allod requires Go 1.25 or newer (for modern embedded SQLite database engines). Install it from [go.dev/dl](https://go.dev/dl/) or enable automatic toolchain fetching via `export GOTOOLCHAIN=auto`.

---

## 🚀 Quick Start (Build & Run from Source)

### 1. Clone the Repository
```bash
git clone https://github.com/asfaltobollente/allod.git
cd allod
```

### 2. Build the Binaries
```bash
go build -o allod ./cmd/allod
go build -o allod-helperd ./cmd/allod-helperd
go build -o allod-panel ./cmd/allod-panel
```

### 3. Initialize Your Node Configuration
```bash
# Initialize config.yaml with your custom node name (e.g. ./allod init my-server)
./allod init my-node-name
```

### 4. Run Preflight Diagnostics & Inspect Plan
```bash
# Verify system RAM and hardware limits
./allod doctor -c config.yaml

# Dry-run configuration plan
./allod plan -c config.yaml
```

### 5. Apply Quadlets (Idempotent)
```bash
# Generate Podman Quadlet units into ~/.config/containers/systemd/
./allod apply -c config.yaml --systemd
```

### 6. Launch Daemons (Privilege Boundary)

Allod operates with a **two-daemon security model**:

1. **Root Helper Daemon (`allod-helperd`)**: Runs with root privileges (via `sudo` or systemd) to handle low-level disk management, btrfs snapshots, and SMART health checks over a secured local UNIX socket (`/run/allod/helper.sock`, mode `0666`, owned by `root:allod`, with kernel-level `SO_PEERCRED` verification).
2. **Web Dashboard (`allod-panel`)**: Runs as an unprivileged user (rootless) to manage containers and serve the web UI. To communicate with the root helper, the user running `allod-panel` must belong to the `allod` system group (analogous to the `docker` or `libvirt` group model).

**Quick Launch (Background):**
```bash
# 1. Start root helper daemon (requires sudo)
sudo nohup ./allod-helperd > helper.log 2>&1 &

# 2. Start unprivileged dashboard (as normal user)
nohup ./allod-panel > panel.log 2>&1 &
```

**Production Launch (systemd Services):**
```bash
# 1. Install & start root helper system daemon
sudo install -m 0755 allod-helperd /usr/local/bin/allod-helperd
sudo groupadd -f allod && sudo usermod -aG allod $USER
sudo cp configs/allod-helperd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now allod-helperd

# 2. Reload session to activate group membership (due to linger)
loginctl terminate-user $USER # or: sudo reboot

# 3. Install & start user dashboard daemon (after logging back in)
mkdir -p ~/.config/systemd/user
cp configs/allod-panel.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now allod-panel
```

Open your browser at **`http://<SERVER-IP>:8080/`** (or `http://localhost:8080/`) to access the responsive web panel.

---

## 🔄 Service Lifecycle & Troubleshooting

In case of issues or configuration updates, you can inspect logs and restart services independently:

| Daemon | Privilege | Role | Restart Command | Check Status & Logs |
| :--- | :--- | :--- | :--- | :--- |
| **`allod-helperd`** | Root (`sudo`) | Disks, btrfs snapshots, SMART checks | `sudo systemctl restart allod-helperd` | `sudo systemctl status allod-helperd` |
| **`allod-panel`** | User (`rootless`) | Web dashboard, preflight engine | `systemctl --user restart allod-panel` | `systemctl --user status allod-panel` |
| **Containers** | User (`podman`) | Immich, Nextcloud, rest-server | `allod start` / `allod stop` | `allod status` / `podman ps` |

---

## 🛠️ CLI Command Reference

| Command | Description |
| :--- | :--- |
| `allod init [node-name]` | Initializes a fresh `config.yaml` with custom or auto-detected node name. |
| `allod plan` | Differential dry-run comparing `config.yaml` with `state.db`. |
| `allod apply` | Generates Quadlet units idempotently, updates `state.db`, and reloads systemd. |
| `allod start [mod\|all]` | Starts container units and host services via systemd/Podman. |
| `allod stop [mod\|all]` | Stops active module containers and host services. |
| `allod status` | Displays real-time running/stopped/failed status of all configured modules. |
| `allod set <mod>=<lvl>` | Changes module level with strict hardware & dependency preflight. |
| `allod doctor` | Comprehensive diagnostic check of active modules and RAM limits. |
| `allod storage disks` | Inspects physical disks and shows current NAS storage topology. |
| `allod storage init [disks...]` | Automated Btrfs RAID 1 formatting, subvolume setup & /mnt/allod-storage mount. |
| `allod ring status` | Displays ring federation health and verifies 2-replica dataset placement. |
| `allod ring add <id> <ip>` | Connects a friend's node to the encrypted federation Ring. |
| `allod ring simulate --remove <id>` | Calculates emergency rebalance plan if a peer leaves the ring. |
| `allod wol list` | Lists saved Wake-on-LAN devices, MAC addresses, and last wake times. |
| `allod wol wake <mac\|name>` | Broadcasts an AMD Magic Packet over all local interfaces to wake a PC. |
| `allod wol add <name> <mac>` | Registers a new device into the persistent Wake-on-LAN database. |
| `allod wol delete <id>` | Removes a device from the persistent Wake-on-LAN database. |
| `allod sbom` | Generates CycloneDX JSON Software Bill of Materials (CRA compliant). |
| `allod admin-password reset [password]` | Emergency reset or initialization of Web Panel administrator password. |
| `allod install <hostname>` | Generates zero-touch cloud-init deployment configuration. |

---

## 📚 Documentation

Complete documentation according to the official project structure is available in [`docs/en/`](docs/en/):

* **Tutorials**:
  * [Your First Allod Node in 15 Minutes](docs/en/tutorial/first-node.md)
* **How-to Guides**:
  * [Telemetry & Historical Metrics](docs/en/howto/telemetry-history-metrics.md) / [🇮🇹 Telemetria & Storico Prestazioni](docs/it/howto/storico-prestazioni-e-metriche.md)
  * [Native Wake-on-LAN (WoL) Hub](docs/en/howto/wake-on-lan.md) / [🇮🇹 Guida Wake-on-LAN](docs/it/howto/wake-on-lan.md)
  * [Sovereign Remote Access via NetBird Mesh](docs/en/howto/remote-access-hybrid.md) / [🇮🇹 Accesso Remoto Sovrano NetBird](docs/it/howto/accesso-remoto-netbird.md)
  * [External Watch Sentinel with Telegram & Weather](docs/en/howto/watch-external-sentinel.md) / [🇮🇹 Watch Sentinel Esterna & Telegram](docs/it/howto/watch-sentinella-esterna.md)
  * [Storage Profiles Guide (Nextcloud vs Samba + Immich vs Hybrid)](docs/en/howto/storage-profiles-guide.md) / [🇮🇹 Guida Profili Storage](docs/it/howto/guida-profili-storage.md)
  * [Physical Disk Management & Btrfs RAID 1](docs/en/howto/disk-management.md) / [🇮🇹 Gestione Dischi](docs/it/howto/gestione-dischi.md)
  * [Invite a Friend to your Ring](docs/en/howto/invite-peer.md)
  * [Disaster Recovery & Ransomware Protection](docs/en/howto/disaster-recovery.md)
  * [Replace a Failed Disk in btrfs RAID 1](docs/en/howto/replace-disk.md)

* **Reference**:
  * [Module Manifest Specification](docs/en/reference/manifest-spec.md)
  * [CLI Command Reference](docs/en/reference/cli.md)
  * [Root Helper Socket API](docs/en/reference/helper-api.md)
* **Architecture & Community**:
  * [Architecture & Privilege Boundary](docs/en/explanation/architecture.md)
  * [Guide for Ring Participants & Friends](docs/en/for-participants.md)

---

## ⚠️ Known Limitations

* **Backup Module (Receiver-Only PoC)**: The federated backup module is under active development. Currently, only the passive backup receiver daemon (`rest-server`) is deployed, running with `--append-only --no-auth`. While `--append-only` prevents existing historical snapshots from being altered or deleted by a compromised client, any node on the mesh network can append data. The active client engine (automated `restic`), backup scheduler, per-peer `htpasswd` authentication, and restore orchestration are not yet implemented and are planned for future releases.

---

## 🛡️ Security & Compliance

* **Cyber Resilience Act (CRA)**: Formal SBOM generation via `allod sbom`.
* **Vulnerability Disclosure**: See [SECURITY.md](SECURITY.md).
* **Contributions**: Governed by the [Contributor License Agreement (CLA)](CLA.md).

---

## 📄 License

Allod is free and open-source software licensed under the **[GNU Affero General Public License v3.0](LICENSE)** (AGPL-3.0).
