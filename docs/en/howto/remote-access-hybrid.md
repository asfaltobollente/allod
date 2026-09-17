# How-to: Sovereign Remote Access via Hybrid Shield (Headscale + Cloudflare Tunnel)

Allod's **Hybrid Shield** remote access level provides zero-trust, end-to-end encrypted remote access to your home cloud (Immich, Jellyfin, Nextcloud, Samba, and the Web Panel) from any smartphone or laptop worldwide **without opening any incoming ports on your home router**.

---

## 🏗️ Architecture Overview

The Hybrid architecture cleanly separates signaling from high-speed data:

1. **Signaling & Coordination (Control Plane)**:
   * Runs self-hosted [Headscale](https://headscale.net) directly on your Allod node with embedded SQLite.
   * Headscale is securely exposed via an outbound Cloudflare Tunnel (`cloudflared`).
   * Bypasses Carrier-Grade NAT (CGNAT), Starlink, 4G/5G mobile carriers, and strict firewalls with **zero open router ports**.
2. **Encrypted High-Speed Data (Data Plane)**:
   * Your smartphone/laptop pairs using the official, open-source Tailscale client app.
   * Heavy traffic (4K Jellyfin streaming, Immich photo backups, Samba transfers) flows **directly peer-to-peer (P2P)** between your phone and Allod node over kernel WireGuard.
   * Video and photo data travels completely outside Cloudflare, guaranteeing **100% Cloudflare ToS compliance** and uncapped gigabit speeds.

---

## 📋 Prerequisites

* An Allod node running with the `network` module level set to `hybrid`.
* A domain name with DNS managed on Cloudflare (e.g., `vpn.example.com`).
* A free Cloudflare Zero Trust account to generate a Tunnel token.
* A smartphone or laptop with the official Tailscale app installed (available free on iOS App Store, Google Play, macOS, Windows, Linux).

---

## 🚀 Step-by-Step Configuration

### Step 1: Create a Cloudflare Tunnel
1. Log in to the [Cloudflare Zero Trust Dashboard](https://one.dash.cloudflare.com/).
2. Navigate to **Networks** → **Tunnels** and click **Add a tunnel**.
3. Select **Cloudflared** as the tunnel type and assign a name (e.g., `allod-tunnel`).
4. Under the install command section, locate the tunnel token string (the base64 string following `--token`). Copy this token.
5. In the **Public Hostname** tab, configure a route:
   * **Subdomain**: e.g., `vpn`
   * **Domain**: e.g., `example.com`
   * **Service Type**: `HTTP`
   * **URL**: `localhost:8085` (Headscale's local port)
6. Save the tunnel configuration.

---

### Step 2: Configure the Network Module in Allod Panel

*(Interface: Web browser connected to Allod Panel on local network `http://<SERVER-IP>:8080`)*

1. Navigate to the **Modules** section on the Allod dashboard.
2. Locate the **🌐 Network & Remote Access** module card.
3. Click the **⚙️ Configure Tunnel** button.
4. An interactive modal dialog opens:
   > **[Modal UI Description]**:  
   > * **Coordination Domain Input**: Enter your public domain pointing to the tunnel (e.g., `https://vpn.example.com`).
   > * **Tunnel Token Input**: Paste the Cloudflare Tunnel secret token copied in Step 1.
   > * **Action Button**: Click **Save & Restart Services**.
5. Allod immediately stores the tunnel token in a protected `0600` secret file (`network/secrets/cloudflared.env`), updates the Headscale configuration, reloads systemd Quadlets, and restarts both `network-headscale` and `network-cloudflared`.

---

### Step 3: Generate a Device Pre-Auth Key

*(Interface: Web browser connected to Allod Panel)*

1. On the Network card (or via the Launchpad header), click the **📱 Pair Device** button.
2. The Pairing modal dialog appears:
   > **[Modal UI Description]**:  
   > * **Coordination URL**: Displays your configured public endpoint (e.g., `https://vpn.example.com`).
   > * **One-Time Key**: Displays a freshly generated cryptographic pre-auth key valid for 1 hour.
   > * Copy both values to use on your smartphone.

---

### Step 4: Connect the Tailscale Mobile App

*(Interface: Smartphone running the official Tailscale app outside your home network, e.g. on 4G/5G)*

1. Open the **Tailscale** app on your iPhone or Android device.
2. If already logged into Tailscale:
   * Tap the settings icon or the three-dot menu in the upper right corner.
   * Tap **Reauthenticate** or **Log out**.
3. On the login screen, open the top-right menu and select **Change server** (or **Use custom coordination server**).
4. Enter your Allod coordination URL:  
   `https://vpn.example.com`
5. Tap **Log in**. When prompted for authentication, enter the 1-hour pre-auth key generated in Step 3.
6. The app connects immediately. The VPN status indicator turns green, showing your device connected to your private Allod mesh network with an assigned mesh IP (e.g., `100.64.0.2`).

---

### Step 5: Remotely Access Your Home Cloud Services

Once connected via the Tailscale app, your phone is part of your private sovereign network. You can access all Allod services using the server's Mesh IP (`100.64.0.1`):

| Service | Remote URL / Connection | Description |
| :--- | :--- | :--- |
| **Allod Panel** | `http://100.64.0.1:8080` | Full administrative control from anywhere |
| **Immich Photos** | `http://100.64.0.1:2283` | Direct automatic photo/video camera backup |
| **Jellyfin Media**| `http://100.64.0.1:8096` | 4K direct streaming without Cloudflare proxy |
| **Nextcloud** | `http://100.64.0.1:8082` | Encrypted file sync, calendar, and notes |
| **Samba Shares** | `smb://100.64.0.1/shares` | Secure file access via iOS/Android Files app |

---

### Step 6: Verify Connected Nodes in Allod Panel

Refresh the Allod Web Panel. In the **Network** module card and **Launchpad**:
* The registered device will appear in the **Connected Nodes** table (showing device hostname, OS, and assigned mesh IP).
* Tunnel health displays **Active (Encrypted P2P)**.
