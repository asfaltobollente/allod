# How-to: External Watch Sentinel (with Telegram Alerts & Weather Digest)

The **Allod Watch Sentinel** (`allod-watch`) is an ultra-lightweight monitoring daemon (consuming **less than 15 MB of RAM**) designed to run on an external server (such as an *always free-tier* cloud VPS or secondary Linux host).

The sentinel continuously inspects your primary Allod home node from the outside. If your home node loses power or internet connectivity, the sentinel immediately dispatches an alert via **Telegram**, notifies you the moment the node recovers, and sends a daily **positive morning digest with today's local weather forecast**.

---

## ⚡ Recommended Method: 100% Plug & Play via Web Dashboard

Starting in v1.2, **you no longer need to write configuration files manually on the VPS**: the entire setup wizard and VPS deployment can be done with a few clicks directly in the Allod Web Dashboard!

### 1. Open the Sentinel Wizard in Allod Panel
* Navigate to the **Modules** tab and click on **🤖 Configure Sentinel & Bot** on the `watch` module card (or under **Settings & Maintenance**).

### 2. Configure Your Telegram Bot in 3 Clicks
1. Click the link **`↗ Open @BotFather on Telegram`** to launch the official setup bot.
2. Send `/newbot`, choose your bot's name and username.
3. Paste the generated **Bot Token** into the field.
4. Open a chat with your new bot on Telegram, click **START** (or send any text), and press **`🔍 Auto-Detect Chat ID`**.
   *Allod automatically queries Telegram and populates your Chat ID!*
5. Click **`💬 Send Test Message`** to verify delivery on your phone immediately.

### 3. Customize Weather & Blackout Thresholds
* On the **Weather & Morning Digest** tab, configure your city (e.g. *Roma*, *London*, *New York*), morning report time (default *08:30*), and outage threshold (default *3 minutes*).

### 4. VPS Mesh Onboarding & 1-Command Deploy (All-in-One)
Because the home server is located behind **CGNAT with zero open inbound ports**, your external VPS must join the private encrypted **NetBird WireGuard Mesh** to query Allod's health endpoint (`/api/health`).

Allod automates this entirely by generating a single compound one-liner:
1. Paste a **Setup Key** from your NetBird dashboard (`https://app.netbird.io/setup-keys`).
2. Click **`📋 Copy Command for VPS`**. The dashboard generates:
   ```bash
   curl -fsSL https://pkgs.netbird.io/install.sh | sh && sudo netbird up --setup-key <KEY> && curl -fsSL http://100.x.x.x:8080/api/watch/install.sh | sudo bash
   ```
3. SSH into your external cloud VPS and paste the command:
   * Installs NetBird and connects the VPS to your private WireGuard mesh.
   * Downloads the precompiled `allod-watch` binary directly from your Allod server over the encrypted mesh.
   * Pre-fills `/etc/allod/watch.yaml` with your tokens, thresholds, and weather city.
   * Enables and starts the `allod-watch.service` systemd unit.
   * Sends an immediate confirmation message to your Telegram!

*(If your VPS is already joined to your NetBird mesh, toggle the checkbox in the UI to get the standalone install command).*

---

## 🏗️ Why an External Sentinel?

If your home suffers a power outage or ISP interruption:
* An alerting process running *inside* your home server **cannot notify you**, because it has neither electricity nor internet connectivity to send packets.
* An **independent external sentinel**, located in a cloud datacenter, detects the absence of responses from the outside and delivers an instant alert to your smartphone via Telegram.

---

## 🤖 Manual Setup Guide for Telegram Bot (Optional)

If you prefer to configure everything manually from the terminal:

### 1. Create a Bot with @BotFather
1. Open Telegram on your smartphone or desktop.
2. Search for the official account **`@BotFather`** (verified with a blue badge).
3. Start the chat and send:
   ```text
   /newbot
   ```
4. BotFather will prompt you for:
   * **Bot Name**: Display name of your choice (e.g., `Allod Sentinel`).
   * **Bot Username**: Must end with `bot` (e.g., `my_allod_alert_bot`).
5. BotFather will reply with your **HTTP API Token**:
   ```text
   Use this token to access the HTTP API:
   7123456789:AAHk...
   ```
   *Keep this token safe: this is your `bot_token`.*

### 2. Start the Chat with Your Bot
1. Click your bot's link provided by BotFather (e.g., `t.me/my_allod_alert_bot`).
2. Press **START** at the bottom (or send `/start`).
   *(This step is required: for privacy reasons Telegram does not allow bots to initiate messages to users who have not first started the chat).*

### 3. Retrieve Your Chat ID
You can find your numeric Chat ID in two ways:
* **Automatic Method**: Run `allod-watch get-chat-id -c watch.yaml` (after configuring your bot_token).
* **Direct Telegram Method**: Search for **`@userinfobot`** on Telegram, press START, and it will return your numeric **`Id`** (e.g., `123456789`).

---

## ⚙️ Sentinel Configuration (`watch.yaml`)

Create or edit `/etc/allod/watch.yaml` (or local `watch.yaml`):

```yaml
# Monitored Allod nodes
nodes:
  - id: "allod-home"
    # WireGuard Mesh IP (NetBird) or reachable endpoint:
    url: "http://100.100.64.229:8080/api/health"
    token: ""

# Telegram Bot configuration
telegram:
  enabled: true
  bot_token: "7123456789:AAHk..." # Provided by @BotFather
  chat_id: "123456789"            # Your numeric chat_id

# Local weather forecast for morning digest (Open-Meteo, 100% free, zero API key)
weather:
  enabled: true
  city: "Rome" # Set to your nearest city for local weather

# Check intervals & thresholds
intervals:
  check_seconds: 60           # Probe health every 60 seconds
  down_threshold_seconds: 180 # Trigger alert after 3 minutes without response (3 consecutive fails)

# Positive daily morning digest (M5)
digest:
  enabled: true
  time: "08:30" # Scheduled time for daily health and weather report (HH:MM)
```

---

## 🚀 Verification & Testing Commands

### 1. Test Telegram Connection
Verify your bot can send messages to your device:
```bash
./allod-watch test-telegram -c watch.yaml
```
You will receive an immediate confirmation message on Telegram!

### 2. Test Morning Digest with Weather
Preview the full morning digest report:
```bash
./allod-watch test-digest -c watch.yaml
```
Telegram will deliver a rich HTML report containing:
* 🌤️ Today's local weather forecast (conditions, min/max temperature, rain probability).
* 🏷️ Server node name and operational uptime.
* 💾 Storage pool integrity and free space.
* 🧠 RAM memory utilization.
* 🧩 Active container modules.

---

## 🛠️ Production Deployment on Linux / VPS (systemd)

1. **Build or copy the binary to your VPS**:
   ```bash
   # Cross-compile for Linux (amd64 or arm64):
   GOOS=linux GOARCH=amd64 go build -o allod-watch ./cmd/allod-watch
   
   # Install binary
   sudo install -m 0755 allod-watch /usr/local/bin/allod-watch
   ```

2. **Configure directory and settings**:
   ```bash
   sudo mkdir -p /etc/allod
   sudo cp configs/watch-sentinel.example.yaml /etc/allod/watch.yaml
   sudo nano /etc/allod/watch.yaml
   ```

3. **Install and enable systemd service**:
   ```bash
   sudo cp configs/allod-watch.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now allod-watch
   ```

4. **Inspect live logs**:
   ```bash
   journalctl -u allod-watch -f
   ```

---

## 🔔 Types of Notifications

### 1. 🚨 Blackout / Outage Alert (PEER_LOST)
Triggered when the node fails to respond for more than 3 consecutive minutes:
> 🚨 **ALLERTA ALLOD: NODO NON RAGGIUNGIBILE**  
> 🏷️ **Nodo:** `allod-home`  
> ⏱️ **Stato:** Non risponde da **3 min, 0 sec**  
> ⚠️ **Dettaglio:** connection refused or timeout  
> *Possibile blackout elettrico, riavvio o interruzione della connettività di rete.*

### 2. 💚 Recovery Notification (PEER_RECOVERED)
Triggered the exact moment the node reconnects:
> 💚 **RIENTRO ALLOD: NODO TORNATO ONLINE**  
> 🏷️ **Nodo:** `allod-home`  
> ✅ **Stato:** Operativo e rispondente  
> ⏱️ **Durata disservizio:** 14 min, 22 sec  
> *Tutti i servizi locali e mesh sono nuovamente operativi.*

### 3. ☀️ Daily Morning Digest with Weather
Triggered daily at 08:30:
> ☀️ **Buongiorno! Resoconto Allod**  
> 🌤️ **Meteo di Oggi:**  
> ☀️ **Rome**: Sereno, Min: **16°C** / Max: **26°C** (Prob. pioggia: 0%)  
> 🏷️ **Nodo:** `allod-home`  
> ⏱️ **Uptime:** 18 giorni, 4 ore  
> 💾 **Pool Storage:** Integro e Sano (Usati: 310 GB, Liberi: 690 GB)  
> 🧠 **Memoria RAM:** 2150 MB / 8192 MB  
> 🧩 **Servizi:** photos, shares, media, network  
> ✓ *Tutto regolare. I tuoi dati personali sono al sicuro.*
