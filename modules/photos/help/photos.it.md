# Modulo Photos (Immich)

Immich è la soluzione sovereign ad altissime prestazioni integrata in Allod per il backup, la gestione e la visualizzazione di foto e video da smartphone e desktop.

---

## Livelli di Risorse Disponibili

* **`standard` (1.5 GB RAM allocati)**:
  * Backup automatico in background da app mobile iOS e Android.
  * Timeline cronologica fluida, mappa interattiva GPS e album condivisi tra utenti.
  * Ideale per macchine con 8 GB di RAM complessiva.
* **`full` (4.0 GB RAM allocati)**:
  * Aggiunge il riconoscimento biometrico dei volti (Face Recognition) e la ricerca semantica avanzata con intelligenza artificiale (CLIP).
  * Richiede CPU con supporto alle istruzioni AVX2 e almeno 16 GB di RAM di sistema.

---

## Integrazione "Triade Sovereign" (Samba + Immich + Jellyfin)

Allod implementa il modello a **Triade** per garantire al tempo stesso condivisione multimediale pubblica e rigorosa riservatezza privata:

* **Contenuti Pubblici di Rete**:
  * Risiedono nella cartella `\\<SERVER-IP>\public` (accessibili a tutta la LAN senza credenziali o con account condiviso).
  * Vengono indicizzati dal media server **Jellyfin** per lo streaming casalingo di film, serie tv e brani musicali.
* **Spazio Personale e Riservato**:
  * Risiede nello share privato protetto da password `\\<SERVER-IP>\<username>`.
  * Contiene le sottocartelle isolate `photos\`, `media\` e `documents\`.
  * La cartella `photos\` è collegata in modo trasparente tramite *bind mount* alla libreria Immich dell'utente, mantenendo invisibili le foto personali a Jellyfin e agli altri utenti della LAN.

---

## Modello di Sicurezza e Riservatezza dei Dati su Disco

### Perché Immich non memorizza i file cifrati a riposo?
I server multimediali ad alte prestazioni necessitano di elaborare costantemente i flussi video e le foto per:
1. Generare istantaneamente miniature e anteprime WebP ad alta compressione;
2. Eseguire l'analisi dei vettori facciali e semantici con i modelli di machine learning integrati;
3. Consentire la transcodifica video hardware in tempo reale in H.264/HEVC.

Una cifratura trasparente del singolo file sul filesystem impedirebbe l'esecuzione delle pipeline AI e dello streaming video fluido.

### Come Allod garantisce la sicurezza totale delle immagini:
1. **Permessi Reali 0770 (`chmod -R go-rwx`)**:
   * Sia la cartella sorgente `/mnt/allod-storage/photos/upload/library/<username>` sia il mount point SMB hanno permessi restrittivi `0770`.
   * Nessun utente non privilegiato del sistema Linux o demone non autorizzato può entrare o leggere i file sul filesystem host. Solo l'utente `root` e il processo container Podman hanno accesso.
2. **Bind-Mount Selettivo (Minimo Privilegio)**:
   * Su Samba viene esposta **unicamente** la specifica cartella `library/<username>`.
   * I database PostgreSQL, le cache temporanee, i file di configurazione di sistema e le foto di tutti gli altri utenti non vengono mai esposti al protocollo SMB.
3. **Modalità Isolamento Esclusivo App (Opzionale)**:
   * Durante la creazione dell'utente (o in qualunque momento tramite il pulsante *Scollega da SMB* nel pannello), è possibile disattivare l'esposizione di rete delle foto.
   * In questo modo, le immagini rimangono confinate all'interno del database e del filesystem isolato di Immich, consultabili unicamente tramite l'applicazione Immich protetta da password e 2FA (autenticazione a due fattori).

---

## Guida di Configurazione: Immich Storage Label & Template

Per far sì che le foto scattate dallo smartphone finiscano automaticamente nella cartella nominativa associata al relativo utente Samba, segui questi semplici passaggi:

### Passo 1: Imposta lo Storage Label dell'Utente
1. Accedi a Immich su `http://<SERVER-IP>:2283` (o tramite il pulsante **Immich Photos** nel Launchpad di Allod) con un account Amministratore.
2. Clicca sull'icona delle impostazioni in alto a destra ➔ **Administration** ➔ scheda **Users**.
3. Clicca sull'icona di modifica (o sui tre puntini) accanto all'utente desiderato (es. `mario`).
4. Nel campo **Storage Label**, inserisci il nome utente esatto dell'account Allod/Samba (es. `mario`).
5. Clicca su **Save**. A partire da questo momento, tutti i nuovi upload di questo utente verranno indirizzati nella cartella `photos/upload/library/mario/`.

### Passo 2: Attiva e Configura lo Storage Template
1. Sempre in **Administration**, seleziona la scheda **Settings** ➔ **Storage Template**.
2. Attiva la spunta su **Enable storage template**.
3. Configura il pattern di archiviazione desiderato. Il template raccomandato per Allod è:
   ```text
   {{y}}/{{MM}}/{{filename}}
   ```
   *(Questo organizzerà le foto cronologicamente in cartelle per Anno e Mese: es. `2026/09/IMG_1234.jpg`).*
4. Clicca su **Save** in basso a destra.

### Passo 3: Esegui la Migrazione dei File Esistenti (Job)
1. Vai nella scheda **Administration** ➔ **Jobs**.
2. Individua la riga denominata **Storage Template Migration**.
3. Clicca sul pulsante **Run**. Immich riorganizzerà e sposterà automaticamente in background tutti i file precedentemente caricati nella cartella strutturata `library/<username>`.

---

## Creazione Automatica con "Crea Utente Triade" di Allod

Dal Pannello di Controllo Web di Allod puoi automatizzare l'intera configurazione:

1. Fai clic sul pulsante **🛡️ Crea Utente Triade** nella barra superiore o nel Launchpad.
2. Il sistema esegue un controllo preliminare in tempo reale: se **Samba**, **Immich** o **Jellyfin** non sono attivi, la procedura viene bloccata e viene indicata la motivazione.
3. Inserisci il **Nome Utente** (es. `mario`) e la **Password**.
4. Scegli se spuntare l'opzione **Collega libreria foto Immich allo share privato Samba**.
5. Clicca su **Crea Utente & Predisponi Triade**:
   * Viene creato l'account di sistema Linux con shell sicura `/usr/sbin/nologin`.
   * Viene impostata la password Samba e creata la cartella `\\allod\mario`.
   * Vengono create le sottocartelle con permessi restrittivi `0770`.
   * Se abilitato, viene eseguito il bind-mount della cartella Immich su `\\allod\mario\photos`.
   * La lista utenti nel modal ti permette di connettere o disconnettere il bind mount SMB in qualsiasi momento con un solo clic!

