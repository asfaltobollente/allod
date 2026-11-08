#!/usr/bin/env bash
# ==============================================================================
# ALLOD STANDALONE VAULT — AGGIORNAMENTO CLI ONE-LINER
# Scarica e installa l'ultima versione di allod-vault dal nodo Allod primario
# ==============================================================================
set -euo pipefail

PRIMARY_HOST="{{PRIMARY_HOST}}"

echo -e "\033[0;36m"
echo "  ╔════════════════════════════════════════════════════════════════╗"
echo "  ║        ALLOD STANDALONE VAULT — AGGIORNAMENTO CLI              ║"
echo "  ║      Sovereign Zero-Knowledge Mesh • Aggiornamento P2P         ║"
echo "  ╚════════════════════════════════════════════════════════════════╝"
echo -e "\033[0m"

if [[ $EUID -ne 0 ]]; then
   echo -e "\033[0;31m[ERRORE] Questo script deve essere eseguito come root o con sudo (es. curl ... | sudo bash)\033[0m" 
   exit 1
fi

# Se PRIMARY_HOST contiene ancora il segnaposto, prova a rilevarlo
if [[ -z "${PRIMARY_HOST:-}" || "${PRIMARY_HOST}" == *"PRIMARY_HOST"* ]]; then
  PRIMARY_HOST="${1:-}"
fi

if [[ -z "${PRIMARY_HOST:-}" && -f "/etc/allod-vault.conf" ]]; then
  # shellcheck source=/dev/null
  source "/etc/allod-vault.conf" 2>/dev/null || true
  if [[ -n "${PEER_HOST:-}" ]]; then
    PRIMARY_HOST="${PEER_HOST}:8080"
  fi
fi

if [[ -z "${PRIMARY_HOST:-}" ]]; then
  if command -v netbird >/dev/null 2>&1; then
    DETECTED_IP=$(netbird status 2>/dev/null | grep -oP '100\.\d+\.\d+\.\d+' | head -n 1 || echo "")
    if [[ -n "$DETECTED_IP" ]]; then
      PRIMARY_HOST="${DETECTED_IP}:8080"
    fi
  fi
fi

if [[ -z "${PRIMARY_HOST:-}" ]]; then
  echo -e "\033[0;31m[ERRORE] Indirizzo del nodo Allod primario non specificato o non rilevato.\033[0m"
  echo "Specifica l'IP mesh: curl -fsSL http://<ip-mesh>:8080/api/vault/update-cli.sh | sudo bash -s -- <ip-mesh:8080>"
  exit 1
fi

# Rimuovi protocollo se presente
PRIMARY_HOST=$(echo "$PRIMARY_HOST" | sed 's~http://~~; s~https://~~; s~/.*~~')
if [[ ! "$PRIMARY_HOST" =~ : ]]; then
  PRIMARY_HOST="${PRIMARY_HOST}:8080"
fi

echo -e "[ALLOD] Download CLI aggiornata da \033[1;36mhttp://${PRIMARY_HOST}/api/vault/cli.sh\033[0m..."

TMP_FILE=$(mktemp)
if curl -fsSL --retry 3 --retry-delay 2 "http://${PRIMARY_HOST}/api/vault/cli.sh" -o "${TMP_FILE}"; then
  if head -n 1 "${TMP_FILE}" | grep -q "bash"; then
    chmod +x "${TMP_FILE}"
    mv "${TMP_FILE}" /usr/local/bin/allod-vault
    
    # Salva il peer host per usi futuri da CLI locale
    PEER_ONLY_IP=$(echo "${PRIMARY_HOST}" | cut -d':' -f1 | tr -d '[]')
    mkdir -p /etc
    echo "PEER_HOST=\"${PEER_ONLY_IP}\"" > /etc/allod-vault.conf
    
    echo -e "\033[0;32m[✓] CLI /usr/local/bin/allod-vault aggiornata con successo!\033[0m"
    echo ""
    /usr/local/bin/allod-vault status
  else
    rm -f "${TMP_FILE}"
    echo -e "\033[0;31m[ERRORE] Il file scaricato non sembra essere uno script bash valido.\033[0m"
    exit 1
  fi
else
  rm -f "${TMP_FILE}"
  echo -e "\033[0;31m[ERRORE] Impossibile scaricare la CLI da http://${PRIMARY_HOST}/api/vault/cli.sh\033[0m"
  echo "Verifica che il nodo primario Allod sia in esecuzione e connesso alla rete mesh WireGuard."
  exit 1
fi
