# Allod — Personal Cloud & Federated Peer Backup Orchestrator

**Full data ownership: modular, subscription-free, with zero exposed ports.**

> *"I tuoi dati in piena proprietà, con la stessa comodità di prima."*  
> *Your data, held in full ownership, with the simplicity you expect.*

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](go.mod)
[![Target OS](https://img.shields.io/badge/Target%20OS-Ubuntu%20Server%2024.04%20LTS-E95420.svg)](docs/en/tutorial/first-node.md)
[![Architecture](https://img.shields.io/badge/Arch-x86--64%20%7C%20ARM64-brightgreen.svg)](docs/en/explanation/architecture.md)

---

## 📖 Overview

**Allod** is a lightweight, declarative personal cloud orchestrator built on top of **Podman Quadlets** and **systemd**. It transforms any mini PC, old laptop, or Raspberry Pi 5 into a resilient home server that can federate with friends over an encrypted WireGuard mesh to provide reciprocal, ransomware-proof backups.

### Core Principles
1. **Full Data Ownership**: You own the hardware, the encryption keys, and the storage. No third-party SaaS accounts required.
2. **Immutable Append-Only Backups**: Peer backups operate in strict append-only mode (`rest-server --append-only`). If your local server suffers a ransomware attack, the attacker cannot delete or tamper with historical backups stored on your peers' servers.
3. **Decentralized 2-Replica Ring**: In a federation group of 3+ nodes, every critical dataset automatically maintains 2 distinct remote replicas with anti-affinity placement.
4. **Strict Privilege Boundary**: The web dashboard is 100% rootless; administrative tasks are delegated over a local UNIX socket to a minimal root helper with a closed 9-action whitelist.
5. **No Open Firewall Ports**: Natively integrates with WireGuard overlay meshes (Headscale / Tailscale) with fine-grained per-port access control lists (ACLs).

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
  │                 │    (Closed 9-Action Whitelist)            │
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

## ⚡ Prerequisites & Requirements

* **Operating System**: Ubuntu Server 24.04 LTS (recommended) or any Debian 12+ system (x86-64 or ARM64 / Raspberry Pi 5).
* **Hardware**: Minimum 4 GB RAM (8 GB recommended for AI photo indexing), 1x or 2x disks for storage.
* **System Packages**:
  ```bash
  sudo apt update && sudo apt install -y podman btrfs-progs git golang-go
  ```

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

### 6. Launch the Web Dashboard

**Option A: Simple background with nohup**
```bash
nohup ./allod-panel > panel.log 2>&1 &
```

**Option B: Persistent systemd user service**
```bash
mkdir -p ~/.config/systemd/user
cp configs/allod-panel.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now allod-panel
```

Open your browser at **`http://<SERVER-IP>:8080/`** (or `http://localhost:8080/`) to access the responsive web panel.

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
| `allod ring status` | Displays ring federation health and verifies 2-replica dataset placement. |
| `allod ring add <id> <ip>` | Connects a friend's node to the encrypted federation Ring. |
| `allod ring simulate --remove <id>` | Calculates emergency rebalance plan if a peer leaves the ring. |
| `allod sbom` | Generates CycloneDX JSON Software Bill of Materials (CRA compliant). |
| `allod install <hostname>` | Generates zero-touch cloud-init deployment configuration. |

---

## 📚 Documentation

Complete documentation according to the official project structure is available in [`docs/en/`](docs/en/):

* **Tutorials**:
  * [Your First Allod Node in 15 Minutes](docs/en/tutorial/first-node.md)
* **How-to Guides**:
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

## 🛡️ Security & Compliance

* **Cyber Resilience Act (CRA)**: Formal SBOM generation via `allod sbom`.
* **Vulnerability Disclosure**: See [SECURITY.md](SECURITY.md).
* **Contributions**: Governed by the [Contributor License Agreement (CLA)](CLA.md).

---

## 📄 License

Allod is free and open-source software licensed under the **[GNU Affero General Public License v3.0](LICENSE)** (AGPL-3.0).
