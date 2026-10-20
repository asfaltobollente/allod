# Modulo Search (SearXNG + Valkey)

SearXNG è un motore di meta-ricerca privato, sovrano e rispettoso della privacy. Integrato in Allod, funge da intermediario sicuro tra i dispositivi della tua famiglia e i principali motori di ricerca del web (Google, Bing, DuckDuckGo, Wikipedia, Reddit, GitHub, Qwant e molti altri), inoltrando le query senza mai trasmettere cookie di tracciamento, indirizzi IP o identificatori dei dispositivi.

---

## Caratteristiche Principali

* **Zero Profilazione & Tracciamento**: Nessuna cronologia salvata, nessun cookie di terze parti e nessun tracciamento per annunci pubblicitari personalizzati.
* **Aggregazione Multi-Motore**: Unifica i risultati da decine di fonti contemporaneamente in un'unica interfaccia pulita e priva di annunci sponsorizzati.
* **Sidecar Valkey per Cache Ultra-Rapida**: Include un'istanza dedicata del database in-memory Valkey che memorizza temporaneamente le ricerche recenti, velocizzando la navigazione e riducendo il traffico di rete verso i provider a monte.
* **Integrazione con Agenti e Workspace**: Oltre all'utilizzo manuale da browser per la famiglia, SearXNG può essere utilizzato da agenti IA locali o workstation LAN per compiti di deep research e reperimento informazioni.
* **Tier Opzionale**: Il modulo è contrassegnato come `optional` (`tier: optional`) e può essere attivato o disattivato in qualunque momento con transizioni sicure (`safe`) a impatto zero sui dati.

---

## Livelli di Risorsa

* **`off`**: Servizio disattivato, container fermati, 0 MB RAM allocati.
* **`standard` (512 MB RAM allocati, 2 GB storage consigliato)**:
  * Container primario SearXNG in ascolto sulla porta LAN `8888` (`PublishPort=8888:8080`).
  * Container secondario Valkey per caching in-memory su rete isolata `allod`.
  * Configurazione generata automaticamente in `/mnt/allod-storage/search/config/settings.yml` con chiave segreta casuale e limiter disattivato per uso domestico.

---

## Utilizzo come Motore Predefinito nel Browser

Per impostare SearXNG come motore di ricerca predefinito su Chrome, Firefox, Safari o Brave:
1. Apri `http://<IP-SERVER-ALLOD>:8888` o `http://<IP-NETBIRD>:8888`.
2. Esegui una ricerca qualsiasi.
3. Nelle impostazioni del browser alla sezione *Motori di ricerca*, seleziona o aggiungi la voce Allod Search con il pattern:
   `http://<IP-SERVER-ALLOD>:8888/search?q=%s`

---

## Sicurezza e Manutenzione

* **Dati Effimeri**: Le ricerche non vengono scritte su disco persistente; la cache Valkey è volatile ed effimera.
* **Dataset di Configurazione**: Le sole impostazioni personalizzate del motore risiedono in `/mnt/allod-storage/search/config` e possono essere incluse nel backup Allod.
