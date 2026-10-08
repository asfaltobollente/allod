# Odysseus Module (AI Workspace & Autonomous Agents)

The `odysseus` module integrates the open-source AI Workspace for LLM chat, autonomous agent workflows, Deep Research with private web crawling, and semantic document retrieval (RAG).

> [!IMPORTANT]
> **Hardware & Inference Constraint (Allod Server Node):**  
> To protect the power efficiency, thermal envelope, and overall responsiveness of the Allod server node (commonly low-power mini PCs or Raspberry Pi 5), **local on-device CPU inference is disabled**.  
> In Allod, Odysseus operates strictly as a **lightweight orchestrator and application workspace**, delegating LLM computation to either:
> 1. An **external GPU workstation on your LAN or NetBird mesh** (e.g. gaming PC with NVIDIA GPU or Apple Silicon Mac running Ollama or vLLM).
> 2. Or **Cloud API providers** (OpenAI, Anthropic Claude, OpenRouter, DeepSeek).

---

## Architecture in Allod

The module deploys a multi-container rootless Podman stack via systemd Quadlets:
* **`odysseus`**: Core FastAPI/React workspace handling agent planning, MCP tools, headless Chromium browser automation for web tasks, and local ONNX FastEmbed for lightweight embeddings.
* **`odysseus-chroma`**: ChromaDB vector store for semantic long-term memory and document RAG.
* **`odysseus-searxng`**: Private local metasearch engine used by autonomous research agents for untracked web search.

---

## Ports & Networking

* **Web UI Port**: `7000` (HTTP).
* **Network Scope (`scope`)**: `lan` (Default for Allod UI services). Accessible both from local LAN (`192.168.1.X:7000`) and remotely over NetBird WireGuard mesh (`100.x.x.x:7000`), with zero router ports opened.

---

## Credential & LLM Endpoint Configuration

API keys and external GPU workstation endpoints are stored in the secure environment file (`0600` permissions):
`/mnt/allod-storage/odysseus/secrets/odysseus.env`  
*(or `~/.local/share/allod/storage/odysseus/secrets/odysseus.env`)*.

### Option A: Connect to External GPU Workstation (Recommended for Data Sovereignty)
If you have a workstation or home server with a dedicated GPU running Ollama:
1. On the GPU machine, ensure Ollama accepts connections across the LAN:
   ```bash
   OLLAMA_HOST=0.0.0.0 ollama serve
   ```
2. In `odysseus.env` on your Allod server, specify the workstation IP:
   ```env
   OLLAMA_BASE_URL=http://192.168.1.100:11434
   ```
   *(Replace with your workstation's LAN IP or NetBird mesh IP)*.

### Option B: External Cloud APIs
If no local GPU machine is available on the LAN, add your preferred API keys in `odysseus.env`:
```env
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
OPENROUTER_API_KEY=sk-or-...
DEEPSEEK_API_KEY=sk-...
```

---

## Integration with Allod Ecosystem

* **`cloud` Module (Nextcloud)**: Odysseus features native CalDAV calendar synchronization. Point it directly to your Nextcloud calendar credentials.
* **`shares` Module (Samba)**: Local `/shares` paths are mounted read-only into Odysseus to allow document indexing and agent search.

---

## Initial Setup & Admin Credentials

On first run, Odysseus prints an auto-generated temporary admin password in the service logs:
```bash
journalctl --user -u odysseus.service -n 50 | grep -i password
```
