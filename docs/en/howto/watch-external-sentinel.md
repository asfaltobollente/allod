# How-to: External Watch Sentinel (with Telegram Alerts & Weather Digest)

The **Allod Watch Sentinel** (`allod-watch`) is an independent, ultra-lightweight monitoring daemon (consuming **less than 15 MB of RAM**) designed to run on an external server (such as an *always free-tier* cloud VPS or secondary remote Linux host).

The sentinel continuously inspects your primary Allod home node from the outside. If your home node loses power or internet connectivity, the sentinel immediately dispatches an alert via **Telegram**, notifies you the moment the node recovers, and delivers a daily **positive morning digest with today's local weather forecast and node hardware vitals (CPU temperature, RAM, storage)**.

---

## 🏗️ Dual Architecture: Push HTTP vs Pull Mesh

Allod Watch supports two operating modes configurable directly from the web dashboard:

| Feature | 📡 Push HTTP Mode (Recommended) | ☁️ Pull Mesh Mode (NetBird) |
| :--- | :--- | :--- |
| **Architecture** | **Dead Man's Snitch** (Periodic outbound push to VPS) | **Prober Mesh** (Active polling from VPS to home) |
| **VPS Access** | The VPS **has no access** to your home network or mesh | The VPS is joined to your private WireGuard mesh |
| **Home Router Ports** | **0 (Zero)** — outbound HTTPS/HTTP push only | **0 (Zero)** — WireGuard NAT traversal |
| **VPS Requirements** | Open receiver port (e.g. `8443/tcp`) on VPS firewall | NetBird client installed with setup key |
| **OPSEC & Security** | **Maximum**: public VPS is strictly decoupled | Excellent: end-to-end encrypted WireGuard tunnel |
| **Deploy** | **1 single command** copy-and-paste via SSH | 1 command with NetBird registration & deploy |

---

## ⚡ Recommended Method: Push HTTP Mode (Dead Man's Snitch)

In **Push HTTP** mode, your Allod server sends an authenticated heartbeat payload every 60 seconds to the `allod-watch` receiver endpoint on your VPS (`POST /api/heartbeat`) using a shared secret token (`X-Allod-Token`).
If the VPS does not receive any heartbeat for **more than 5 minutes**, it declares a blackout and sends an immediate alert to Telegram. When connectivity resumes, it dispatches a recovery notification with the exact downtime duration.

### 1. Configuration in Allod Web Dashboard
1. Open the Allod web dashboard and navigate to **Modules** ➔ **🤖 Configure Sentinel & Bot** (or under **Settings**).
2. **Telegram Bot Tab**:
   - Click `↗ Open @BotFather on Telegram`, create your bot with `/newbot`, and paste the HTTP API token.
   - Start the chat with your bot on Telegram and click **`🔍 Auto-Detect Chat ID`**.
   - Click **`💬 Send Test Message`** to verify instant delivery on your phone.
3. **Weather & Morning Digest Tab**:
   - Set your city for local weather forecasts (powered by Open-Meteo, 100% free with no API keys needed).
   - Configure the morning schedule (default `08:30`) and outage threshold (default `5 minutes`).
4. **Architecture & VPS Deploy Tab**:
   - Select the **📡 Push HTTP (Dead Man's Snitch)** card.
   - Enter your VPS **Public Host or IP** (e.g. `129.150.x.x` or a domain name).
   - Set the receiver port (default `8443`).
   - Click **`⚡ Generate`** to create a secure random secret token (`X-Allod-Token`).
   - Click **`📋 Copy Command for VPS`**.

### 2. Run the Deployment Command on the VPS (10 Seconds)
SSH into your external VPS and paste the command:
```bash
# The self-contained command downloads allod-watch, configures /etc/allod/watch.yaml,
# opens the port in the UFW firewall, starts the systemd service, and verifies Telegram!
```
Once started, return to the Allod dashboard and click **`📡 Verify VPS Connection (Test Push)`** to confirm heartbeat receipt.

---

## ☁️ Alternative Mode: Pull Mesh (NetBird WireGuard)

If you prefer connecting the VPS directly to your private WireGuard mesh network:

1. In the Allod panel, select the **☁️ Pull Mesh (NetBird)** card.
2. Paste a **Setup Key** generated in your NetBird dashboard (`https://app.netbird.io/setup-keys`).
3. Click **`📋 Copy Command for VPS`**. The command will execute:
   ```bash
   curl -fsSL https://pkgs.netbird.io/install.sh | sh && sudo netbird up --setup-key <KEY> && curl -fsSL "http://<ALLOD-MESH-IP>:8080/api/watch/install.sh?mode=mesh" | sudo bash
   ```
4. The VPS connects to your private mesh and polls Allod's `/api/health` endpoint periodically.

---

## ⚙️ Configuration File Structure (`/etc/allod/watch.yaml`)

### Example Push Mode (`mode: receiver`)
```yaml
mode: receiver

receiver:
  port: 8443
  secret_token: "a1b2c3d4e5f67890abcdef1234567890"

intervals:
  down_threshold_seconds: 300 # Alert after 5 minutes of missing heartbeats

telegram:
  enabled: true
  bot_token: "7123456789:AAHk..."
  chat_id: "123456789"

weather:
  enabled: true
  city: "Roma"

digest:
  enabled: true
  time: "08:30"
```

### Example Mesh Mode (`mode: poller`)
```yaml
mode: poller

server:
  port: 9099

nodes:
  - id: "allod-node"
    url: "http://100.100.x.x:8080/api/health"

intervals:
  check_seconds: 60
  down_threshold_seconds: 180

telegram:
  enabled: true
  bot_token: "7123456789:AAHk..."
  chat_id: "123456789"

weather:
  enabled: true
  city: "Roma"

digest:
  enabled: true
  time: "08:30"
```

---

## 🚀 Terminal Commands & Testing

From your VPS shell, you can test daemon features directly:

```bash
# Test Telegram notifications
allod-watch test-telegram -c /etc/allod/watch.yaml

# Test morning digest generation with live weather and node vitals
allod-watch test-digest -c /etc/allod/watch.yaml

# View real-time systemd service logs
journalctl -u allod-watch -f

# Check systemd service status
systemctl status allod-watch
```

---

## 🔔 Telegram Notifications Overview

### 1. 🚨 Outage / Blackout Alert
Triggered promptly when heartbeats stop arriving:
> 🚨 **ALLOD ALERT: NODE UNREACHABLE**  
> 🏷️ **Node:** `allod-node`  
> ⏱️ **Status:** No heartbeat received for **5 min, 12 sec**  
> ⚠️ **Details:** Heartbeat timeout  
> *Possible power outage, server shutdown, or ISP connection loss.*

### 2. 💚 Recovery Notification
Dispatched the moment the first heartbeat resumes:
> 💚 **ALLOD RECOVERY: NODE BACK ONLINE**  
> 🏷️ **Node:** `allod-node`  
> ✅ **Status:** Heartbeat successfully restored  
> ⏱️ **Downtime duration:** 18 min, 40 sec  
> *All local and remote services are operational again.*

### 3. ☀️ Morning Digest (Daily at 08:30)
Delivered daily with local weather forecast and node hardware telemetry:
> ☀️ **Good morning! Allod Daily Report**  
> 🌤️ **Today's Weather:**  
> ☀️ **Roma**: Clear skies, Min: **16°C** / Max: **26°C** (Precipitation: 0%)  
> 🏷️ **Node:** `allod-node`  
> ⏱️ **Uptime:** 24 days, 6 hours  
> 🌡️ **CPU Temperature:** 42.5°C | Load: 0.18  
> 💾 **Storage Pool:** Healthy (Used: 310 GB, Free: 690 GB)  
> 🧠 **RAM Utilization:** 2150 MB / 8192 MB  
> 🧩 **Active Modules:** photos, shares, media, backup  
> ✓ *All systems nominal. Your personal cloud is secure.*
