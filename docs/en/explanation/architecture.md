# Architecture & Design Principles

Allod is an orchestrator designed to solve a fundamental problem: **how to build a personal, self-hosted cloud that regular people can rely on for a lifetime without relying on centralized SaaS providers**.

---

## 1. The Privilege Boundary

Traditional NAS appliances run entire web applications (PHP, Node.js, Python) as `root`. A single remote code execution vulnerability immediately compromises the entire operating system and all drives.

Allod solves this with a strict boundary:
* **The Web Panel (`allod-panel`)**: An unprivileged rootless process. It cannot delete partitions, execute arbitrary shell commands, or access raw disks.
* **The Helper Daemon (`allod-helperd`)**: A tiny root service that exposes validated actions over a local UNIX domain socket (`/run/allod/helper.sock`, mode `0666`, owned by `root:allod`) with kernel-level caller verification via `SO_PEERCRED`.
* **The Container Units**: Generated as rootless systemd Quadlets managed by Podman.

---

## 2. Immutable Append-Only Peer Backup

Mirroring backups (such as rsync or Syncthing) propagates deletions and ransomware. If your primary node is compromised, a mirrored backup is encrypted within seconds.

Allod uses **Restic** over **rest-server** configured strictly in **append-only mode**:
$$\text{HTTP DELETE} \longrightarrow \text{HTTP 403 Forbidden}$$

A compromised node can only push *new* encrypted data chunks, but is mathematically and logically prevented from deleting or overwriting historical snapshots stored on the peer.

---

## 3. The Ring: 2-Replica Decentralized Placement

In a group of 3 or more nodes connected via WireGuard overlay mesh:
1. Every critical dataset is replicated on **at least 2 distinct remote nodes**.
2. **Anti-Affinity Rule**: Replicas are never placed on the dataset's owner node.
3. If any node goes offline, the remaining nodes detect the condition via reciprocal watchdog monitoring and calculate an automated rebalancing plan.

---

## 4. Multi-Tier Authentication & Family Self-Service Portal

Rather than deploying heavyweight enterprise Single Sign-On (SSO) frameworks (such as Keycloak or Authentik) that consume 1–2 GB of RAM and introduce fragile external dependencies, Allod implements a lightweight, zero-bloat two-tier architecture tailored for households:

```text
  ┌────────────────────────────────────────────────────────────────────────┐
  │                         ALLOD-PANEL HTTP GATE                          │
  └───────────────────────────────────┬────────────────────────────────────┘
                                      │
              ┌───────────────────────┴───────────────────────┐
              ▼                                               ▼
  ┌───────────────────────────────┐               ┌───────────────────────────────┐
  │    ADMIN GATE (/login)        │               │   FAMILY PORTAL (/portal)     │
  │  - Requires Admin Password    │               │  - Household Member Login     │
  │  - PBKDF2-SHA256 (state.db)   │               │  - Private SMB Network Paths  │
  │  - Protects Dashboard (/)     │               │  - Launchpad (Immich/Jellyfin)│
  │  - Protects System APIs       │               │  - Self-Service Password Reset│
  │  - CLI Recovery:              │               │    (Syncs Linux + Samba via   │
  │    'allod admin-password'     │               │     allod-helperd socket)     │
  └───────────────────────────────┘               └───────────────────────────────┘
```

1. **Administrator Protection Gate**:
   * All administrative views (`/`) and mutating/monitoring endpoints (`/api/*`) are guarded by `adminProtectedHandler`.
   * On first boot, if no admin password exists, users are guided through an onboarding setup wizard at `/setup`.
   * Password hashes are calculated using PBKDF2 with HMAC-SHA256 (600,000 iterations) with a 16-byte cryptographically secure random salt, stored in the embedded SQLite `state.db`.
   * Authenticated sessions are issued crypto-random 32-byte session tokens managed entirely in memory and transmitted via secure `HttpOnly` session cookies or `X-Allod-Session` headers.
   * Emergency console recovery is provided by `allod admin-password reset [password]`.

2. **Family Member Self-Service Portal (`/portal`)**:
   * A clean, friendly interface designed specifically for non-administrative family members.
   * Provides direct access links to personal applications (Immich Photos, Jellyfin Streaming, Nextcloud).
   * Displays clear, copy-pasteable network paths for Windows (`\\allod\<user>`), macOS, and mobile devices to access their private Samba storage.
   * Enables members to autonomously change their credentials. When a password is changed, `allod-panel` communicates with `allod-helperd` via `/run/allod/helper.sock` to synchronously update both the unprivileged Linux account and the Samba credentials database (`smbpasswd`).

---

## 5. Sovereign Overlay Mesh & NAT Traversal Architecture

Remote access to Allod does not require exposing incoming ports or configuring port forwarding on home routers:

1. **Signaling vs Direct P2P Data Flow**:
   * Signaling and peer discovery are handled via **NetBird** (European cloud or private self-hosted).
   * All application traffic (Samba SMB TCP 445, Jellyfin 4K streaming, Immich camera sync) flows directly peer-to-peer over kernel WireGuard (`wt0`).
2. **CGNAT & UPnP Reality**:
   * On residential connections behind **Carrier-Grade NAT (CGNAT)** — such as satellite providers, 4G/5G mobile networks, or fixed-wireless access (FWA) — the router's WAN port receives a private address (`100.64.0.0/10`).
   * On such connections, UPnP correctly reports `No valid IGD found` because there is no public IPv4 gateway on the router. Manual IPv4 port forwarding is similarly impossible.
   * If both endpoints are behind symmetric CGNAT, direct IPv4 hole punching is blocked, causing NetBird to route traffic through European cloud relays (capping bandwidth to ~10 Mbps with ~90 ms latency).
3. **IPv6 as the Zero-Cost Sovereign Highway**:
   * Modern fiber, satellite, and broadband providers natively allocate dynamic public IPv6 prefix delegations (such as `/56` or `/64` via DHCPv6-PD).
   * By configuring DHCPv6 Prefix Delegation on the home router/firewall and enabling SLAAC on the LAN, the Allod node receives a global public IPv6 address.
   * NetBird discovers IPv6 ICE candidates and negotiates a **Direct WireGuard P2P connection**, delivering uncapped ISP upload speeds and minimal latency without requiring public IPv4 or port forwarding.


