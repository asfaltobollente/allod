# Modulo Photos (Immich)

Immich è il server di backup e visualizzazione di foto e video ad alte prestazioni di Allod.

## Livelli Disponibili

* **`standard` (1.5 GB RAM)**:
  * Backup automatico da app iOS e Android.
  * Timeline cronologica, mappa GPS e album condivisi.
  * Consigliato per macchine con 8 GB di RAM.
* **`full` (4.0 GB RAM)**:
  * Aggiunge riconoscimento dei volti e ricerca semantica AI.
  * Richiede 16 GB di RAM di sistema e CPU AVX2.

## Integrazione con Shares (Samba LAN)

Allod permette di collegare la libreria fotografica direttamente a Windows Explorer / Mac Finder su:
`\\<SERVER-IP>\shares\photos`

Per visualizzare le foto in modo pulito ordinate per `/utente/anno/foto.ext`:
1. Apri Immich su `http://<SERVER-IP>:2283`.
2. Vai su **Administration ➔ Settings ➔ Storage Template**.
3. Abilita il template e imposta: `{{user.name}}/{{y}}/{{filename}}`.
4. In **Administration ➔ Jobs**, clicca su **Run** su **Storage Template Migration**.
