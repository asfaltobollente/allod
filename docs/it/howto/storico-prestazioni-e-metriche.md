# Guida Pratica: Telemetria & Storico Prestazioni in Allod

Allod include un sottosistema di **monitoraggio temporale e telemetria continua** ad altissima efficienza, progettato per tracciare lo stato di salute, l'utilizzo delle risorse e il carico termico del server nel corso del tempo senza appesantire la macchina con stack complessi (es. Prometheus, Grafana o Netdata).

---

## 📈 Architettura del Monitoraggio

1. **Campionamento in background ad impatto quasi zero**:
   - Un worker leggero integrato in `allod-panel` interroga le statistiche vitali del sistema ogni 60 secondi.
   - Memoria RAM aggiuntiva consumata: **< 2 MB**.
   - Utilizzo CPU del collector: **< 0.01%**.
2. **Archiviazione su SQLite WAL locale**:
   - I campioni vengono salvati nella tabella `metrics_history` di `state.db`.
   - Ciascun record occupa circa 48 byte (timestamp, temperatura CPU in °C, utilizzo CPU %, RAM usata/totale in MB, storage usato/totale in byte).
   - In 30 giorni di storico continuo (~43.200 punti grezzi), lo spazio totale su disco non supera **2.5 MB**.
3. **Aggregazione dinamica server-side (*Time Bucketing*)**:
   - Le query storiche sfruttano la matematica SQLite nativa `(timestamp / ?) * ? AS bucket` e le funzioni di aggregazione `AVG(...)` per servire serie temporali compresse in meno di **1 millisecondo**:
     * **1h**: campionamento raw al minuto (~60 punti).
     * **24h**: aggregazione in bucket da 5 minuti (~288 punti).
     * **7d**: aggregazione in bucket da 30 minuti (~336 punti).
     * **30d**: aggregazione in bucket da 2 ore (~360 punti).
4. **Grafici vettoriali nativi in HTML5 `<canvas>`**:
   - Nessuna libreria JavaScript esterna pesante (niente npm, Chart.js, D3 o CDN di terze parti).
   - Supporto nativo per display HiDPI / Retina con `window.devicePixelRatio`.
   - Linee di andamento vettoriali con sfumature traslucide e ombreggiatura fluida.
   - Tracciamento interattivo con linea guida a mirino (*crosshair*) e tooltip informativo dinamico al passaggio del mouse o al tocco su smartphone.
5. **Auto-Pruning Automatico**:
   - Una routine di pulizia oraria elimina automaticamente i record più vecchi di 30 giorni, garantendo che il database mantenga una dimensione fissa e controllata nel tempo.

---

## 🖥️ Utilizzo dalla Web Dashboard

I grafici di telemetria sono disponibili direttamente nella scheda **Overview** (*Panoramica Nodo*), subito sotto il riquadro della salute in tempo reale:

### 1. Metriche Tracciate
1. **Temperatura CPU (°C)**:
   - Monitora la curva termica del processore (con evidenziazione del valore attuale, media e picco massimo).
   - Consente di rilevare tempestivamente polvere nei dissipatori, problemi alla ventola o carichi prolungati.
2. **Utilizzo CPU (%)**:
   - Rappresenta l'utilizzo percentuale complessivo del processore nel tempo (0-100%).
3. **Utilizzo RAM (MB)**:
   - Traccia la memoria RAM fisica occupata rispetto al totale installato nel sistema.
4. **Allocazione Storage Pool (GB)**:
   - Mostra la crescita del pool di archiviazione dati (Btrfs) nel tempo, utile per stimare la velocità di riempimento dovuta a foto, backup o file personali.

### 2. Selezione dell'Intervallo Temporale
In alto a destra nel riquadro della telemetria, puoi selezionare con un clic il periodo desiderato:
- **`1h`**: Dettaglio minuto per minuto dell'ultima ora.
- **`24h`**: Visione completa della giornata con granularità di 5 minuti.
- **`7d`**: Andamento dell'ultima settimana con granularità di 30 minuti.
- **`30d`**: Tendenza dell'ultimo mese con granularità di 2 ore.

Al passaggio del mouse (o trascinando il dito su touch screen) su qualsiasi punto del grafico compare una linea guida tratteggiata e un riquadro con la data, l'orario esatto del campionamento e il valore misurato.

---

## 🔌 API REST per Integrazioni Esterne

Se desideri estrarre la cronologia delle metriche per automazioni o script esterni:

### Endpoint
```http
GET /api/system/history?range=24h
```

### Parametri di Query
| Parametro | Valori Ammessi | Predefinito | Descrizione |
| :--- | :--- | :--- | :--- |
| `range` | `1h`, `24h`, `7d`, `30d` | `24h` | Intervallo temporale richiesto |

### Esempio di Risposta JSON
```json
{
  "status": "ok",
  "data": {
    "range": "24h",
    "points": [
      {
        "t": 1727218800,
        "cpu_temp": 42.5,
        "cpu_usage": 14.8,
        "ram_used_mb": 2150,
        "ram_total_mb": 15890,
        "storage_used_bytes": 145892300000,
        "storage_total_bytes": 980000000000
      }
    ]
  }
}
```

---

## ⚙️ Manutenzione e Conservazione Dati

L'eliminazione dei campioni obsoleti avviene in background senza necessità di interventi manuali:
- Tutti i record con età superiore a 30 giorni vengono automaticamente eliminati (`DELETE FROM metrics_history WHERE timestamp < ?`).
- Il file SQLite opera in modalità `WAL` (*Write-Ahead Logging*), consentendo scritture al secondo da parte del collector senza bloccare le letture dell'interfaccia web o di altri processi.
