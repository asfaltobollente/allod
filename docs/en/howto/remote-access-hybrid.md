# How-to: Sovereign Remote Access via NetBird Mesh (Cloud & Self-Hosted)

Allod's **NetBird Mesh** network module provides zero-trust, end-to-end encrypted remote access to your home cloud (Immich, Jellyfin, Nextcloud, Samba, and the Web Panel) from any smartphone or laptop worldwide **without opening any incoming ports on your home router**.

---

## 🏗️ Architecture Overview

NetBird cleanly separates signaling coordination from high-speed data transfer:

1. **Signaling & Peer Discovery (Control Plane)**:
   * **`cloud` mode**: Connects to NetBird's European cloud infrastructure (Frankfurt, Germany — 100% GDPR compliant, free up to 100 peers & 5 users).
   * **`selfhosted` mode**: Connects to your private self-hosted NetBird management server (e.g. running on a VPS).
   * Uses WebRTC (ICE, STUN, TURN) to achieve automated NAT traversal behind Carrier-Grade NAT (CGNAT), satellite/cellular providers, 4G/5G mobile carriers, and strict firewalls with **zero open router ports**.
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
* The node displays **Connected (WireGuard P2P)** or **Connected (Relayed)**.
* Connected peers and assigned mesh IP addresses are reported live.

---

## 🔍 Troubleshooting & Network Performance (CGNAT, Carrier NAT & UPnP)

### Why is Speedtest Low (~9-10 Mbps) with High Latency (~90 ms)?

When testing remote speeds from a mobile device (4G/5G) to your home Allod node over NetBird, you may notice speeds capped around 9–10 Mbps with ~90 ms ping, even if your local home network delivers 1 Gbps.

To understand why, run on your server:
```bash
netbird status --detail
```

Look at the **Peers detail** section:
* `Connection type: Direct (P2P)`: True peer-to-peer WireGuard connection. Traffic flows directly between your phone and your server at your ISP's full upload/download speed.
* `Connection type: Relayed`: Traffic is routed through NetBird's public relay servers (e.g. Frankfurt, Germany). 

> [!NOTE]
> NetBird public cloud relays enforce a natural rate limit of **~10 Mbps per stream** to prevent traffic abuse and ensure fair sharing for the community. The extra round-trip to Frankfurt also adds ~40–80 ms of latency.

---

### Why UPnP Fails Behind CGNAT and Providers Without Public IPv4

If you try to test or use UPnP on your server with a tool like `upnpc`:
```bash
upnpc -l
# Output:
# Found a (not connected?) IGD : http://192.168.0.1:38179/ctl/IPConn
# No valid UPNP Internet Gateway Device found.
```

This error is expected when your home router or firewall is connected to an ISP using **Carrier-Grade NAT (CGNAT)** — common with satellite connections, 4G/5G home routers, fixed-wireless access (FWA), and modern fiber providers:
1. **No Public IPv4 on the WAN**: The ISP does not allocate a public IPv4 to residential subscribers. Instead, the router's WAN port receives a private CGNAT address in the `100.64.0.0/10` block.
2. **UPnP Rejection**: When UPnP queries the router, the router reports that its WAN interface is not connected to a public IP gateway (`No valid IGD found`).
3. **No Inbound IPv4 Routing**: Even if the router opened a local port via UPnP or manual port forwarding, the upstream provider's carrier gateways drop all unrequested inbound IPv4 traffic from the internet.
4. **Symmetric NAT Traversal Failure**: When both the mobile carrier (4G/5G) and home ISP are behind symmetric CGNAT, standard IPv4 UDP hole punching fails, forcing NetBird into **Relayed** mode.

---

### 🚀 The Sovereign Solution: Native IPv6 Direct P2P (Zero Cost, No Open Ports)

While many residential providers do not supply a public IPv4 address, **almost all modern fiber, cable, and satellite providers allocate a public IPv6 prefix delegation (often a `/56` or `/64` prefix via DHCPv6-PD)**! 

NetBird fully supports dual-stack IPv4/IPv6 ICE candidate negotiation. Once IPv6 is active:
1. Every device (your Allod server and your smartphone on 4G/5G with dual-stack IPv6) receives a real, globally routable IPv6 address.
2. NetBird uses IPv6 ICE candidates to punch a **Direct WireGuard P2P connection**, bypassing CGNAT, UPnP, and relay servers completely!
3. Transfer speeds jump to the full upload capacity of your home internet connection with minimal latency (~25–35 ms).

#### Configuring IPv6 on Your Home Router / Gateway:

1. **WAN (Internet) Settings**:
   * Open your router's management dashboard -> navigate to the **IPv6** configuration for your WAN connection.
   * Set **IPv6 Connection** to **`DHCPv6`** (or DHCPv6 Prefix Delegation).
   * Set **Prefix Delegation Size** to **`56`** (or `64`, depending on your ISP specifications).
   * Save changes.

2. **LAN (Local Network) Settings**:
   * Navigate to your local network (LAN) settings -> **IPv6**.
   * Set **IPv6 Interface Type** to **`Prefix Delegation`** (delegated from your primary WAN).
   * Enable **Router Advertisement (RA)** and set to **`SLAAC`** (Stateless Address Autoconfiguration).
   * Save changes.

3. **Verify Global IPv6 on Ubuntu Server**:
   ```bash
   ip -6 addr show scope global
   ```
   You should see a global unicast address (with `scope global`).

4. **Verify Direct NetBird Connection**:
   Reconnect the NetBird app on your smartphone over cellular data, then run on the server:
   ```bash
   netbird status --detail
   ```
   The peer status will transition from `Relayed` to:
   ```text
   Connection type: Direct (P2P)
   ICE candidate endpoints (Local/Remote): [2a02:...]:51820 / [2a02:...]:51820
   No open router ports, no UPnP, and full direct WireGuard speed!

