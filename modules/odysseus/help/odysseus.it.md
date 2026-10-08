# Modulo Odysseus (AI Workspace & Agenti)

Il modulo `odysseus` integra l'AI Workspace open-source per chat con modelli linguistici, orchestrazione di agenti autonomi, Deep Research con web crawling privato e gestione documentale semantica (RAG).

> [!IMPORTANT]
> **Vincolo Hardware & Inferenza (Server Allod):**  
> Per preservare la stabilità, l'efficienza energetica e le basse temperature del nodo Allod (spesso basato su hardware low-power come Intel N100 o Raspberry Pi 5), **l'inferenza locale on-device sulla CPU del server è disattivata**.  
> Odysseus in Allod opera unicamente come **Orchestratore e Workspace applicativo leggero** collegandosi a:
> 1. Una **workstation GPU esterna nella LAN o nella Mesh NetBird** (es. PC con GPU NVIDIA o Mac con Apple Silicon che esegue Ollama o vLLM).
> 2. Oppure a **provider di API Cloud** (OpenAI, Anthropic Claude, OpenRouter, DeepSeek).

---

## Architettura del Modulo in Allod

Il modulo avvia uno stack rootless Podman coordinato da Quadlet:
* **`odysseus`**: Applicazione principale FastAPI/React con gestione agenti, tool MCP, automazione browser headless Chromium e FastEmbed ONNX locale per gli embedding vettoriali leggeri.
* **`odysseus-chroma`**: Vector Database ChromaDB per la memoria semantica a lungo termine e RAG documentale.
* **`odysseus-searxng`**: Motore metasearch privato locale, interrogato dagli agenti per effettuare ricerche web e scraping senza tracciamento.

---

## Porte e Rete

* **Porta Web UI**: `7000` (HTTP).
* **Ambito di Rete (`scope`)**: `lan` (Default per i servizi Allod con UI). Accessibile sia dalla rete locale interna (`192.168.1.X:7000`) sia da remoto via Mesh NetBird WireGuard (`100.x.x.x:7000`), con zero porte aperte sul router.

---

## Configurazione Credenziali ed Endpoint LLM

Le chiavi API e l'indirizzo della workstation GPU esterna sono memorizzati nel file protetto (permessi `0600`):
`/mnt/allod-storage/odysseus/secrets/odysseus.env`  
*(oppure `~/.local/share/allod/storage/odysseus/secrets/odysseus.env` se non è presente un pool Btrfs dedicato)*.

### Opzione A: Collegamento a Workstation GPU Esterna (Consigliata per la Sovranità dei Dati)
Se hai un PC desktop o un server casalingo dotato di scheda grafica dedicata con Ollama:
1. Sulla workstation GPU, avvia Ollama abilitando le connessioni dalla LAN:
   ```bash
   # Su Linux/macOS
   OLLAMA_HOST=0.0.0.0 ollama serve
   ```
2. Nel file `odysseus.env` del server Allod, imposta l'indirizzo IP della workstation:
   ```env
   OLLAMA_BASE_URL=http://192.168.1.100:11434
   ```
   *(Sostituisci `192.168.1.100` con l'IP locale della tua workstation o con il suo IP NetBird `100.x.x.x`)*.

### Opzione B: Utilizzo di API Cloud Esterne
Se non disponi di una GPU locale nella rete, puoi inserire le chiavi API nel file `odysseus.env`:
```env
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
OPENROUTER_API_KEY=sk-or-...
DEEPSEEK_API_KEY=sk-...
```

---

## Sinergie con gli altri Moduli Allod

* **Modulo `cloud` (Nextcloud)**: Odysseus supporta la sincronizzazione di calendari ed eventi via CalDAV. È possibile puntare direttamente alle credenziali CalDAV di Nextcloud.
* **Modulo `shares` (Samba)**: Le cartelle condivise `/shares` sono collegate in sola lettura all'interno del container per consentire agli agenti l'indicizzazione e la consultazione di documenti locali.

---

## Primo Accesso e Password Amministratore

Al primo avvio, Odysseus genera una password casuale per l'utente amministratore (`admin`).  
Puoi visualizzare la password temporanea nei log di sistema:
```bash
journalctl --user -u odysseus.service -n 50 | grep -i password
```
