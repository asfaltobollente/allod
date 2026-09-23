# Guida Pratica: Allod Watch Sentinel su Server Esterno (con Notifiche Telegram e Meteo)

Il componente **Allod Watch Sentinel** (`allod-watch`) è un demone di monitoraggio ultra-leggero (occupa **meno di 15 MB di RAM**) progettato per essere eseguito su una VPS esterna (es. un'istanza *always free-tier* o un piccolo server Linux in cloud).

La sentinella sorveglia continuamente dall'esterno il tuo server Allod domestico: se il nodo casalingo perde la connessione a Internet o subisce un blackout elettrico, la sentinella invia immediatamente un allarme su **Telegram**, ti avvisa non appena il server torna online, e ogni mattina ti invia un **resoconto positivo con il meteo della giornata**.

---

## ⚡ Metodo Consigliato: Configurazione 100% Plug & Play dalla Dashboard Web

Dalla versione v1.2, **non è più necessario configurare file a mano sulla VPS**: l'intera procedura di configurazione del Bot Telegram e il deploy della VPS si esegue con pochi clic direttamente dalla Web Dashboard di Allod!

### 1. Apri la Procedura Guidata nel Pannello Allod
* Vai nella scheda **Moduli** e clicca su **🤖 Configura Sentinella & Bot** nella card del modulo `watch` (oppure in **Settings & Manutenzione**).

### 2. Configura il Bot Telegram in 3 Clic
1. Clicca sul link **`↗ Apri @BotFather su Telegram`** per avviare la chat ufficiale.
2. Invia il comando `/newbot`, scegli il nome del bot e il suo username.
3. Incolla il **Bot Token** nel campo apposito del pannello.
4. Apri la chat con il tuo nuovo bot su Telegram, premi **AVVIA** (o invia un messaggio), e clicca sul pulsante magico **`🔍 Rileva Chat ID Automaticamente`**.
   *Allod interrogherà Telegram e compilerà il tuo Chat ID in automatico!*
5. Clicca su **`💬 Invia Messaggio di Prova`** per ricevere subito una notifica di conferma sul telefono.

### 3. Personalizza Meteo e Soglie
* Nella scheda **Meteo & Resoconto**, imposta la tua città (es. *Roma*, *Milano*, *Napoli*), l'orario di invio del buongiorno (default *08:30*) e la soglia di allarme blackout (default *3 minuti*).

### 4. Messa in Mesh della VPS & Deploy in 1 Comando (All-in-One)
Poiché il server Allod si trova in casa dietro una rete **CGNAT senza porte pubbliche aperte sul router**, la VPS esterna deve entrare nella rete privata crittografata **NetBird WireGuard Mesh** per poter interrogare lo stato di salute di Allod (`/api/health`).

Allod risolve questo passaggio generando un **singolo comando combinato** che fa tutto in automatico:
1. Copia una **Setup Key** dalla tua dashboard NetBird (`https://app.netbird.io/setup-keys`) e incollala nel pannello Allod.
2. Clicca su **`📋 Copia Comando per VPS`**. Il pannello genererà un comando simile a:
   ```bash
   curl -fsSL https://pkgs.netbird.io/install.sh | sh && sudo netbird up --setup-key <KEY> && curl -fsSL http://100.x.x.x:8080/api/watch/install.sh | sudo bash
   ```
3. Connettiti via SSH alla tua VPS esterna e incolla il comando:
   * Installa NetBird e collega la VPS alla tua rete WireGuard privata.
   * Scarica il binario `allod-watch` precompilato direttamente dal nodo Allod via tunnel sicuro.
   * Crea la configurazione `/etc/allod/watch.yaml` con tutti i tuoi parametri già compilati.
   * Registra e avvia il servizio di sistema systemd.
   * Invia un ping istantaneo su Telegram per confermarti che la sentinella è attiva!

*(Se la VPS è già collegata alla rete NetBird, basta attivare la spunta corrispondente nel pannello per ottenere il comando semplificato di sola installazione).*

---

## 🏗️ Perché una Sentinella Esterna?

Se a casa salta la corrente o cade la connessione del provider:
* Un servizio di allarme in esecuzione *dentro* il server Allod **non può avvisarti**, perché non ha più energia né linea internet per inviare messaggi.
* Una **sentinella esterna indipendente**, collocata in un datacenter cloud, rileva l'assenza di risposte dall'esterno e ti invia subito l'allarme sullo smartphone via Telegram.

---

## 🤖 Guida Manuale Dettagliata per il Bot Telegram (Opzionale)

Se preferisci eseguire l'intera procedura manualmente da terminale:

### 1. Crea il Bot con @BotFather
1. Apri Telegram sul tuo smartphone o PC.
2. Cerca l'utente ufficiale **`@BotFather`** (ha la spunta blu di verifica).
3. Avvia la chat e invia il comando:
   ```text
   /newbot
   ```
4. BotFather ti chiederà due informazioni:
   * **Nome del bot**: un nome visualizzato a tua scelta (es: `Allod Sentinel`).
   * **Username del bot**: deve finire obbligatoriamente per `bot` (es: `mio_allod_alert_bot`).
5. BotFather ti risponderà con un messaggio contenente il tuo **Token HTTP API**:
   ```text
   Use this token to access the HTTP API:
   7123456789:AAHk...
   ```
   *Salva questo token: sarà il tuo `bot_token`.*

### 2. Avvia la Chat con il tuo Nuovo Bot
1. Clicca sul link del tuo bot fornito da BotFather (es: `t.me/mio_allod_alert_bot`).
2. Premi il pulsante **AVVIA** in basso (oppure scrivigli `/start`).
   *(Questo passaggio è indispensabile: per motivi di privacy Telegram non consente ai bot di inviare messaggi a chi non li ha prima avviati).*

### 3. Recupera il tuo Chat ID
Puoi recuperare il tuo Chat ID numerico in due modi:
* **Metodo Automatico**: Esegui sul terminale `allod-watch get-chat-id -c watch.yaml` (dopo aver inserito il bot_token).
* **Metodo Immediato via Telegram**: Cerca su Telegram il bot **`@userinfobot`**, premigli AVVIA e ti risponderà subito con il tuo **`Id`** numerico (es: `123456789`).

---

## ⚙️ Configurazione della Sentinella (`watch.yaml`)

Crea o modifica il file `/etc/allod/watch.yaml` (o locale `watch.yaml`):

```yaml
# Nodi Allod da sorvegliare
nodes:
  - id: "allod-casa"
    # Indirizzo WireGuard Mesh (NetBird) o IP del nodo Allod:
    url: "http://100.100.64.229:8080/api/health"
    token: ""

# Configurazione Bot Telegram
telegram:
  enabled: true
  bot_token: "7123456789:AAHk..." # Inserisci il token fornito da @BotFather
  chat_id: "123456789"            # Inserisci il tuo chat_id numerico

# Previsioni meteo nel resoconto del mattino (Open-Meteo, 100% gratuita senza API key)
weather:
  enabled: true
  city: "Roma" # Inserisci la tua città per le previsioni locali

# Frequenza controlli e soglia allarmi
intervals:
  check_seconds: 60           # Verifica la salute del nodo ogni 60 secondi
  down_threshold_seconds: 180 # Invia allarme dopo 3 minuti di assenza di risposte (3 fallimenti consecutivi)

# Resoconto mattutino positivo (M5)
digest:
  enabled: true
  time: "08:30" # Orario di invio del buongiorno quotidiano con meteo e salute server (HH:MM)
```

---

## 🚀 Comandi e Collaudo

### 1. Test Connessione Telegram
Verifica che il bot riesca a scriverti:
```bash
./allod-watch test-telegram -c watch.yaml
```
Riceverai subito un messaggio di conferma su Telegram!

### 2. Test Resoconto Mattutino con Meteo
Verifica come si presenta il resoconto completo:
```bash
./allod-watch test-digest -c watch.yaml
```
Telegram ti invierà un messaggio formattato con:
* 🌤️ Previsioni meteo della tua città (condizione, temperature min/max e probabilità di pioggia).
* 🏷️ Nome del server e tempo di attività (Uptime).
* 💾 Stato di integrità del pool dischi e spazio libero.
* 🧠 Utilizzo memoria RAM.
* 🧩 Servizi attivi.

---

## 🛠️ Installazione Permanente su Linux / VPS (systemd)

1. **Compila o copia il binario sulla VPS**:
   ```bash
   # Compilazione (puoi compilare anche su PC per Linux amd64 o arm64):
   GOOS=linux GOARCH=amd64 go build -o allod-watch ./cmd/allod-watch
   
   # Installa il binario
   sudo install -m 0755 allod-watch /usr/local/bin/allod-watch
   ```

2. **Crea la directory e copia la configurazione**:
   ```bash
   sudo mkdir -p /etc/allod
   sudo cp configs/watch-sentinel.example.yaml /etc/allod/watch.yaml
   # Modifica il file con i tuoi dati:
   sudo nano /etc/allod/watch.yaml
   ```

3. **Installa e attiva il servizio systemd**:
   ```bash
   sudo cp configs/allod-watch.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now allod-watch
   ```

4. **Verifica i log in tempo reale**:
   ```bash
   journalctl -u allod-watch -f
   ```

---

## 🔔 Tipologie di Notifiche Ricevute

### 1. 🚨 Allarme Blackout / Nodo Offline (PEER_LOST)
Inviato non appena il nodo non risponde per oltre 3 minuti consecutivi:
> 🚨 **ALLERTA ALLOD: NODO NON RAGGIUNGIBILE**  
> 🏷️ **Nodo:** `allod-casa`  
> ⏱️ **Stato:** Non risponde da **3 min, 0 sec**  
> ⚠️ **Dettaglio:** connessione rifiutata o timeout  
> *Possibile blackout elettrico, riavvio o interruzione della connettività di rete.*

### 2. 💚 Notifica di Rientro Online (PEER_RECOVERED)
Inviato nel momento esatto in cui il nodo si ricollega:
> 💚 **RIENTRO ALLOD: NODO TORNATO ONLINE**  
> 🏷️ **Nodo:** `allod-casa`  
> ✅ **Stato:** Operativo e rispondente  
> ⏱️ **Durata disservizio:** 14 min, 22 sec  
> *Tutti i servizi locali e mesh sono nuovamente operativi.*

### 3. ☀️ Resoconto Positivo del Mattino con Meteo (Morning Digest)
Inviato ogni giorno alle 08:30:
> ☀️ **Buongiorno! Resoconto Allod**  
> 🌤️ **Meteo di Oggi:**  
> ☀️ **Roma**: Sereno, Min: **16°C** / Max: **26°C** (Prob. pioggia: 0%)  
> 🏷️ **Nodo:** `allod-casa`  
> ⏱️ **Uptime:** 18 giorni, 4 ore  
> 💾 **Pool Storage:** Integro e Sano (Usati: 310 GB, Liberi: 690 GB)  
> 🧠 **Memoria RAM:** 2150 MB / 8192 MB  
> 🧩 **Servizi:** photos, shares, media, network  
> ✓ *Tutto regolare. I tuoi dati personali sono al sicuro.*
