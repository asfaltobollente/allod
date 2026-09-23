# Guida Pratica: Accesso Remoto Sovrano con NetBird Mesh (Cloud & Self-Hosted)

Il modulo di rete **NetBird Mesh** di Allod fornisce accesso remoto cifrato end-to-end e zero-trust al proprio cloud domestico (Immich, Jellyfin, Nextcloud, Samba e Pannello Web) da qualsiasi smartphone o computer nel mondo **senza aprire alcuna porta sul router di casa**.

---

## 🏗️ Panoramica dell'Architettura

NetBird separa nettamente il coordinamento del segnale dal trasferimento dati ad alta velocità:

1. **Segnalazione e Rilevamento Peer (Control Plane)**:
   * **Modalità `cloud`**: Si connette all'infrastruttura cloud europea di NetBird (Francoforte, Germania — 100% conforme GDPR, gratuita fino a 100 nodi e 5 utenti).
   * **Modalità `selfhosted`**: Si connette al proprio server di gestione NetBird privato ospitato su un VPS o server dedicato.
   * Utilizza WebRTC (ICE, STUN, TURN) per l'attraversamento automatico del NAT anche dietro CGNAT (Carrier-Grade NAT), connessioni satellitari/FWA, operatori mobili 4G/5G e firewall restrittivi, con **zero porte aperte sul router**.
2. **Dati Cifrati ad Alta Velocità (Data Plane)**:
   * Lo smartphone o laptop si associa tramite l'app ufficiale open source NetBird.
   * Tutto il traffico delle applicazioni scorre **direttamente peer-to-peer (P2P)** tra il proprio dispositivo e il nodo Allod via WireGuard (`wt0`).
   * Supporto nativo per:
     * **Condivisioni Samba (TCP 445)** direttamente nell'app File di iOS e nei gestori file di Android.
     * **Streaming diretto 4K con Jellyfin** senza limiti artificiali di banda.
     * **Backup automatico foto e video con Immich** in background dal telefono.

---

## 📋 Prerequisiti

* Un nodo Allod funzionante con il modulo `network` impostato a `cloud` (consigliato) oppure `selfhosted`.
* Un account gratuito su [netbird.io](https://netbird.io) (per la modalità `cloud`) o un server NetBird attivo (per `selfhosted`).
* L'app ufficiale NetBird installata su smartphone o PC (iOS, Android, Windows, macOS, Linux).

---

## 🚀 Configurazione Passo-Passo

### 1. Generare una Setup Key su NetBird
1. Accedi al pannello di gestione NetBird ([app.netbird.io](https://app.netbird.io) o la tua istanza self-hosted).
2. Vai su **Setup Keys** e clicca **Add Key**.
3. Seleziona il tipo (es. `Reusable` per i server) e imposta la scadenza.
4. Copia la Setup Key generata (es. `4A8B7C21-D4E5-...`).

### 2. Configurare il Modulo nel Pannello Allod
1. Apri il browser all'indirizzo `http://<IP-SERVER>:8080` ed entra come amministratore.
2. Vai nella sezione **Moduli** e trova la scheda **🌐 NetBird Sovereign Mesh**.
3. Clicca su **⚙️ Configura NetBird**.
4. Seleziona la modalità operativa, incolla la **Setup Key** e clicca **Salva & Riavvia NetBird**.

### 3. Connettere lo Smartphone in 4G/5G
1. Apri l'app **NetBird** sul telefono.
2. Esegui il login con le stesse credenziali NetBird o inserisci una Setup Key.
3. Attiva la levetta su **Connected**.
4. L'indicatore VPN diventa verde: il telefono è connesso alla rete mesh WireGuard.

### 4. Accedere ai Servizi
Tramite l'IP NetBird del server (es. `100.100.64.229`), puoi raggiungere tutti i servizi ovunque ti trovi:
* **Pannello Allod**: `http://100.100.64.229:8080`
* **Immich Foto**: `http://100.100.64.229:2283`
* **Jellyfin Media**: `http://100.100.64.229:8096`
* **Samba Shares**: `smb://100.100.64.229/shares`

---

## 🔍 Risoluzione Problemi e Prestazioni di Rete (CGNAT, ISP Senza IP Pubblico & UPnP)

### Perché lo Speedtest da Remoto è Basso (~9-10 Mbps) e il Ping è Alto (~90 ms)?

Se eseguendo uno speedtest da smartphone in 4G verso il server Allod riscontri una velocità bloccata attorno ai 9–10 Mbps e circa 90 ms di latenza (mentre a casa in Wi-Fi navighi alla massima velocità di linea), esegui sul server:

```bash
netbird status --detail
```

Controlla il campo del peer relativo allo smartphone:
* `Connection type: Direct (P2P)`: Connessione WireGuard diretta. I dati viaggiano direttamente tra telefono e server alla massima velocità consentita dall'upload della propria connessione internet.
* `Connection type: Relayed`: La connessione passa attraverso i server relay pubblici di NetBird (es. Francoforte, Germania).

> [!NOTE]
> I relay pubblici gratuiti di NetBird applicano un limite cautelativo di banda di circa **10 Mbps per singolo flusso** per evitare abusi di traffico, e il percorso telefono ➡️ Germania ➡️ server aggiunge circa 40–80 ms di latenza.

---

### Perché UPnP Fallisce Dietro CGNAT e Provider Senza IP Pubblico Diretto

Eseguendo un test UPnP sul server con `upnpc -l`:
```text
Found a (not connected?) IGD : http://192.168.0.1:38179/ctl/IPConn
No valid UPNP Internet Gateway Device found.
```

Questo comportamento è del tutto fisiologico quando il router o firewall domestico è collegato a un ISP che adotta il **Carrier-Grade NAT (CGNAT)** — tipico di connessioni satellitari, ponti radio FWA, modem 4G/5G e molti operatori in fibra ottica:
1. **Nessun IPv4 Pubblico sulla WAN**: L'operatore non assegna un IPv4 pubblico all'utente residenziale. La porta WAN del router riceve un IP privato della sottorete CGNAT `100.64.0.0/10`.
2. **Rifiuto UPnP**: Il router segnala a UPnP che la propria interfaccia esterna non è collegata direttamente a un gateway pubblico internet (`No valid IGD`).
3. **Impossibilità di Port Forwarding IPv4**: Anche aprendo manualmente le porte sul firewall locale, i nodi e gateway a monte dell'operatore scartano qualsiasi connessione IPv4 in entrata non richiesta.
4. **NAT Simmetrico**: Poiché sia la rete mobile (4G/5G) sia la connessione di casa sono dietro CGNAT simmetrico, l'UDP Hole Punching automatico su IPv4 non riesce ad aprire un tunnel diretto, costringendo NetBird alla modalità **Relayed**.

---

### 🚀 La Soluzione Definitiva: Connessione Diretta P2P via IPv6 (Senza Porte Aperte)

Mentre molti provider non forniscono un IPv4 pubblico (se non a pagamento o con contratti business dedicati), quasi tutti gli operatori moderni e i provider satellitari/fibra assegnano nativamente una delega di prefisso IPv6 pubblico (spesso una subnet `/56` o `/64` tramite DHCPv6-PD)!

NetBird supporta nativamente lo scambio di candidati ICE sia IPv4 che IPv6. Con IPv6 attivo:
1. Sia il server Allod sia lo smartphone in 4G/5G (la maggior parte degli operatori mobili assegna connettività dual-stack IPv4/IPv6) ottengono un indirizzo IPv6 globale pubblico.
2. NetBird scopre l'indirizzo IPv6 e stabilisce un **tunnel WireGuard Diretto P2P**, superando completamente CGNAT, UPnP e i server relay!
3. La velocità sale al massimo dell'upload reale della propria linea internet, con latenza minima (25–35 ms).

#### Come Attivare IPv6 sul Proprio Router / Gateway Domestico:

1. **Configurazione WAN (Internet)**:
   * Nel pannello di amministrazione del proprio router, cercare la sezione **IPv6** della connessione WAN.
   * Impostare il tipo di connessione su **`DHCPv6`** (o DHCPv6 Prefix Delegation).
   * Impostare la dimensione della delega del prefisso (**Prefix Delegation Size**) a **`56`** (o `64` a seconda dell'operatore).
   * Salvare la configurazione.

2. **Configurazione LAN (Rete Locale)**:
   * Nella sezione delle reti locali (LAN), individuare le impostazioni **IPv6**.
   * Impostare il tipo di interfaccia su **`Prefix Delegation`** (associata alla WAN) o SLAAC.
   * Abilitare **Router Advertisement (RA)** su modalità automatica / **`SLAAC`**.
   * Salvare la configurazione.

3. **Verifica sul Server Ubuntu**:
   ```bash
   ip -6 addr show scope global
   ```
   Dovresti visualizzare un indirizzo IPv6 pubblico globale (es. con scope `global`).

4. **Verifica su NetBird**:
   Riconnetti l'app NetBird sul telefono e riesegui sul server:
   ```bash
   netbird status --detail
   ```
   Lo stato del peer passerà a:
   ```text
   Connection type: Direct (P2P)
   ICE candidate endpoints (Local/Remote): [2a02:...]:51820 / [2a02:...]:51820
   ```
   Tutto senza aprire porte sul router, senza UPnP e a piena banda!
