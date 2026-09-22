// Network module for Allod Panel (NetBird Sovereign Mesh - ES module)

export function toggleNetworkModeUI() {
  const modeSelect = document.getElementById('network-mode-select');
  const mgmtGroup = document.getElementById('network-mgmt-group');
  const serverGroup = document.getElementById('network-managed-server-group');
  const keyGroup = document.getElementById('network-setup-key-group');
  if (!modeSelect) return;

  const mode = modeSelect.value;
  if (mode === 'selfhosted_remote' || mode === 'selfhosted') {
    if (mgmtGroup) mgmtGroup.style.display = 'block';
    if (serverGroup) serverGroup.style.display = 'none';
    if (keyGroup) keyGroup.style.display = 'block';
  } else if (mode === 'selfhosted_managed') {
    if (mgmtGroup) mgmtGroup.style.display = 'none';
    if (serverGroup) serverGroup.style.display = 'block';
    if (keyGroup) keyGroup.style.display = 'none';
  } else {
    // Cloud
    if (mgmtGroup) mgmtGroup.style.display = 'none';
    if (serverGroup) serverGroup.style.display = 'none';
    if (keyGroup) keyGroup.style.display = 'block';
  }
}

export async function openNetworkConfigModal() {
  const modal = document.getElementById('network-config-modal');
  if (!modal) return;

  try {
    const res = await fetch('/api/network/status');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      const data = json.data;
      const modeSelect = document.getElementById('network-mode-select');
      const mgmtInput = document.getElementById('network-mgmt-url-input');
      const keyInput = document.getElementById('network-setup-key-input');

      // Runtime client inspection
      const runtimeBadge = document.getElementById('network-runtime-badge');
      const runtimeDesc = document.getElementById('network-runtime-desc');
      const installBtn = document.getElementById('network-install-native-btn');

      const isNative = data.client_runtime === 'native';
      if (runtimeBadge) {
        if (isNative) {
          runtimeBadge.textContent = '⚡ Nativo Host';
          runtimeBadge.style.background = '#10b981';
          runtimeBadge.style.color = '#0f172a';
        } else {
          runtimeBadge.textContent = '📦 Container Podman';
          runtimeBadge.style.background = '#38bdf8';
          runtimeBadge.style.color = '#0f172a';
        }
      }

      if (runtimeDesc) {
        if (isNative) {
          runtimeDesc.textContent = 'In esecuzione nativa con modulo kernel WireGuard (wt0). Prestazioni massime.';
        } else {
          runtimeDesc.textContent = 'In esecuzione container. Consigliato passare al client nativo per massima stabilità.';
        }
      }

      if (installBtn) {
        installBtn.style.display = isNative ? 'none' : 'inline-block';
      }

      // Mode and inputs
      if (modeSelect && data.mode) {
        modeSelect.value = data.mode;
      }
      if (mgmtInput && data.management_url) {
        mgmtInput.value = data.management_url;
      }
      if (keyInput) {
        keyInput.value = '';
        if (data.has_key) {
          keyInput.placeholder = '●●●●●●●●●● (Già configurata - lascia vuoto per mantenere)';
        } else {
          keyInput.placeholder = 'es. 4A8B7C21-D4E5-...';
        }
      }

      // Managed server status
      if (data.server_managed) {
        const srv = data.server_managed;
        const srvDomain = document.getElementById('network-server-domain-input');
        const srvPort = document.getElementById('network-server-port-input');
        const srvDashPort = document.getElementById('network-server-dashport-input');
        const srvBadge = document.getElementById('network-server-status-badge');
        const srvLink = document.getElementById('network-server-dash-link');

        if (srvDomain && srv.domain) srvDomain.value = srv.domain;
        if (srvPort && srv.port) srvPort.value = srv.port;
        if (srvDashPort && srv.dashboard_port) srvDashPort.value = srv.dashboard_port;

        if (srvBadge) {
          if (srv.running) {
            srvBadge.textContent = '🟢 Attivo';
            srvBadge.style.background = '#10b981';
            srvBadge.style.color = '#0f172a';
          } else {
            srvBadge.textContent = '⚪ Inattivo';
            srvBadge.style.background = '#64748b';
            srvBadge.style.color = '#fff';
          }
        }

        if (srvLink) {
          if (srv.running && srv.dashboard_url) {
            srvLink.href = srv.dashboard_url;
            srvLink.style.display = 'inline-block';
          } else {
            srvLink.style.display = 'none';
          }
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

export async function installNativeNetBird() {
  const btn = document.getElementById('network-install-native-btn');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Installazione nativa in corso...</span>';
  }

  try {
    const res = await fetch('/api/network/install-native', {
      method: 'POST'
    });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') {
        showAlert('NetBird nativo installato con successo sul server! Rete mesh operativa.', 'success');
      }
      openNetworkConfigModal();
      if (typeof refreshData === 'function') refreshData();
    } else {
      if (typeof showAlert === 'function') {
        showAlert('Errore installazione: ' + (data.message || 'Verifica connessione'), 'danger');
      }
    }
  } catch (err) {
    if (typeof showAlert === 'function') showAlert('Errore di connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = '🚀 Passa a Nativo (1-click)';
    }
  }
}

export async function startManagedServer() {
  const domainInput = document.getElementById('network-server-domain-input');
  const portInput = document.getElementById('network-server-port-input');
  const dashPortInput = document.getElementById('network-server-dashport-input');

  const domain = domainInput ? domainInput.value.trim() : '127.0.0.1';
  const port = portInput ? parseInt(portInput.value, 10) : 33073;
  const dashPort = dashPortInput ? parseInt(dashPortInput.value, 10) : 8088;

  if (typeof showAlert === 'function') showAlert('Avvio del server NetBird sovrano locale in corso...', 'info');

  try {
    const res = await fetch('/api/network/server/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain, port, dash_port: dashPort })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') showAlert(data.message || 'Server NetBird avviato!', 'success');
      openNetworkConfigModal();
      if (typeof refreshData === 'function') refreshData();
    } else {
      if (typeof showAlert === 'function') showAlert('Errore: ' + data.message, 'danger');
    }
  } catch (err) {
    if (typeof showAlert === 'function') showAlert('Errore di rete: ' + err.message, 'danger');
  }
}

export async function stopManagedServer() {
  if (typeof showAlert === 'function') showAlert('Arresto del server NetBird sovrano...', 'info');

  try {
    const res = await fetch('/api/network/server/stop', { method: 'POST' });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') showAlert('Server NetBird gestito arrestato', 'success');
      openNetworkConfigModal();
      if (typeof refreshData === 'function') refreshData();
    } else {
      if (typeof showAlert === 'function') showAlert('Errore: ' + data.message, 'danger');
    }
  } catch (err) {
    if (typeof showAlert === 'function') showAlert('Errore di rete: ' + err.message, 'danger');
  }
}

export async function saveNetworkConfig() {
  const modeSelect = document.getElementById('network-mode-select');
  const keyInput = document.getElementById('network-setup-key-input');
  const mgmtInput = document.getElementById('network-mgmt-url-input');
  const domainInput = document.getElementById('network-server-domain-input');
  const portInput = document.getElementById('network-server-port-input');
  const dashPortInput = document.getElementById('network-server-dashport-input');
  const btn = document.getElementById('network-save-btn');

  const mode = modeSelect ? modeSelect.value : 'cloud';
  const setupKey = keyInput ? keyInput.value.trim() : '';
  const managementURL = mgmtInput ? mgmtInput.value.trim() : '';
  const serverDomain = domainInput ? domainInput.value.trim() : '127.0.0.1';
  const serverPort = portInput ? parseInt(portInput.value, 10) : 33073;
  const serverDashPort = dashPortInput ? parseInt(dashPortInput.value, 10) : 8088;

  if ((mode === 'selfhosted_remote' || mode === 'selfhosted') && !managementURL) {
    if (typeof showAlert === 'function') {
      showAlert('Inserisci l\'URL del server di coordinamento NetBird per la modalità Self-Hosted Remota', 'warning');
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
        management_url: managementURL,
        server_domain: serverDomain,
        server_port: serverPort,
        server_dash_port: serverDashPort
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
  const dashRow = document.getElementById('pairing-dash-url-row');
  const dashLink = document.getElementById('pairing-dash-url-link');
  const step2Text = document.getElementById('pairing-step2-text');

  if (modeEl) modeEl.textContent = 'Caricamento...';
  if (meshIpEl) meshIpEl.textContent = '--';
  if (serverUrlEl) serverUrlEl.textContent = 'Caricamento...';

  modal.classList.remove('hidden');

  try {
    const res = await fetch('/api/network/status');
    const data = await res.json();
    if (data.status === 'ok' && data.data) {
      const mode = data.data.mode;
      const isRemote = mode === 'selfhosted_remote' || mode === 'selfhosted';
      const isManaged = mode === 'selfhosted_managed';

      if (modeEl) {
        if (isManaged) modeEl.textContent = 'NetBird Server Gestito (Allod)';
        else if (isRemote) modeEl.textContent = 'NetBird Self-Hosted (Remoto)';
        else modeEl.textContent = 'NetBird Cloud (EU)';
      }

      const realMeshIp = (data.data.mesh_ip && data.data.mesh_ip !== '--') ? data.data.mesh_ip : '--';
      if (meshIpEl) meshIpEl.textContent = realMeshIp;
      if (serverUrlEl) serverUrlEl.textContent = data.data.management_url || 'https://api.netbird.io:443';

      if (dashRow && dashLink) {
        if (isManaged && data.data.server_managed && data.data.server_managed.dashboard_url) {
          dashRow.style.display = 'block';
          dashLink.href = data.data.server_managed.dashboard_url;
          dashLink.textContent = data.data.server_managed.dashboard_url;
        } else {
          dashRow.style.display = 'none';
        }
      }

      if (step2Text) {
        if (isManaged || isRemote) {
          step2Text.textContent = `2. Nelle impostazioni dell'app NetBird inserisci come Management Server URL: ${data.data.management_url || 'l\'URL del tuo server'}.`;
        } else {
          step2Text.textContent = "2. Per NetBird Cloud: lascia l'impostazione predefinita server (api.netbird.io).";
        }
      }

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

export async function restartNetworkModule() {
  if (!confirm('Vuoi riavviare il servizio NetBird? La connessione VPN si interromperà per 2-3 secondi per poi ricollegarsi automaticamente con le nuove impostazioni UPnP.')) {
    return;
  }
  if (typeof showAlert === 'function') showAlert('Riavvio NetBird in corso...', 'info');
  try {
    const res = await fetch('/api/network/restart', { method: 'POST' });
    const data = await res.json();
    if (data.status === 'ok') {
      if (typeof showAlert === 'function') showAlert(data.message, 'success');
    }
  } catch (err) {
    if (typeof showAlert === 'function') showAlert('Comando inviato al server! Riconnessione in corso...', 'info');
  }
  setTimeout(() => {
    if (typeof refreshData === 'function') refreshData();
  }, 4000);
}

// Bind to window object for inline HTML onclick handlers
window.toggleNetworkModeUI = toggleNetworkModeUI;
window.openNetworkConfigModal = openNetworkConfigModal;
window.closeNetworkConfigModal = closeNetworkConfigModal;
window.saveNetworkConfig = saveNetworkConfig;
window.openNetworkPairingModal = openNetworkPairingModal;
window.closeNetworkPairingModal = closeNetworkPairingModal;
window.installNativeNetBird = installNativeNetBird;
window.startManagedServer = startManagedServer;
window.stopManagedServer = stopManagedServer;
window.restartNetworkModule = restartNetworkModule;
