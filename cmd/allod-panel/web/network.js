// Network module for Allod Panel (NetBird Sovereign Mesh - ES module)

export function toggleNetworkModeUI() {
  const modeSelect = document.getElementById('network-mode-select');
  const mgmtGroup = document.getElementById('network-mgmt-group');
  if (!modeSelect || !mgmtGroup) return;

  if (modeSelect.value === 'selfhosted') {
    mgmtGroup.style.display = 'block';
  } else {
    mgmtGroup.style.display = 'none';
  }
}

export async function openNetworkConfigModal() {
  const modal = document.getElementById('network-config-modal');
  if (!modal) return;

  try {
    const res = await fetch('/api/network/status');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      const modeSelect = document.getElementById('network-mode-select');
      const mgmtInput = document.getElementById('network-mgmt-url-input');
      const keyInput = document.getElementById('network-setup-key-input');

      if (modeSelect && json.data.mode) {
        modeSelect.value = json.data.mode;
      }
      if (mgmtInput && json.data.management_url) {
        mgmtInput.value = json.data.management_url;
      }
      if (keyInput) {
        keyInput.value = '';
        if (json.data.has_key) {
          keyInput.placeholder = '●●●●●●●●●● (Già configurata - lascia vuoto per mantenere)';
        } else {
          keyInput.placeholder = 'es. 4A8B7C21-D4E5-...';
        }
      }
      toggleNetworkModeUI();
    }
  } catch (_) {}

  modal.classList.remove('hidden');
}

export function closeNetworkConfigModal() {
  const modal = document.getElementById('network-config-modal');
  if (modal) modal.classList.add('hidden');
}

export async function saveNetworkConfig() {
  const modeSelect = document.getElementById('network-mode-select');
  const keyInput = document.getElementById('network-setup-key-input');
  const mgmtInput = document.getElementById('network-mgmt-url-input');
  const btn = document.getElementById('network-save-btn');

  const mode = modeSelect ? modeSelect.value : 'cloud';
  const setupKey = keyInput ? keyInput.value.trim() : '';
  const managementURL = mgmtInput ? mgmtInput.value.trim() : '';

  if (mode === 'selfhosted' && !managementURL) {
    if (typeof showAlert === 'function') {
      showAlert('Inserisci l\'URL del server di coordinamento NetBird per la modalità Self-Hosted', 'warning');
    }
    return;
  }

  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Salvataggio NetBird in corso...</span>';
  }

  try {
    const res = await fetch('/api/network/configure', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        mode: mode,
        setup_key: setupKey,
        management_url: managementURL
      })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') showAlert(data.message || 'Configurazione NetBird salvata!', 'success');
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
      const label = typeof t === 'function' ? t('network_modal_save', 'Salva e Riavvia NetBird') : 'Salva e Riavvia NetBird';
      btn.innerHTML = label;
    }
  }
}

export async function openNetworkPairingModal() {
  const modal = document.getElementById('network-pairing-modal');
  if (!modal) return;

  const modeEl = document.getElementById('pairing-mode-label');
  const meshIpEl = document.getElementById('pairing-mesh-ip');
  const serverUrlEl = document.getElementById('pairing-server-url');

  if (modeEl) modeEl.textContent = 'Caricamento...';
  if (meshIpEl) meshIpEl.textContent = '--';
  if (serverUrlEl) serverUrlEl.textContent = 'Caricamento...';

  modal.classList.remove('hidden');

  try {
    const res = await fetch('/api/network/status');
    const data = await res.json();
    if (data.status === 'ok' && data.data) {
      const isSelf = data.data.mode === 'selfhosted';
      if (modeEl) modeEl.textContent = isSelf ? 'NetBird Self-Hosted' : 'NetBird Cloud (EU)';
      const realMeshIp = (data.data.mesh_ip && data.data.mesh_ip !== '--') ? data.data.mesh_ip : '--';
      if (meshIpEl) meshIpEl.textContent = realMeshIp;
      if (serverUrlEl) serverUrlEl.textContent = data.data.management_url || 'https://api.netbird.io:443';
      const badgeEl = document.getElementById('network-mesh-ip-badge');
      if (badgeEl && realMeshIp !== '--') badgeEl.textContent = realMeshIp;
      if (window.currentMeshIP !== undefined && realMeshIp !== '--') {
        window.currentMeshIP = realMeshIp;
      }
    }
  } catch (err) {
    if (modeEl) modeEl.textContent = 'Errore di connessione';
  }
}

export function closeNetworkPairingModal() {
  const modal = document.getElementById('network-pairing-modal');
  if (modal) modal.classList.add('hidden');
}

// Bind to window object for inline HTML onclick handlers
window.toggleNetworkModeUI = toggleNetworkModeUI;
window.openNetworkConfigModal = openNetworkConfigModal;
window.closeNetworkConfigModal = closeNetworkConfigModal;
window.saveNetworkConfig = saveNetworkConfig;
window.openNetworkPairingModal = openNetworkPairingModal;
window.closeNetworkPairingModal = closeNetworkPairingModal;
