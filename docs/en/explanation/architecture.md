# Architecture & Design Principles

Allod is an orchestrator designed to solve a fundamental problem: **how to build a personal, self-hosted cloud that regular people can rely on for a lifetime without relying on centralized SaaS providers**.

---

## 1. The Privilege Boundary

Traditional NAS appliances run entire web applications (PHP, Node.js, Python) as `root`. A single remote code execution vulnerability immediately compromises the entire operating system and all drives.

Allod solves this with a strict boundary:
* **The Web Panel (`allod-panel`)**: An unprivileged rootless process. It cannot delete partitions, execute arbitrary shell commands, or access raw disks.
* **The Helper Daemon (`allod-helperd`)**: A tiny root service that exposes a closed list of 9 validated actions over a local UNIX socket.
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
