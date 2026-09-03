# Guida: Configurazione Storage Template di Immich per Condivisione Samba (LAN)

Questa guida spiega come configurare lo **Storage Template** di Immich per organizzare automaticamente le foto scattate da smartphone in una struttura di cartelle pulita e leggibile:
```text
/library/<nomeutente>/<anno>/<foto.ext>
```
rendendole direttamente navigabili e sfogliabili tramite la condivisione di rete Samba (`\\allod\shares\photos`).

---

## 🎯 Perché attivare lo Storage Template?

1. **Esplorazione Pulita da Windows / Mac**:
   * Senza Storage Template, Immich conserva le foto nella cartella grezza di upload con ID alfanumerici casuali.
   * Con lo Storage Template attivo, Immich sposta i file nella cartella `/library` organizzandoli automaticamente per utente e anno.
2. **Zero File di Servizio**:
   * Il puntamento di Allod (`\\allod\shares\photos`) punta a `/library`.
   * Non vedrai le migliaia di miniature generate dall'AI (`thumbs/`) o i file transcodificati (`encoded-video/`).
3. **Indipendenza e Zero Lock-in**:
   * I tuoi file originali mantengono nomi riconoscibili e una cronologia perfetta, apribile con qualsiasi visualizzatore di foto anche senza Immich.

---

## ⚙️ Procedura di Attivazione (Passo-Passo)

### 1. Accedi alla Web UI di Immich
Apri il browser sul tuo server locale:
👉 **`http://<SERVER-IP>:2283`**

### 2. Vai nelle Impostazioni Amministrazione
1. Clicca sull'icona ingranaggio **Administration** (in alto a destra o barra laterale).
2. Nel menu a sinistra seleziona: **Settings** ➔ **Storage Template**.

### 3. Abilita e Configura il Modello
1. Attiva l'interruttore: **`Enabled`** (spunta attiva).
2. Nel campo **TEMPLATE**, inserisci il formato compatto desiderato:
   ```text
   {{user.name}}/{{y}}/{{filename}}
   ```
   *(Esempio risultato: `library/mario/2026/vacanze_01.png`)*
3. Clicca sul pulsante **Save** in basso a destra.

### 4. Avvia la Migrazione dei File Esistenti (Job)
1. Dal menu di sinistra, clicca su **Administration** ➔ **Jobs**.
2. Cerca la riga **Storage Template Migration**.
3. Clicca sul pulsante **▶ Run**.
4. Immich analizzerà in pochi secondi tutti gli asset e li sposterà istantaneamente nella nuova alberatura ordinata `/library/<user>/<year>/`.

---

## 🖥️ Come Accedere da Windows

1. Premi **`Win + R`** e digita:
   ```text
   \\<SERVER-IP>\shares\photos
   ```
2. Troverai le cartelle pulite:
   ```text
   \\<SERVER-IP>\shares\photos\
      └── Mario/
           └── 2026/
                ├── IMG_001.png
                └── DSC_0042.jpg
   ```
3. Qualsiasi nuova foto scattata e sincronizzata con l'app mobile Immich verrà automaticamente classificata nella rispettiva cartella dell'anno in corso!
