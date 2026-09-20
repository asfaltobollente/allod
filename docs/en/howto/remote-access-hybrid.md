# How-to: Sovereign Remote Access via NetBird Mesh (Cloud & Self-Hosted)

Allod's **NetBird Mesh** network module provides zero-trust, end-to-end encrypted remote access to your home cloud (Immich, Jellyfin, Nextcloud, Samba, and the Web Panel) from any smartphone or laptop worldwide **without opening any incoming ports on your home router**.

---

## 🏗️ Architecture Overview

NetBird cleanly separates signaling coordination from high-speed data transfer:

1. **Signaling & Peer Discovery (Control Plane)**:
   * **`cloud` mode**: Connects to NetBird's European cloud infrastructure (Frankfurt, Germany — 100% GDPR compliant, free up to 100 peers & 5 users).
   * **`selfhosted` mode**: Connects to your private self-hosted NetBird management server (e.g. running on a VPS).
   * Uses WebRTC (ICE, STUN, TURN) to achieve automated NAT traversal behind Carrier-Grade NAT (CGNAT), Starlink, 4G/5G mobile carriers, and strict firewalls with **zero open router ports**.
2. **Encrypted High-Speed Data (Data Plane)**:
   * Your smartphone/laptop pairs using the official, open-source NetBird client app.
   * All application traffic flows **directly peer-to-peer (P2P)** between your device and your Allod node over kernel WireGuard (`wt0`).
   * Full native support for:
     * **Samba Shares (TCP 445)** directly inside iOS Files and Android file managers.
     * **Jellyfin 4K direct streaming** at full uncapped upload bandwidth.
     * **Immich photo & video camera backup** syncing automatically in the background.

---

## 📋 Prerequisites

* An Allod node running with the `network` module level set to `cloud` (default) or `selfhosted`.
* A free NetBird account at [netbird.io](https://netbird.io) (for `cloud` mode) OR a deployed NetBird management server (for `selfhosted` mode).
* A smartphone or laptop with the official NetBird app installed (available free on iOS App Store, Google Play, macOS, Windows, Linux).

---

## 🚀 Step-by-Step Configuration

### Step 1: Generate a NetBird Setup Key

1. Log in to your NetBird management dashboard (e.g. [app.netbird.io](https://app.netbird.io) or your self-hosted dashboard).
2. Navigate to **Setup Keys** and click **Add Key**.
3. Choose a key type (e.g. `Reusable` for servers or `One-off`) and set the expiration.
4. Copy the generated Setup Key (e.g. `4A8B7C21-D4E5-...`).

---

### Step 2: Configure the Network Module in Allod Panel

*(Interface: Web browser connected to Allod Panel on local network `http://<SERVER-IP>:8080`)*

1. Navigate to the **Modules** section on the Allod dashboard.
2. Locate the **🌐 NetBird Sovereign Mesh** module card.
3. Click the **⚙️ Configure NetBird** button.
4. In the configuration modal dialog:
   * **Operational Mode**: Select **NetBird Cloud (EU)** or **NetBird Self-Hosted**.
   * **Setup Key**: Paste the Setup Key copied in Step 1.
   * **Management URL** *(only if Self-Hosted)*: Enter your management server URL (e.g., `https://mesh.yourdomain.com:443`).
   * Click **Save & Restart NetBird**.
5. Allod immediately stores the credentials in a protected `0600` secret file (`network/secrets/netbird.env`), regenerates systemd Quadlets, reloads systemd, and restarts the `network` container.

---

### Step 3: Connect the NetBird Mobile App

*(Interface: Smartphone running the official NetBird app on 4G/5G)*

1. Open the **NetBird** app on your iPhone or Android device.
2. If using **NetBird Cloud**:
   * Log in with your NetBird credentials or enter a Setup Key.
3. If using **NetBird Self-Hosted**:
   * In the app settings (gear icon), enter your custom **Management Server URL** (e.g. `https://mesh.yourdomain.com:443`).
   * Log in or enter a Setup Key.
4. Toggle the connection switch to **Connected**.
5. The VPN indicator turns green. Your device is now directly connected to your Allod node via an encrypted WireGuard P2P tunnel!

---

### Step 4: Remotely Access Your Home Cloud Services

Once connected via the NetBird app, you can access all Allod services using the server's Mesh IP (`100.64.0.1`):

| Service | Remote URL / Connection | Description |
| :--- | :--- | :--- |
| **Allod Panel** | `http://100.64.0.1:8080` | Full administrative dashboard from anywhere |
| **Immich Photos** | `http://100.64.0.1:2283` | Direct automatic photo/video camera backup |
| **Jellyfin Media**| `http://100.64.0.1:8096` | 4K direct streaming without proxy bandwidth limits |
| **Nextcloud** | `http://100.64.0.1:8443` | Encrypted file sync, calendar, and notes |
| **Samba Shares** | `smb://100.64.0.1/shares` | Full SMB access in iOS/Android Files app |

---

### Step 5: Verify Connected Peers in Allod Panel

Refresh the Allod Web Panel. In the **Network** module card and **Launchpad**:
* The node displays **Connected (WireGuard P2P)**.
* Connected peers and assigned mesh IP addresses are reported live.
