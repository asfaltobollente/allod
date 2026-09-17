// Network module for Allod Panel (ES module)

export async function openNetworkConfigModal() {
  const modal = document.getElementById('network-config-modal');
  if (!modal) return;

  try {
    const res = await fetch('/api/network/status');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      const domainInput = document.getElementById('network-domain-input');
      if (domainInput && json.data.server_url) {
        domainInput.value = json.data.server_url;
      }
    }
  } catch (_) {}

  modal.classList.remove('hidden');
}

export function closeNetworkConfigModal() {
  const modal = document.getElementById('network-config-modal');
  if (modal) modal.classList.add('hidden');
}

export async function saveNetworkConfig() {
  const domainInput = document.getElementById('network-domain-input');
  const tokenInput = document.getElementById('network-token-input');
  const btn = document.getElementById('network-save-btn');

  const domain = domainInput ? domainInput.value.trim() : '';
  const token = tokenInput ? tokenInput.value.trim() : '';

  if (!domain && !token) {
    if (typeof showAlert === 'function') {
      showAlert('Inserisci il dominio o il token del tunnel', 'warning');
    }
    return;
  }

  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Salvataggio...</span>';
  }

  try {
    const res = await fetch('/api/network/configure', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain: domain, tunnel_token: token })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') showAlert(data.message || 'Configurazione tunnel salvata!', 'success');
      closeNetworkConfigModal();
      if (typeof refreshData === 'function') refreshData();
    } else {
      if (typeof showAlert === 'function') showAlert('Errore: ' + data.message, 'danger');
    }
  } catch (err) {
    if (typeof showAlert === 'function') showAlert('Errore di connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      const label = typeof t === 'function' ? t('network_modal_save', 'Save & Restart Services') : 'Save & Restart Services';
      btn.innerHTML = label;
    }
  }
}

export async function openNetworkPairingModal() {
  const modal = document.getElementById('network-pairing-modal');
  if (!modal) return;

  const serverUrlEl = document.getElementById('pairing-server-url');
  const authKeyEl = document.getElementById('pairing-auth-key');

  if (serverUrlEl) serverUrlEl.textContent = 'Caricamento...';
  if (authKeyEl) authKeyEl.textContent = 'Generazione chiave crittografica in corso...';

  modal.classList.remove('hidden');

  try {
    const res = await fetch('/api/network/preauth-key', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });
    const data = await res.json();
    if (data.status === 'ok' && data.data) {
      if (serverUrlEl) serverUrlEl.textContent = data.data.server_url || 'https://tuodominio.it';
      if (authKeyEl) authKeyEl.textContent = data.data.key;
    } else {
      if (authKeyEl) authKeyEl.textContent = 'Errore: ' + (data.message || 'Verifica che il modulo network sia avviato');
    }
  } catch (err) {
    if (authKeyEl) authKeyEl.textContent = 'Errore di connessione: ' + err.message;
  }
}

export function closeNetworkPairingModal() {
  const modal = document.getElementById('network-pairing-modal');
  if (modal) modal.classList.add('hidden');
}

// Bind to window object for inline HTML onclick handlers
window.openNetworkConfigModal = openNetworkConfigModal;
window.closeNetworkConfigModal = closeNetworkConfigModal;
window.saveNetworkConfig = saveNetworkConfig;
window.openNetworkPairingModal = openNetworkPairingModal;
window.closeNetworkPairingModal = closeNetworkPairingModal;
