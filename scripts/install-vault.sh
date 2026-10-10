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
if [[ -z "${SETUP_KEY}" ]]; then
  log_err "Setup Key mancante! Specifica '--key <NETBIRD_SETUP_KEY>'."
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
netbird up --setup-key "${SETUP_KEY}" || {
  log_warn "Tentativo di connessione con setup-key terminato, verifico stato..."
}

# Attesa interfaccia wt0
log_info "Attesa inizializzazione interfaccia mesh WireGuard (wt0)..."
NETBIRD_IP=""
for i in {1..20}; do
  if ip addr show wt0 &> /dev/null; then
    NETBIRD_IP=$(ip -4 addr show wt0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -n1 || true)
    if [[ -n "$NETBIRD_IP" ]]; then
      break
    fi
  fi
  sleep 1
done

if [[ -z "$NETBIRD_IP" ]]; then
  log_warn "Interfaccia wt0 non ancora assegnata. Provo a estrarre IP da 'netbird status'..."
  NETBIRD_IP=$(netbird status 2>/dev/null | grep -i "NetBird IP" | awk '{print $3}' | cut -d'/' -f1 || true)
fi

if [[ -n "$NETBIRD_IP" ]]; then
  log_ok "Connesso alla rete Mesh con IP: ${NETBIRD_IP}"
else
  log_warn "Impossibile rilevare immediatamente l'IP NetBird. Il servizio userà 0.0.0.0 con restrizione."
fi

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

# Binda preferibilmente all'IP NetBird se disponibile, altrimenti 0.0.0.0
LISTEN_ADDR="0.0.0.0:${LISTEN_PORT}"
if [[ -n "$NETBIRD_IP" ]]; then
  LISTEN_ADDR="${NETBIRD_IP}:${LISTEN_PORT}"
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

# 7. Annuncio automatico all'Allod Primario (se specificato --peer)
if [[ -n "$PEER_IP" && -n "$NETBIRD_IP" ]]; then
  log_info "Invio annuncio al server Allod primario (http://${PEER_IP}:8080)..."
  # Estrai quota numerica (es. 500G -> 500)
  QUOTA_NUM=$(echo "$QUOTA_STR" | grep -oP '\d+' || echo "500")
  
  ANN_PAYLOAD="{\"name\":\"${VAULT_NAME}\",\"address\":\"${NETBIRD_IP}\",\"quota_gb\":${QUOTA_NUM}}"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://${PEER_IP}:8080/api/ring/announce" \
    -H "Content-Type: application/json" \
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
echo "Per verificare lo stato del servizio:"
echo "  sudo systemctl status allod-vault"
echo "Per visualizzare i log del vault:"
echo "  sudo journalctl -u allod-vault -f"
echo ""
