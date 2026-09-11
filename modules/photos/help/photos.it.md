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

## Integrazione con Shares (Samba LAN) e Privacy Utenti

Allod permette di collegare le foto caricate da Immich direttamente alle cartelle SMB protette di ciascun utente, mantenendo totale separazione e privacy:

* **Film e musica pubblica**: risiedono in `\\<SERVER-IP>\public` (accessibili da tutti e indicizzati da Jellyfin).
* **Foto personali**: risiedono nella cartella privata `\\<SERVER-IP>\<username>\photos`, accessibile unicamente da quell'utente con la propria password Samba (e invisibili a Jellyfin).

### Come associare gli account Immich alle cartelle private SMB:

1. **Configura lo Storage Label in Immich**:
   * Apri Immich su `http://<SERVER-IP>:2283` con account amministratore.
   * Vai su **Administration ➔ Users**, clicca sui tre puntini accanto all'utente e seleziona **Edit**.
   * Nel campo **Storage Label**, inserisci il nome utente esatto dell'account SMB (es. `mario`, `chiara`).
2. **Attiva lo Storage Template**:
   * In **Administration ➔ Settings ➔ Storage Template**, spunta **Enable storage template**.
   * Imposta la struttura desiderata (es. `{{y}}/{{MM}}/{{filename}}`).
   * In **Administration ➔ Jobs**, clicca su **Run** sulla voce **Storage Template Migration** (per riordinare le foto già caricate).
3. **Abilita l'utente in Allod**:
   * Nel pannello di Allod, nella sezione **Shares**, imposta la password Samba per l'utente.
   * Allod creerà la condivisione privata `\\allod\<username>` e collegherà istantaneamente la cartella `/library/<username>` di Immich dentro `\\allod\<username>\photos`.
