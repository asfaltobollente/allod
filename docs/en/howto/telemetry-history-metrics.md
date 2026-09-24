# Practical Guide: Telemetry & Historical Metrics in Allod

Allod includes an ultra-efficient, continuous **time-series telemetry and performance monitoring** engine designed to track server health, hardware utilization, and thermal trends over time without the heavy resource footprint of monitoring stacks like Prometheus, Grafana, or Netdata.

---

## 📈 Monitoring Architecture

1. **Near-Zero Footprint Background Sampling**:
   - A lightweight collector goroutine inside `allod-panel` samples hardware vitals once every 60 seconds.
   - Resident RAM footprint: **< 2 MB**.
   - CPU utilization during collection: **< 0.01%**.
2. **Local SQLite WAL Time-Series Storage**:
   - Telemetry records are appended to the `metrics_history` table in `state.db`.
   - Each data point occupies ~48 bytes (timestamp, CPU temperature in °C, CPU utilization %, RAM used/total in MB, storage used/total in bytes).
   - A full 30-day continuous history (~43,200 raw data points) consumes less than **2.5 MB** of disk space.
3. **Server-Side Dynamic Time-Bucketing**:
   - Queries utilize native SQLite arithmetic `(timestamp / ?) * ? AS bucket` and `AVG(...)` aggregation functions to return downsampled time series in under **1 millisecond**:
     * **1h**: 1-minute raw intervals (~60 points).
     * **24h**: 5-minute bucket aggregation (~288 points).
     * **7d**: 30-minute bucket aggregation (~336 points).
     * **30d**: 2-hour bucket aggregation (~360 points).
4. **Native HTML5 `<canvas>` Vector Visualizations**:
   - Zero external JavaScript dependencies (no npm packages, Chart.js, D3, or third-party CDN scripts).
   - Crisp rendering on HiDPI / Retina displays via `window.devicePixelRatio`.
   - Glowing gradient fills with smooth trendlines.
   - Interactive mouse crosshair and dynamic floating tooltip on desktop hover and mobile touch.
5. **Automated Data Pruning**:
   - An hourly maintenance job purges records older than 30 days, keeping the database capped at ~2.5 MB forever.

---

## 🖥️ Web Dashboard Usage

Historical charts are embedded directly in the **Overview** (*Node Overview*) tab, positioned immediately below the real-time server vitals:

### 1. Monitored Metrics
1. **CPU Temperature (°C)**:
   - Tracks processor thermal curves over time with real-time current, average, and peak readings.
   - Helps identify dust buildup, fan degradation, or sustained high-load workloads.
2. **CPU Utilization (%)**:
   - Displays percentage load (0–100%) across all processor cores.
3. **RAM Utilization (MB)**:
   - Tracks physical memory usage against total installed system memory.
4. **Storage Pool Allocation (GB)**:
   - Graphs storage pool growth over time, making it easy to project storage capacity needs as photos, backups, and shares expand.

### 2. Time Range Switcher
Switch between time horizons with one click in the card header:
- **`1h`**: Minute-by-minute detail for immediate diagnostics.
- **`24h`**: Daily overview aggregated into 5-minute intervals.
- **`7d`**: Weekly trend analysis aggregated into 30-minute intervals.
- **`30d`**: Monthly trend analysis aggregated into 2-hour intervals.

Hovering over any point reveals exact date, time, and recorded value.

---

## 🔌 REST API Integration

Extract historical telemetry data for custom scripts or home automation tools:

### Endpoint
```http
GET /api/system/history?range=24h
```

### Query Parameters
| Parameter | Permitted Values | Default | Description |
| :--- | :--- | :--- | :--- |
| `range` | `1h`, `24h`, `7d`, `30d` | `24h` | Desired time range |

### JSON Response Example
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

## ⚙️ Maintenance & Data Retention

- Records older than 30 days are automatically removed (`DELETE FROM metrics_history WHERE timestamp < ?`).
- SQLite runs in `WAL` mode, enabling non-blocking writes by the background collector without interrupting web dashboard reads.
