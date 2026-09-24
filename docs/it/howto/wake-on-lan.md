# Guida Pratica: Wake-on-LAN (WoL) Nativo in Allod

Allod include un motore **Wake-on-LAN (WoL)** nativo scritto interamente in Go puro, che consente di accendere da remoto il computer desktop principale o qualsiasi workstation della rete locale con **un solo clic dalla Web Dashboard** o tramite un **semplice comando da terminale**, senza doversi collegare via SSH al server.

---

## ⚡ Caratteristiche del Motore WoL di Allod

1. **Zero dipendenze esterne**: Nessun pacchetto Debian/Ubuntu aggiuntivo (come `wakeonlan` o `etherwake`) da installare. Il pacchetto AMD Magic Packet (102 byte standard su UDP porta 9) viene generato nativamente in memoria.
2. **Broadcast multi-interfaccia intelligente**:
   * Su server dotati di interfacce mesh private crittografate (es. WireGuard o NetBird), il traffico broadcast globale `255.255.255.255` potrebbe talvolta essere instradato erroneamente sulle interfacce virtuali.
   * Il motore WoL di Allod rileva automaticamente tutte le schede di rete fisiche attive con flag broadcast (`net.FlagBroadcast`), calcola l'indirizzo broadcast specifico della subnet locale (es. `192.168.1.255` o `192.168.0.255`) e invia una copia del pacchetto su ciascuna di esse, garantendo che il frame raggiunga sempre lo switch di rete e la scheda del computer spento.
3. **Persistenza in `state.db`**: I computer salvati (Nome, MAC, Porta, Broadcast IP) sono memorizzati in modo permanente nel database SQLite di Allod, con tracciamento dell'orario dell'ultima accensione.
4. **Zero permessi root**: L'apertura e la trasmissione su socket UDP broadcast su porta 9 non richiede privilegi di root.

---

## 🖥️ Configurazione del Computer di Destinazione

Affinché una scheda di rete possa ricevere il Magic Packet e accendere il PC, il computer di destinazione deve essere configurato correttamente sia nel BIOS/UEFI che nel sistema operativo.

### 1. Impostazioni BIOS/UEFI (Motherboard)
1. Riavvia il computer ed entra nel BIOS/UEFI (premendo ripetutamente `CANC`, `DEL`, o `F2` durante l'avvio).
2. Individua la sezione **Advanced**, **Power Management** o **ACPI Configuration**:
   * **Wake on LAN / WOL**: Imposta su **Enabled**.
   * **Power On By PCI-E Device**: Imposta su **Enabled**.
   * **ErP / EuP Ready**: Imposta su **Disabled** (lo standard ErP disattiva l'alimentazione a riposo della scheda di rete per risparmiare energia, impedendo l'ascolto del Magic Packet).
   * **Deep Sleep**: Imposta su **Disabled**.
3. Salva le modifiche e avvia il sistema.

---

### 2. Configurazione su Windows (PC Client)

Su Windows è necessario verificare due aspetti: le proprietà della scheda di rete e l'Avvio Rapido (*Fast Startup*).

#### A. Proprietà della Scheda di Rete
1. Premi `Win + X` e seleziona **Gestione dispositivi** (*Device Manager*).
2. Espandi la categoria **Schede di rete** e fai doppio clic sulla tua scheda Ethernet fisica (es. *Realtek Gaming GbE*, *Intel Ethernet Controller*).
3. Vai nella scheda **Risparmio energia** (*Power Management*):
   * Spunta **Consenti al dispositivo di riattivare il computer** (*Allow this device to wake the computer*).
   * Spunta **Consenti solo a Magic Packet di riattivare il computer** (*Only allow a magic packet to wake the computer*).
4. Vai nella scheda **Avanzate** (*Advanced*):
   * Trova la voce **Wake on Magic Packet** (o *Attivazione con pacchetto Magic*) e impostala su **Abilitato** (*Enabled*).
   * Se presente, imposta **Spegnimento con riattivazione LAN** (*Shutdown Wake-On-Lan*) su **Abilitato**.

#### B. Disattivazione dell'Avvio Rapido (Consigliato)
La funzionalità di "Avvio rapido" (*Fast Startup*) di Windows mette il computer in uno stato di ibernazione ibrida (S4) che su molte schede madri disalimenta completamente la porta Ethernet:
1. Premi `Win + R`, digita `powercfg.cpl` e premi Invio.
2. Clicca su **Specifica comportamento pulsanti di alimentazione** nel menu a sinistra.
3. Clicca su **Modifica le impostazioni attualmente non disponibili**.
4. Nella sezione in basso, deseleziona la casella **Attiva avvio rapido (scelta consigliata)**.
5. Clicca su **Salva modifiche**.

---

### 3. Configurazione su Linux (Workstation Client)

Se il computer da accendere monta Linux (Ubuntu, Debian, Fedora, Arch):

1. Verifica lo stato attuale del WoL con `ethtool`:
   ```bash
   sudo ethtool <nome-interfaccia-eth> | grep Wake-on
   # Esempio:
   # Supports Wake-on: pumbg
   # Wake-on: d   <-- 'd' significa disabilitato
   ```
2. Abilita il Magic Packet:
   ```bash
   sudo ethtool -s <nome-interfaccia-eth> wol g
   ```
3. Per rendere l'impostazione persistente ai riavvii tramite NetworkManager o udev:
   ```bash
   sudo nmcli connection modify "<NomeConnessione>" 802-3-ethernet.wake-on-lan magic
   ```

---

## 🌐 Utilizzo dalla Dashboard Web Allod

Allod offre un'integrazione immediata su tutte le sezioni del pannello:

1. **Launchpad (Accesso Rapido 1-Clic)**:
   * Sotto la griglia delle applicazioni personali è presente la card **⚡ Wake-on-LAN**: ogni computer salvato compare con un pulsante diretto **`⚡ Accendi`** e l'indicazione dell'orario dell'ultima accensione.
   * Con un semplice clic, il pacchetto Magic Packet viene inviato istantaneamente e viene mostrato un banner di conferma con i dettagli della subnet raggiunta.
2. **Finestra di Gestione Dedicata (`wol-modal`)**:
   * Clicca su **`⚡ Wake-on-LAN`** nella barra superiore del Launchpad o nella sezione **Impostazioni**.
   * Da qui puoi:
     * Consultare la tabella con tutti i computer salvati, MAC address, porta UDP e data di ultima accensione.
     * Salvare nuovi computer o modificare quelli esistenti.
     * Utilizzare la sezione **Accensione Rapida per MAC** per svegliare qualsiasi scheda di rete inserendo al volo un MAC address senza bisogno di salvarlo.

---

## ⌨️ Utilizzo da Riga di Comando (CLI Allod)

Allod mette a disposizione il comando `allod wol`:

### 1. Accendere un computer salvato per nome
```bash
allod wol wake "PC Principale"
```

### 2. Accendere al volo un computer tramite indirizzo MAC
```bash
allod wol wake 00:D8:61:33:0E:1F
```
*(Accetta formati con due punti `00:11:22:33:44:55`, trattini `00-11-22-33-44-55` o compatti `001122334455`).*

### 3. Visualizzare l'elenco dei dispositivi memorizzati
```bash
allod wol list
```
*Output di esempio:*
```
ID    NOME                  MAC ADDRESS         BROADCAST           PORTA   ULTIMA ACCENSIONE   
----------------------------------------------------------------------------------------------
1     PC Principale         00:d8:61:33:0e:1f   255.255.255.255     9       2026-09-24 21:15:02
2     Workstation Studio    00:11:22:33:44:55   255.255.255.255     9       Mai                 
```

### 4. Aggiungere un nuovo dispositivo
```bash
allod wol add "Workstation Studio" 00:11:22:33:44:55
```

### 5. Rimuovere un dispositivo salvato
```bash
allod wol delete "Workstation Studio"
# oppure specificando l'ID:
allod wol delete 2
```

---

## 🔍 Risoluzione dei Problemi (Troubleshooting)

* **Il PC non si accende al clic**:
  1. Controlla il LED della porta Ethernet sul retro del PC da spento: deve essere acceso fisso o lampeggiare lentamente in arancione/verde. Se è spento, la scheda di rete non riceve alimentazione in standby (verifica le voci *ErP / EuP* e *Deep Sleep* nel BIOS).
  2. Su Windows, disabilita sempre **Attiva avvio rapido**.
  3. Verifica che il cavo di rete sia collegato direttamente alla porta Ethernet della scheda madre (i dongle USB-Ethernet non supportano quasi mai il WoL a computer spento).
  4. Verifica che il MAC address inserito corrisponda alla scheda fisica cablata e non a un adattatore Wi-Fi.
