#!/usr/bin/env bash
# ==============================================================================
# Allod Standalone Vault Installer (allod-vault)
# https://github.com/asfaltobollente/allod
#
# Configura in automatico un server/nodo remoto indipendente per ospitare
# i backup cifrati (append-only) di Allod tramite NetBird Mesh e rest-server.
# ==============================================================================
set -euo pipefail

# Colori per terminale
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${CYAN}[ALLOD-VAULT]${NC} $1"; }
log_ok()   { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[!]${NC} $1"; }
log_err()  { echo -e "${RED}[✗]${NC} $1"; }

# Valori predefiniti
VAULT_NAME="allod-vault-remote"
SETUP_KEY=""
QUOTA_STR="500G"
PEER_IP=""
ANNOUNCE_TOKEN=""
LISTEN_PORT="8000"
STORAGE_DIR="/var/lib/allod-vault"
REST_SERVER_VERSION="0.12.1"

# Parsing argomenti
while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)
      VAULT_NAME="$2"
      shift 2
      ;;
    --key|--setup-key)
      SETUP_KEY="$2"
      shift 2
      ;;
    --quota)
      QUOTA_STR="$2"
      shift 2
      ;;
    --peer)
      PEER_IP="$2"
      shift 2
      ;;
    --token)
      ANNOUNCE_TOKEN="$2"
      shift 2
      ;;
    --port)
      LISTEN_PORT="$2"
      shift 2
      ;;
    --dir)
      STORAGE_DIR="$2"
      shift 2
      ;;
    -h|--help)
      echo "Uso: sudo $0 --key <NETBIRD_SETUP_KEY> [opzioni]"
      echo ""
      echo "Opzioni:"
      echo "  --name <nome>      Nome identificativo del Vault (default: allod-vault-remote)"
      echo "  --key <chiave>     Setup Key di NetBird (obbligatoria)"
      echo "  --quota <dim>      Quota disco dichiarata es. 500G, 1000G (default: 500G)"
      echo "  --peer <ip_mesh>   IP NetBird dell'Allod primario (opzionale, per auto-annuncio)"
      echo "  --token <token>    Token monouso di annuncio al Ring primario"
      echo "  --port <porta>     Porta di ascolto per rest-server (default: 8000)"
      echo "  --dir <path>       Percorso locale per i dati cifrati (default: /var/lib/allod-vault)"
      exit 0
      ;;
    *)
      log_warn "Argomento sconosciuto ignorato: $1"
      shift
      ;;
  esac
done

# Verifica privilegi di root
if [[ $EUID -ne 0 ]]; then
   log_err "Questo script deve essere eseguito come root (usa 'sudo bash')."
   exit 1
fi

echo -e "${BLUE}"
echo "  ╔════════════════════════════════════════════════════════════════╗"
echo "  ║         ALLOD STANDALONE VAULT — DEPLOYMENT AUTOMATICO         ║"
echo "  ║       Zero-Knowledge • Append-Only Storage • NetBird Mesh      ║"
echo "  ╚════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

log_info "Configurazione Vault: Nome='${VAULT_NAME}', Quota='${QUOTA_STR}', Storage='${STORAGE_DIR}'"

# Verifica Setup Key
if [[ -z "${SETUP_KEY}" ]] && ! (command -v netbird &>/dev/null && netbird status 2>/dev/null | grep -qi "connected"); then
  log_err "Setup Key mancante e nodo non connesso a NetBird! Specifica '--key <NETBIRD_SETUP_KEY>'."
  exit 1
fi

# 1. Rilevamento Architettura CPU
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  R_ARCH="amd64" ;;
  aarch64) R_ARCH="arm64" ;;
  armv7l)  R_ARCH="armv7" ;;
  *)       log_err "Architettura non supportata: $ARCH"; exit 1 ;;
esac
log_ok "Architettura rilevata: $ARCH ($R_ARCH)"

# 2. Verifica / Installazione NetBird
if ! command -v netbird &> /dev/null; then
  log_info "NetBird non trovato. Installazione client ufficiale in corso..."
  curl -fsSL https://pkgs.netbird.io/install.sh | sh
  log_ok "NetBird installato con successo."
else
  log_ok "NetBird è già installato."
fi

# 3. Connessione a NetBird Mesh con Setup Key
log_info "Connessione alla rete mesh sovrana..."
if [[ -n "${SETUP_KEY}" ]]; then
  log_info "Connessione alla rete mesh sovrana..."
  netbird up --setup-key "${SETUP_KEY}" || {
    log_warn "Tentativo di connessione con setup-key terminato, verifico stato..."
  }
else
  log_info "Nessuna setup-key fornita: utilizzo sessione NetBird già attiva sul nodo..."
fi

# Attesa interfaccia wt0
log_info "Attesa inizializzazione interfaccia mesh WireGuard (wt0)..."
NETBIRD_IP=""
NETBIRD_IPV6=""
for i in {1..20}; do
  if ip addr show wt0 &> /dev/null; then
    NETBIRD_IP=$(ip -4 addr show wt0 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -n1 || true)
    NETBIRD_IPV6=$(ip -6 addr show wt0 scope global 2>/dev/null | grep -oP '(?<=inet6\s)[a-f0-9:]+' | head -n1 || true)
    if [[ -n "$NETBIRD_IP" || -n "$NETBIRD_IPV6" ]]; then
      break
    fi
  fi
  sleep 1
done

if [[ -z "$NETBIRD_IP" && -z "$NETBIRD_IPV6" ]]; then
  log_err "ERRORE: Impossibile rilevare l'indirizzo IP (IPv4 o IPv6) di NetBird sull'interfaccia wt0."
  log_err "Per ragioni di sicurezza, rest-server non verrà avviato su 0.0.0.0 senza autenticazione."
  log_err "Verifica che il nodo sia connesso alla mesh con: 'netbird status --detail' e riesegui l'installazione."
  exit 1
fi

PRIMARY_NODE_IP="${NETBIRD_IP:-$NETBIRD_IPV6}"
log_ok "Connesso alla rete Mesh (IPv4: ${NETBIRD_IP:-'N/D'}, IPv6: ${NETBIRD_IPV6:-'N/D'})"

# 4. Installazione di rest-server
log_info "Installazione demone rest-server (v${REST_SERVER_VERSION})..."
BIN_TARGET="/usr/local/bin/rest-server"
if [[ ! -f "$BIN_TARGET" ]]; then
  TMP_DIR=$(mktemp -d)
  DOWNLOAD_URL="https://github.com/restic/rest-server/releases/download/v${REST_SERVER_VERSION}/rest-server_${REST_SERVER_VERSION}_linux_${R_ARCH}.tar.gz"
  log_info "Download da $DOWNLOAD_URL ..."
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/rs.tar.gz"
  tar -xzf "$TMP_DIR/rs.tar.gz" -C "$TMP_DIR"
  
  # Trova il binario estratto
  EXTRACTED_BIN=$(find "$TMP_DIR" -type f -name "rest-server" | head -n1)
  if [[ -f "$EXTRACTED_BIN" ]]; then
    mv "$EXTRACTED_BIN" "$BIN_TARGET"
    chmod +x "$BIN_TARGET"
    log_ok "rest-server installato in $BIN_TARGET"
  else
    log_err "Binario rest-server non trovato nell'archivio scaricato!"
    rm -rf "$TMP_DIR"
    exit 1
  fi
  rm -rf "$TMP_DIR"
else
  log_ok "rest-server è già presente in $BIN_TARGET"
fi

# 5. Creazione directory storage
mkdir -p "${STORAGE_DIR}"
chmod 750 "${STORAGE_DIR}"
log_ok "Directory storage pronta su: ${STORAGE_DIR}"

# 6. Creazione Servizio Systemd
SERVICE_FILE="/etc/systemd/system/allod-vault.service"
log_info "Configurazione servizio systemd (${SERVICE_FILE})..."

# Binda rigorosamente all'IP Mesh NetBird
if [[ -n "$NETBIRD_IP" ]]; then
  LISTEN_ADDR="${NETBIRD_IP}:${LISTEN_PORT}"
else
  LISTEN_ADDR="[${NETBIRD_IPV6}]:${LISTEN_PORT}"
fi

cat << EOF > "$SERVICE_FILE"
[Unit]
Description=Allod Standalone Vault (Append-Only Restic Receiver)
After=network-online.target netbird.service
Wants=network-online.target netbird.service

[Service]
Type=simple
ExecStart=${BIN_TARGET} --append-only --no-auth --listen ${LISTEN_ADDR} --path ${STORAGE_DIR}
Restart=always
RestartSec=5
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable allod-vault.service
systemctl restart allod-vault.service
log_ok "Servizio allod-vault.service avviato e abilitato all'avvio del sistema."

# 6b. Creazione utility CLI locale allod-vault
cat << 'EOF' > /usr/local/bin/allod-vault
#!/bin/bash
# ==============================================================================
# ALLOD STANDALONE VAULT — DIAGNOSTICA & GESTIONE LOCALE
# Sovereign Zero-Knowledge Storage Node CLI
# ==============================================================================
set -euo pipefail

STORAGE_DIR="/var/lib/allod-vault"
CONF_FILE="/etc/allod-vault.conf"
VERSION="1.2.0"

PEER_HOST=""
if [[ -f "$CONF_FILE" ]]; then
  # shellcheck source=/dev/null
  source "$CONF_FILE" 2>/dev/null || true
fi

case "${1:-status}" in
  status)
    echo -e "\033[0;36m====================================================\033[0m"
    echo -e "\033[1;37m   ALLOD STANDALONE VAULT — DIAGNOSTICA STORAGE     \033[0m"
    echo -e "\033[0;36m====================================================\033[0m"
    echo -e "Versione CLI:            \033[1;33mv${VERSION}\033[0m"
    
    if systemctl is-active --quiet allod-vault; then
      echo -e "Servizio Vault:          \033[0;32m● ATTIVO (rest-server in esecuzione)\033[0m"
    else
      echo -e "Servizio Vault:          \033[0;31m○ INATTIVO (systemctl start allod-vault)\033[0m"
    fi
    
    if command -v netbird >/dev/null 2>&1; then
      NB_STATUS="ATTIVO"
      if netbird status 2>/dev/null | grep -iq "management: connected"; then
        NB_STATUS="CONNESSO"
      fi
      NB_IP=$(ip -4 addr show wt0 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' || echo "")
      echo -e "Mesh WireGuard:          \033[0;32m● ${NB_STATUS}\033[0m (${NB_IP:-wt0})"
    fi

    echo "Directory Repository:    ${STORAGE_DIR}"
    
    USED_BYTES=$(du -sh "${STORAGE_DIR}" 2>/dev/null | awk '{print $1}' || echo "0 B")
    echo "Spazio Occupato Chunk:   ${USED_BYTES}"
    
    FREE_SPACE=$(df -h "${STORAGE_DIR}" 2>/dev/null | awk 'NR==2 {print $4 " liberi di " $2}' || echo "N/D")
    echo "Spazio Disco Disponibile: ${FREE_SPACE}"

    SNAPSHOT_COUNT=0
    if [[ -d "${STORAGE_DIR}/snapshots" ]]; then
      SNAPSHOT_COUNT=$(find "${STORAGE_DIR}/snapshots" -type f 2>/dev/null | wc -l)
    fi
    echo -e "Snapshot Consolidati:    \033[1;33m${SNAPSHOT_COUNT}\033[0m"

    INDEX_COUNT=0
    if [[ -d "${STORAGE_DIR}/index" ]]; then
      INDEX_COUNT=$(find "${STORAGE_DIR}/index" -type f 2>/dev/null | wc -l)
    fi
    echo "Indici Crittografici:    ${INDEX_COUNT}"

    DATA_CHUNKS=0
    if [[ -d "${STORAGE_DIR}/data" ]]; then
      DATA_CHUNKS=$(find "${STORAGE_DIR}/data" -type f 2>/dev/null | wc -l)
    fi
    echo "Chunk Dati Cifrati:      ${DATA_CHUNKS}"

    if [[ -n "${PEER_HOST:-}" ]]; then
      echo -e "Nodo Primario Config:    ${PEER_HOST}"
    fi

    echo -e "\033[0;36m----------------------------------------------------\033[0m"
    echo "Ultime richieste HTTP ricevute dai nodi Mesh:"
    journalctl -u allod-vault -n 6 --no-pager 2>/dev/null | tail -n 6
    echo -e "\033[0;36m====================================================\033[0m"
    ;;
    
  logs)
    echo "Streaming dei log di allod-vault (premi Ctrl+C per uscire)..."
    journalctl -u allod-vault -f
    ;;

  restart)
    echo "Riavvio del servizio allod-vault in corso..."
    systemctl restart allod-vault
    echo -e "\033[0;32m✓ Servizio allod-vault riavviato.\033[0m"
    ;;

  test-mesh)
    TARGET="${2:-${PEER_HOST:-}}"
    if [[ -z "$TARGET" ]]; then
      if command -v netbird >/dev/null 2>&1; then
        TARGET=$(netbird status 2>/dev/null | grep -oP '100\.\d+\.\d+\.\d+' | head -n 1 || echo "")
      fi
    fi
    TARGET="${TARGET:-100.64.0.1}"
    TARGET_HOST=$(echo "$TARGET" | cut -d':' -f1 | tr -d '[]')
    echo "Verifica connettività Mesh verso il nodo Allod primario (http://${TARGET_HOST}:8080/api/health)..."
    if curl -fs -m 5 "http://${TARGET_HOST}:8080/api/health" >/dev/null 2>&1; then
      echo -e "\033[0;32m✓ Connessione Mesh WireGuard funzionante e nodo primario raggiungibile!\033[0m"
    else
      echo -e "\033[0;31m✗ Impossibile raggiungere http://${TARGET_HOST}:8080/api/health\033[0m"
      echo "Verifica con 'netbird status' lo stato dei peer nella rete mesh."
    fi
    ;;

  update)
    PEER_TARGET="${2:-${PEER_HOST:-}}"
    if [[ -z "$PEER_TARGET" ]]; then
      if command -v netbird >/dev/null 2>&1; then
        PEER_TARGET=$(netbird status 2>/dev/null | grep -oP '100\.\d+\.\d+\.\d+' | head -n 1 || echo "")
      fi
    fi
    PEER_TARGET=$(echo "${PEER_TARGET:-}" | cut -d':' -f1 | tr -d '[]')
    
    if [[ -z "$PEER_TARGET" ]]; then
      echo -e "\033[0;31mErrore: Nessun IP del nodo Allod primario rilevato o specificato.\033[0m"
      echo "Uso: sudo allod-vault update <ip-peer-mesh>"
      exit 1
    fi

    echo -e "Scaricamento aggiornamento CLI da \033[1;36mhttp://${PEER_TARGET}:8080/api/vault/cli.sh\033[0m..."
    TMP_CLI=$(mktemp)
    if curl -fsSL --retry 3 --retry-delay 1 "http://${PEER_TARGET}:8080/api/vault/cli.sh" -o "$TMP_CLI"; then
      if head -n 1 "$TMP_CLI" | grep -q "bash"; then
        chmod +x "$TMP_CLI"
        mv "$TMP_CLI" /usr/local/bin/allod-vault
        mkdir -p /etc
        echo "PEER_HOST=\"${PEER_TARGET}\"" > "$CONF_FILE"
        echo -e "\033[0;32m✓ CLI allod-vault aggiornata con successo!\033[0m"
        /usr/local/bin/allod-vault status
      else
        rm -f "$TMP_CLI"
        echo -e "\033[0;31mErrore: il file scaricato non è uno script valido.\033[0m"
        exit 1
      fi
    else
      rm -f "$TMP_CLI"
      echo -e "\033[0;31mErrore durante il download da http://${PEER_TARGET}:8080/api/vault/cli.sh\033[0m"
      exit 1
    fi
    ;;

  version|--version|-v)
    echo "allod-vault version ${VERSION}"
    ;;

  help|--help|-h)
    echo "Uso: allod-vault [comando]"
    echo ""
    echo "Comandi disponibili:"
    echo "  status            Mostra spazio occupato, snapshot ricevuti e diagnostica (default)"
    echo "  logs              Visualizza in streaming i log in tempo reale di rest-server"
    echo "  test-mesh         Verifica la connettività HTTP/WireGuard con il nodo Allod primario"
    echo "  update [ip-peer]  Aggiorna la CLI direttamente dal nodo Allod primario"
    echo "  restart           Riavvia il servizio systemd allod-vault"
    echo "  version           Visualizza la versione installata"
    echo "  help              Mostra questo messaggio di aiuto"
    ;;

  *)
    echo "Comando non riconosciuto: $1"
    echo "Esegui 'allod-vault help' per visualizzare i comandi disponibili."
    exit 1
    ;;
esac
EOF
chmod 755 /usr/local/bin/allod-vault
log_ok "Utility CLI allod-vault installata in /usr/local/bin/allod-vault"

# Salvataggio configurazione peer se specificato
if [[ -n "$PEER_IP" ]]; then
  mkdir -p /etc
  PEER_CLEAN=$(echo "$PEER_IP" | cut -d':' -f1 | tr -d '[]')
  echo "PEER_HOST=\"${PEER_CLEAN}\"" > /etc/allod-vault.conf
fi

# 7. Annuncio automatico all'Allod Primario (se specificato --peer)
if [[ -n "$PEER_IP" && -n "$PRIMARY_NODE_IP" ]]; then
  PEER_URL_HOST="$PEER_IP"
  if [[ "$PEER_IP" =~ : ]]; then
    PEER_URL_HOST="[${PEER_IP}]"
  fi
  log_info "Invio annuncio al server Allod primario (http://${PEER_URL_HOST}:8080)..."
  QUOTA_NUM=$(echo "$QUOTA_STR" | grep -oP '\d+' || echo "500")
  
  ANN_PAYLOAD="{\"name\":\"${VAULT_NAME}\",\"address\":\"${PRIMARY_NODE_IP}\",\"quota_gb\":${QUOTA_NUM},\"token\":\"${ANNOUNCE_TOKEN}\"}"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://${PEER_URL_HOST}:8080/api/ring/announce" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${ANNOUNCE_TOKEN}" \
    -d "$ANN_PAYLOAD" || echo "000")
  
  if [[ "$HTTP_CODE" == "200" ]]; then
    log_ok "Annuncio registrato con successo nel Ring dell'Allod primario!"
  else
    log_warn "Annuncio non confermato (HTTP $HTTP_CODE). Puoi aggiungere manualmente il nodo con:"
    echo "  allod ring add ${VAULT_NAME} ${NETBIRD_IP} ${QUOTA_NUM}"
  fi
fi

# 8. Riepilogo Finale
echo ""
echo -e "${GREEN}════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ✓ ALLOD STANDALONE VAULT CONFIGURATO CON SUCCESSO!${NC}"
echo -e "${GREEN}════════════════════════════════════════════════════════════════${NC}"
echo -e "  • Nome Vault:          ${CYAN}${VAULT_NAME}${NC}"
echo -e "  • Indirizzo Mesh:      ${CYAN}${NETBIRD_IP:-'In attesa di assegnazione'}:${LISTEN_PORT}${NC}"
echo -e "  • Quota Riservata:     ${CYAN}${QUOTA_STR}${NC}"
echo -e "  • Percorso Dati:       ${CYAN}${STORAGE_DIR}${NC}"
echo -e "  • Modalità Protezione: ${CYAN}Append-Only Immutabile (Anti-Ransomware)${NC}"
echo ""
echo "Comandi utili per la gestione del Vault:"
echo "  allod-vault status    # Mostra spazio occupato, snapshot ricevuti e attività"
echo "  allod-vault logs      # Segui in tempo reale il traffico HTTP in arrivo"
echo "  sudo systemctl status allod-vault"
echo ""
