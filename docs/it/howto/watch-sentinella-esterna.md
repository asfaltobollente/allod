# Guida Pratica: Allod Watch Sentinel su Server Esterno (con Notifiche Telegram e Meteo)

Il componente **Allod Watch Sentinel** (`allod-watch`) è un demone di monitoraggio indipendente ultra-leggero (occupa **meno di 15 MB di RAM**) progettato per essere eseguito su una VPS esterna (es. un'istanza *always free-tier* in cloud o un piccolo server Linux remoto).

La sentinella sorveglia continuamente dall'esterno il tuo server Allod domestico: se il nodo casalingo perde la connessione a Internet o subisce un blackout elettrico, la sentinella invia immediatamente un allarme su **Telegram**, ti avvisa non appena il server torna online, e ogni mattina ti invia un **resoconto positivo con il meteo locale della giornata e lo stato vitale del sistema (temperatura CPU, memoria, storage)**.

---

## 🏗️ Architettura Doppia: Push HTTP vs Pull Mesh

Allod Watch supporta due modalità di funzionamento selezionabili dalla dashboard web:

| Caratteristica | 📡 Modalità Push HTTP (Consigliata) | ☁️ Modalità Pull Mesh (NetBird) |
| :--- | :--- | :--- |
| **Architettura** | **Dead Man's Snitch** (Push periodico da casa a VPS) | **Prober Mesh** (Polling attivo da VPS a casa) |
| **Accesso VPS** | La VPS **non ha accesso** alla rete domestica né al mesh | La VPS fa parte della rete privata mesh WireGuard |
| **Porte sul router domestico** | **0 (Zero)** — solo connessioni in uscita verso la VPS | **0 (Zero)** — NAT traversal WireGuard |
| **Requisiti VPS** | Porta ricevitore aperta (es. `8443/tcp`) nel firewall VPS | Client NetBird installato e registrato con setup key |
| **Sicurezza OPSEC** | **Massima**: VPS pubblica completamente isolata | Ottima: tunnel WireGuard cifrato punto-punto |
| **Deploy** | **1 singolo comando** copia-e-incolla via SSH | 1 comando con registrazione NetBird e deploy |

---

## ⚡ Metodo Consigliato: Modalità Push HTTP (Dead Man's Snitch)

Nella modalità **Push HTTP**, il tuo server Allod invia un battito cardiaco crittografato ogni 60 secondi all'endpoint del demone `allod-watch` sulla VPS (`POST /api/heartbeat`) con autenticazione via token segreto (`X-Allod-Token`).
Se la VPS non riceve alcun segnale per **più di 5 minuti**, dichiara il blackout e invia subito l'allarme su Telegram. Al ripristino della connettività, invia la notifica di rientro con la durata esatta dell'interruzione.

### 1. Configurazione dalla Dashboard Web Allod
1. Accedi alla dashboard web di Allod e apri **Moduli** ➔ **🤖 Configura Sentinella & Bot** (oppure da **Settings**).
2. **Tab Bot Telegram**:
   - Clicca su `↗ Apri @BotFather su Telegram`, crea il bot con `/newbot` e incolla il token HTTP API.
   - Avvia la chat col tuo bot su Telegram e clicca **`🔍 Rileva Chat ID Automaticamente`**.
   - Clicca **`💬 Invia Messaggio di Prova`** per verificare la ricezione istantanea.
3. **Tab Meteo & Resoconto**:
   - Imposta la tua città per le previsioni meteorologiche locali (Open-Meteo, 100% gratuita senza API key).
   - Scegli l'orario del buongiorno (default `08:30`) e la soglia di allarme blackout (default `5 minuti`).
4. **Tab Architettura & Deploy VPS**:
   - Seleziona la card **📡 Push HTTP (Dead Man's Snitch)**.
   - Inserisci l'**Host o IP Pubblico** della tua VPS (es. `129.150.x.x` o un tuo dominio).
   - Imposta la porta del ricevitore (predefinita `8443`).
   - Clicca **`⚡ Genera`** per creare un token casuale sicuro (`X-Allod-Token`).
   - Clicca **`📋 Copia Comando per VPS`**.

### 2. Esecuzione del Comando sulla VPS (10 Secondi)
Connettiti via SSH alla tua VPS esterna e incolla il comando copiato:
```bash
# Il comando autogenerato scarica allod-watch, configura /etc/allod/watch.yaml,
# apre la porta nel firewall UFW, attiva systemd e invia il test su Telegram!
```
Una volta avviato, torna nella dashboard di Allod e clicca su **`📡 Verifica Connessione VPS (Test Push)`** per confermare la ricezione dell'heartbeat.

---

## ☁️ Modalità Alternativa: Pull Mesh (NetBird WireGuard)

Se preferisci collegare la VPS direttamente alla tua rete privata WireGuard:

1. Nel pannello Allod, seleziona la card **☁️ Pull Mesh (NetBird)**.
2. Incolla una **Setup Key** generata dalla dashboard di NetBird (`https://app.netbird.io/setup-keys`).
3. Clicca **`📋 Copia Comando per VPS`**. Il comando eseguirà:
   ```bash
   curl -fsSL https://pkgs.netbird.io/install.sh | sh && sudo netbird up --setup-key <KEY> && curl -fsSL "http://<IP-MESH-ALLOD>:8080/api/watch/install.sh?mode=mesh" | sudo bash
   ```
4. La VPS entrerà nella rete privata ed effettuerà il polling periodico dell'endpoint `/api/health`.

---

## ⚙️ Struttura Configurazione (`/etc/allod/watch.yaml`)

### Esempio Modalità Push (`mode: receiver`)
```yaml
mode: receiver

receiver:
  port: 8443
  secret_token: "a1b2c3d4e5f67890abcdef1234567890"

intervals:
  down_threshold_seconds: 300 # Allarme dopo 5 minuti di assenza segnale

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

### Esempio Modalità Mesh (`mode: poller`)
```yaml
mode: poller

server:
  port: 9099

nodes:
  - id: "allod-casa"
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

## 🚀 Comandi Manuali da Terminale

Se ti trovi collegato alla VPS e vuoi testare le singole funzionalità:

```bash
# Test notifica Telegram
allod-watch test-telegram -c /etc/allod/watch.yaml

# Test invio resoconto mattutino con meteo e salute server
allod-watch test-digest -c /etc/allod/watch.yaml

# Visualizzazione log in tempo reale del demone systemd
journalctl -u allod-watch -f

# Stato del servizio systemd
systemctl status allod-watch
```

---

## 🔔 Notifiche Ricevute su Telegram

### 1. 🚨 Allarme Blackout / Server Offline
Inviato tempestivamente non appena il server smette di trasmettere:
> 🚨 **ALLERTA ALLOD: NODO NON RAGGIUNGIBILE**  
> 🏷️ **Nodo:** `allod-node`  
> ⏱️ **Stato:** Nessun battito cardiaco da **5 min, 12 sec**  
> ⚠️ **Dettaglio:** Mancata ricezione heartbeat  
> *Possibile blackout elettrico, spegnimento del server o assenza di linea internet.*

### 2. 💚 Notifica di Rientro Online
Inviato nel momento esatto in cui il primo nuovo heartbeat raggiunge il VPS:
> 💚 **RIENTRO ALLOD: NODO TORNATO ONLINE**  
> 🏷️ **Nodo:** `allod-node`  
> ✅ **Stato:** Battito cardiaco ripristinato  
> ⏱️ **Durata disservizio:** 18 min, 40 sec  
> *Tutti i servizi locali e remoti sono nuovamente operativi.*

### 3. ☀️ Resoconto del Mattino (Morning Digest)
Inviato ogni giorno all'orario stabilito (es. 08:30):
> ☀️ **Buongiorno! Resoconto Allod**  
> 🌤️ **Meteo di Oggi:**  
> ☀️ **Roma**: Sereno, Min: **16°C** / Max: **26°C** (Prob. pioggia: 0%)  
> 🏷️ **Nodo:** `allod-node`  
> ⏱️ **Uptime:** 24 giorni, 6 ore  
> 🌡️ **Temperatura CPU:** 42.5°C | Carico: 0.18  
> 💾 **Pool Storage:** Integro e Sano (Usati: 310 GB, Liberi: 690 GB)  
> 🧠 **Memoria RAM:** 2150 MB / 8192 MB  
> 🧩 **Moduli Attivi:** photos, shares, media, backup  
> ✓ *Tutto regolare. I tuoi dati personali sono al sicuro.*
