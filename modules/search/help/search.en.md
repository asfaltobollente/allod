# Search Module (SearXNG + Valkey)

SearXNG is a sovereign, privacy-respecting meta-search engine. Integrated into Allod, it acts as a private proxy between your family's devices and major upstream search engines (Google, Bing, DuckDuckGo, Wikipedia, Reddit, GitHub, Qwant, and more), querying them on your behalf without forwarding tracking cookies, device identifiers, or IP addresses.

---

## Key Features

* **Zero Profiling & Tracking**: No search history retention, zero tracking cookies, and no behavioral ad targeting.
* **Multi-Engine Aggregation**: Consolidates results from dozens of sources into a single clean, ad-free interface.
* **Sidecar Valkey Cache**: Runs a dedicated in-memory Valkey container to cache frequent queries, boosting search speed and preventing upstream rate limits.
* **AI & Agent Integration**: Beyond human search queries, SearXNG serves local AI agents and workstations on your LAN for deep research workflows.
* **Optional Tier**: Designed as an `optional` module (`tier: optional`), meaning it can be toggled on and off safely with zero data loss or system impact.

---

## Resource Levels

* **`off`**: Service disabled, containers stopped, 0 MB RAM allocated.
* **`standard` (512 MB RAM allocated, 2 GB recommended storage)**:
  * Primary SearXNG container listening on host port `8888` (`PublishPort=8888:8080`).
  * Secondary Valkey caching container connected over the isolated `allod` container network.
  * Auto-generated configuration at `/mnt/allod-storage/search/config/settings.yml` with secure secret keys and disabled rate limiters for family LAN use.

---

## Setting SearXNG as Default Browser Search Engine

To use SearXNG as your default search provider in Chrome, Firefox, Safari, or Brave:
1. Navigate to `http://<ALLOD-SERVER-IP>:8888` or `http://<NETBIRD-IP>:8888`.
2. Perform any search query.
3. Open your browser's *Search Engine* settings and select or add Allod Search using the URL template:
   `http://<ALLOD-SERVER-IP>:8888/search?q=%s`

---

## Security & Maintenance

* **Ephemeral Data**: Search queries are processed in memory and never written to persistent disk storage; the Valkey cache is entirely volatile.
* **Configuration Dataset**: Persistent configuration files live in `/mnt/allod-storage/search/config` and can be backed up via Allod Ring.
