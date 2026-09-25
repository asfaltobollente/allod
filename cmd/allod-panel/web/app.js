// Global Fetch Interceptor: Automatically redirect to /login on 401 Unauthorized
const _rawFetch = window.fetch;
window.fetch = async function(...args) {
  const res = await _rawFetch.apply(this, args);
  if (res.status === 401) {
    const url = typeof args[0] === 'string' ? args[0] : (args[0] && args[0].url) || '';
    if (url.includes('/api/') && !url.includes('/api/auth/status')) {
      window.location.href = '/login';
    }
  }
  return res;
};

async function logoutAdmin() {
  const msg = typeof t === 'function' ? t('confirm_logout', 'Sei sicuro di voler uscire dal pannello amministratore?') : 'Sei sicuro di voler uscire dal pannello amministratore?';
  if (!confirm(msg)) {
    return;
  }
  try {
    await _rawFetch('/api/auth/logout', { method: 'POST' });
  } catch (e) {
    console.error('Logout error:', e);
  }
  window.location.href = '/login';
}

// Allod Panel SPA Logic
let currentStatus = null;
let currentModules = null;
let currentRing = null;
let unlockedModules = new Set();
let startingModules = new Map(); // modID -> timestamp
let photosSharesState = { enabled: false, mounted: false, shares_active: false };

document.addEventListener('DOMContentLoaded', () => {
  initTheme();
  setupTabs();
  if (typeof setLanguage === 'function') {
    setLanguage(currentLang);
  }
  switchToTab('launchpad');
  refreshData();
  // Live vitals polling every 10 seconds for real-time temperature and CPU stats
  setInterval(() => {
    if (typeof fetchLiveVitals === 'function') {
      fetchLiveVitals();
    }
  }, 10000);

  // Periodic metrics history update every 60s if Overview tab is active
  setInterval(() => {
    const activeTab = document.querySelector('.nav-item.active');
    if (activeTab && activeTab.dataset.tab === 'overview') {
      if (typeof loadMetricsHistory === 'function') {
        loadMetricsHistory(currentMetricsRange, true);
      }
    }
  }, 60000);

  // Redraw charts on window resize
  window.addEventListener('resize', () => {
    if (window._metricsResizeTimer) clearTimeout(window._metricsResizeTimer);
    window._metricsResizeTimer = setTimeout(() => {
      if (typeof renderAllMetricsCharts === 'function') {
        renderAllMetricsCharts();
      }
    }, 150);
  });
});

function switchToTab(tab) {
  const tabButtons = document.querySelectorAll('.nav-item');
  const tabPanes = document.querySelectorAll('.tab-pane');
  const pageTitle = document.getElementById('page-title');
  const pageSubtitle = document.getElementById('page-subtitle');

  tabButtons.forEach(b => {
    if (b.dataset.tab === tab) {
      b.classList.add('active');
    } else {
      b.classList.remove('active');
    }
  });

  tabPanes.forEach(p => p.classList.remove('active'));
  const targetPane = document.getElementById(`tab-${tab}`);
  if (targetPane) targetPane.classList.add('active');

  if (pageTitle) pageTitle.textContent = t(`page_${tab}_title`, 'Node Overview');
  if (pageSubtitle) pageSubtitle.textContent = t(`page_${tab}_sub`, 'System state, hardware resources, and security boundary');

  if (tab === 'overview' && typeof loadMetricsHistory === 'function') {
    loadMetricsHistory(currentMetricsRange);
  }
}

function setupTabs() {
  const tabButtons = document.querySelectorAll('.nav-item');
  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      switchToTab(btn.dataset.tab);
    });
  });
}

async function refreshData() {
  try {
    const [statusRes, modulesRes, ringRes] = await Promise.all([
      fetch('/api/status').then(r => r.json()),
      fetch('/api/modules').then(r => r.json()),
      fetch('/api/ring').then(r => r.json())
    ]);

    currentStatus = statusRes ? statusRes.data : null;
    currentModules = modulesRes ? modulesRes.data : null;
    currentRing = ringRes ? ringRes.data : null;

    try {
      await fetchPhotosSharesIntegration();
    } catch (_) {}

    try {
      const netRes = await fetch('/api/network/status').then(r => r.json());
      if (netRes && netRes.status === 'ok' && netRes.data) {
        if (netRes.data.mesh_ip && netRes.data.mesh_ip !== '--') {
          currentMeshIP = netRes.data.mesh_ip;
          window.currentMeshIP = currentMeshIP;
        }
        if (netRes.data.client_runtime) {
          window.currentClientRuntime = netRes.data.client_runtime;
        }
      }
    } catch (_) {}

    renderLaunchpad();
    renderOverview();
    renderModules();
    renderRing();
    renderResilience();
    checkSetupStatus();
    loadWoLDevices();
  } catch (err) {
    showAlert('Errore di comunicazione con il backend Allod: ' + err.message, 'danger');
  }
}

let launchpadMode = 'lan';
let currentMeshIP = '--';
window.currentMeshIP = '--';

function setLaunchpadMode(mode) {
  launchpadMode = mode;
  const lanBtn = document.getElementById('launchpad-toggle-lan');
  const meshBtn = document.getElementById('launchpad-toggle-mesh');
  if (lanBtn && meshBtn) {
    if (mode === 'lan') {
      lanBtn.className = 'btn btn-sm btn-primary';
      lanBtn.style.background = '';
      lanBtn.style.color = '';
      meshBtn.className = 'btn btn-sm';
      meshBtn.style.background = 'transparent';
      meshBtn.style.color = 'var(--text-muted)';
    } else {
      meshBtn.className = 'btn btn-sm btn-primary';
      meshBtn.style.background = '';
      meshBtn.style.color = '';
      lanBtn.className = 'btn btn-sm';
      lanBtn.style.background = 'transparent';
      lanBtn.style.color = 'var(--text-muted)';
    }
  }
  renderLaunchpad();
}

function renderLaunchpad() {
  const container = document.getElementById('launchpad-grid');
  if (!container || !currentModules) return;

  const lanHost = window.location.hostname || '192.168.1.50';
  const meshHost = (currentMeshIP && currentMeshIP !== '--') ? currentMeshIP : lanHost;
  const activeHost = (launchpadMode === 'mesh') ? meshHost : lanHost;

  const apps = [
    {
      id: 'photos',
      name: 'Immich Photos',
      icon: '📸',
      color: '#38bdf8',
      desc: 'Galleria fotografica ad altissime prestazioni con timeline, album e backup automatico continuo da smartphone.',
      port: 2283,
      url: `http://${activeHost}:2283`,
      path: null,
      primaryActionText: '🌐 Apri Galleria Foto',
      secondaryActionText: null
    },
    {
      id: 'media',
      name: 'Jellyfin Media Server',
      icon: '🍿',
      color: '#a855f7',
      desc: 'Cinema personale per film in 4K, serie TV e libreria musicale in streaming diretto senza pubblicità.',
      port: 8096,
      url: `http://${activeHost}:8096`,
      path: `\\\\${activeHost}\\shares\\media`,
      primaryActionText: '🎬 Apri Streaming Video',
      secondaryActionText: '📁 Cartella Media'
    },
    {
      id: 'cloud',
      name: 'Nextcloud Hub',
      icon: '☁️',
      color: '#0284c7',
      desc: 'Cloud personale per documenti, contatti, calendario e sincronizzazione desktop/mobile (Dropbox/Drive privato).',
      port: 8443,
      url: `http://${activeHost}:8443`,
      path: null,
      primaryActionText: '📂 Apri Cloud Drive',
      secondaryActionText: null
    },
    {
      id: 'shares',
      name: 'Samba File Shares',
      icon: '📁',
      color: '#10b981',
      desc: 'Condivisioni di rete locali native per Esplora Risorse di Windows, Mac Finder e app di gestione file mobili.',
      port: 445,
      url: null,
      path: `\\\\${activeHost}\\shares`,
      primaryActionText: '📋 Copia Percorso Rete',
      secondaryActionText: '🔑 Password SMB'
    },
    {
      id: 'network',
      name: 'NetBird Sovereign Mesh',
      icon: '🌐',
      color: '#f59e0b',
      desc: 'Rete mesh WireGuard sovrana punto-punto per accedere a Immich, Jellyfin, Nextcloud e Samba da remoto con zero porte aperte sul router.',
      port: null,
      url: null,
      path: null,
      primaryActionText: '📱 Connetti Dispositivi',
      secondaryActionText: '⚙️ Configura NetBird'
    },
    {
      id: 'storage',
      name: 'Pannello Allod & Storage RAID 1',
      icon: '🛡️',
      color: '#ec4899',
      desc: 'Gestione del sistema operativo, monitoraggio dei dischi fisici Btrfs RAID 1 con checksum e stato dei moduli.',
      port: 8080,
      url: null,
      path: null,
      primaryActionText: '📊 Stato Dischi & RAM',
      secondaryActionText: '📦 Gestione Moduli'
    }
  ];

  container.innerHTML = '';

  apps.forEach(app => {
    const mod = currentModules.find(m => m.id === app.id);
    const isRunning = mod ? (mod.runtime_status === 'running' || mod.runtime_status === 'active') : (app.id === 'storage');
    const isInstalled = mod ? (mod.current_level && mod.current_level !== 'off') : true;

    const card = document.createElement('div');
    card.className = `launchpad-card ${!isRunning && isInstalled ? 'offline' : ''}`;

    let statusBadge = `<span class="badge badge-success">🟢 Online</span>`;
    if (!isInstalled) {
      statusBadge = `<span class="badge" style="background:#334155; color:#94a3b8;">⚪ Non Configurato</span>`;
    } else if (!isRunning) {
      statusBadge = `<span class="badge" style="background:#ef4444; color:#fff;">⏹ Fermato</span>`;
    }

    let endpointDisplay = '';
    if (app.url) {
      endpointDisplay = `
        <div class="launchpad-endpoint">
          <span>${app.url}</span>
          <span style="font-size:10px; color:var(--text-muted);">Porta ${app.port}</span>
        </div>
      `;
    } else if (app.path) {
      endpointDisplay = `
        <div class="launchpad-endpoint">
          <span>${app.path}</span>
          <span style="font-size:10px; color:var(--text-muted);">SMB LAN</span>
        </div>
      `;
    } else if (app.id === 'network') {
      endpointDisplay = `
        <div class="launchpad-endpoint">
          <span>IP Mesh: ${meshHost}</span>
          <span style="font-size:10px; color:var(--text-muted);">Zero-Trust</span>
        </div>
      `;
    }

    let actionButtons = '';
    if (app.id === 'storage') {
      actionButtons = `
        <div class="launchpad-actions">
          <button class="launchpad-btn-primary" onclick="switchToTab('overview')">
            ${app.primaryActionText}
          </button>
          <button class="launchpad-btn-secondary" onclick="switchToTab('modules')">
            ${app.secondaryActionText}
          </button>
        </div>
      `;
    } else if (app.id === 'shares') {
      actionButtons = `
        <div class="launchpad-actions">
          <button class="launchpad-btn-primary" onclick="copyTextToClipboard('${app.path}', this)">
            ${app.primaryActionText}
          </button>
          <button class="launchpad-btn-secondary" onclick="openSmbPasswordModal()">
            ${app.secondaryActionText}
          </button>
        </div>
      `;
    } else if (app.id === 'network') {
      actionButtons = `
        <div class="launchpad-actions">
          <button class="launchpad-btn-primary" onclick="openNetworkPairingModal()">
            ${app.primaryActionText}
          </button>
          <button class="launchpad-btn-secondary" onclick="openNetworkConfigModal()">
            ${app.secondaryActionText}
          </button>
        </div>
      `;
    } else if (isRunning && app.url) {
      actionButtons = `
        <div class="launchpad-actions">
          <a href="${app.url}" target="_blank" class="launchpad-btn-primary">
            ${app.primaryActionText} ➔
          </a>
          ${app.path ? `
            <button class="launchpad-btn-secondary" onclick="copyTextToClipboard('${app.path}', this)">
              ${app.secondaryActionText}
            </button>
          ` : ''}
        </div>
      `;
    } else if (isInstalled && !isRunning) {
      actionButtons = `
        <div class="launchpad-actions">
          <button class="launchpad-btn-primary" style="background:#3b82f6; color:#fff;" onclick="startModule('${app.id}')">
            ▶ Avvia Modulo
          </button>
          <button class="launchpad-btn-secondary" onclick="switchToTab('modules')">
            ⚙️ Impostazioni
          </button>
        </div>
      `;
    } else {
      actionButtons = `
        <div class="launchpad-actions">
          <button class="launchpad-btn-primary" style="background:#334155; color:#f8fafc;" onclick="switchToTab('modules')">
            📦 Abilita in Moduli
          </button>
        </div>
      `;
    }

    card.innerHTML = `
      <div class="launchpad-card-glow" style="background:${app.color};"></div>
      <div>
        <div class="launchpad-header">
          <div class="launchpad-icon">${app.icon}</div>
          <div>${statusBadge}</div>
        </div>
        <div class="launchpad-title">${app.name}</div>
        <div class="launchpad-subtitle">${mod && mod.current_level ? `Livello: ${mod.current_level.toUpperCase()}` : ''}</div>
        <div class="launchpad-desc">${app.desc}</div>
        ${endpointDisplay}
      </div>
      ${actionButtons}
    `;

    container.appendChild(card);
  });
}

function goToOverviewHealth() {
  switchToTab('overview');
  setTimeout(() => {
    const el = document.getElementById('card-server-vitals');
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, 100);
}
window.goToOverviewHealth = goToOverviewHealth;

async function fetchLiveVitals() {
  try {
    const res = await fetch('/api/system/vitals');
    const data = await res.json();
    if (data && data.status === 'ok' && data.data) {
      renderServerVitals(data.data);
    }
  } catch (err) {
    // Silent fail for polling
  }
}
window.fetchLiveVitals = fetchLiveVitals;

function renderServerVitals(v) {
  if (!v) return;

  const tempVal = v.cpu_temp_c ? v.cpu_temp_c.toFixed(1) : '--';
  const cpuVal = v.cpu_usage_percent !== undefined ? v.cpu_usage_percent.toFixed(1) : '--';
  const uptimeVal = v.uptime_formatted || '--';
  const status = v.temp_status || 'normal';

  // 1. Topbar Pill Updates
  const topbarTemp = document.getElementById('topbar-temp-val');
  if (topbarTemp) topbarTemp.textContent = v.cpu_temp_c ? `${tempVal}°C` : '--°C';

  const topbarBadge = document.getElementById('topbar-temp-badge');
  if (topbarBadge) {
    topbarBadge.className = `vitals-temp-badge temp-${status}`;
  }

  const topbarCpu = document.getElementById('topbar-cpu-val');
  if (topbarCpu) topbarCpu.textContent = `${cpuVal}%`;

  const topbarUptime = document.getElementById('topbar-uptime-val');
  if (topbarUptime) topbarUptime.textContent = uptimeVal;

  // 2. Overview Card Updates
  const cardTemp = document.getElementById('vitals-cpu-temp-val');
  if (cardTemp) cardTemp.textContent = v.cpu_temp_c ? `${tempVal}°C` : '--°C';

  const cardProg = document.getElementById('vitals-temp-progress');
  if (cardProg) {
    const pct = v.cpu_temp_c ? Math.min(100, Math.max(5, (v.cpu_temp_c / 90) * 100)) : 0;
    cardProg.style.width = `${pct}%`;
    if (status === 'normal') cardProg.style.background = '#10b981';
    else if (status === 'warm') cardProg.style.background = '#f59e0b';
    else if (status === 'hot') cardProg.style.background = '#ef4444';
    else cardProg.style.background = '#dc2626';
  }

  const overallBadge = document.getElementById('vitals-temp-overall-badge');
  if (overallBadge) {
    if (status === 'normal') {
      overallBadge.className = 'badge badge-success';
      overallBadge.textContent = (currentLang === 'it') ? '🟢 Normale (< 60°C)' : '🟢 Normal (< 60°C)';
    } else if (status === 'warm') {
      overallBadge.className = 'badge badge-warning';
      overallBadge.textContent = (currentLang === 'it') ? '🟡 Sotto Carico (60-75°C)' : '🟡 Under Load (60-75°C)';
    } else if (status === 'hot') {
      overallBadge.className = 'badge badge-danger';
      overallBadge.textContent = (currentLang === 'it') ? '🔴 Caldo (> 75°C)' : '🔴 High Temp (> 75°C)';
    } else {
      overallBadge.className = 'badge badge-danger';
      overallBadge.style.background = '#dc2626';
      overallBadge.textContent = (currentLang === 'it') ? '🔥 Throttling Critico' : '🔥 Critical Throttling';
    }
  }

  const cardCpu = document.getElementById('vitals-cpu-load-val');
  if (cardCpu) cardCpu.textContent = `${cpuVal}%`;

  const cpuProg = document.getElementById('vitals-cpu-progress');
  if (cpuProg) {
    const cPct = Math.min(100, Math.max(0, v.cpu_usage_percent || 0));
    cpuProg.style.width = `${cPct}%`;
    if (cPct > 80) cpuProg.style.background = '#ef4444';
    else if (cPct > 50) cpuProg.style.background = '#f59e0b';
    else cpuProg.style.background = '#38bdf8';
  }

  const loadFooter = document.getElementById('vitals-loadavg-footer');
  if (loadFooter) {
    const l1 = (v.load_avg_1 !== undefined) ? v.load_avg_1.toFixed(2) : '--';
    const l5 = (v.load_avg_5 !== undefined) ? v.load_avg_5.toFixed(2) : '--';
    const l15 = (v.load_avg_15 !== undefined) ? v.load_avg_15.toFixed(2) : '--';
    loadFooter.textContent = `Load Avg: ${l1}, ${l5}, ${l15}`;
  }

  const cardUptime = document.getElementById('vitals-uptime-val');
  if (cardUptime) cardUptime.textContent = uptimeVal;

  const cardCores = document.getElementById('vitals-cores-val');
  if (cardCores) {
    const coreCount = v.cpu_cores || 1;
    cardCores.textContent = (currentLang === 'it')
      ? `${coreCount} Core logici rilevati`
      : `${coreCount} Logical cores detected`;
  }

  // 3. Dynamic Sensors List
  const sensorsList = document.getElementById('vitals-sensors-list');
  if (sensorsList) {
    if (Array.isArray(v.sensors) && v.sensors.length > 0) {
      sensorsList.innerHTML = v.sensors.map(s => {
        let icon = '🌡️';
        if (s.type === 'cpu') icon = '⚡';
        else if (s.type === 'disk') icon = '💾';
        else if (s.type === 'ambient') icon = '🏠';

        const sTemp = s.temp_c ? s.temp_c.toFixed(1) : '--';
        const sStatus = s.temp_c ? (s.temp_c < 60 ? 'temp-normal' : (s.temp_c < 75 ? 'temp-warm' : 'temp-hot')) : '';

        return `
          <div class="vitals-sensor-card">
            <div style="display:flex; align-items:center; gap:6px; min-width:0;">
              <span>${icon}</span>
              <span class="vitals-sensor-label" title="${s.label}">${s.label}</span>
            </div>
            <span class="vitals-sensor-temp ${sStatus}">${sTemp}°C</span>
          </div>
        `;
      }).join('');
    } else {
      sensorsList.innerHTML = `
        <div style="grid-column: 1 / -1; padding: 10px; background: rgba(0,0,0,0.25); border-radius: 6px; font-size: 12px; color: var(--text-muted);">
          ℹ️ ${t('vitals_no_sensors', 'Nessun sensore termico dedicato rilevato (ambiente virtualizzato o container)')}
        </div>
      `;
    }
  }
}

function renderOverview() {
  if (!currentStatus) return;

  if (currentStatus.vitals) {
    renderServerVitals(currentStatus.vitals);
  }

  // Helper Permission Denied banner (EACCES migration)
  const eaccesBanner = document.getElementById('helper-eacces-banner');
  if (eaccesBanner) {
    if (currentStatus.helper_permission_denied) {
      eaccesBanner.classList.remove('hidden');
    } else {
      eaccesBanner.classList.add('hidden');
    }
  }

  // Helper Root connection pill
  const helperPill = document.getElementById('helper-status-pill');
  if (helperPill) {
    if (currentStatus.helper_connected) {
      helperPill.className = 'helper-status';
      helperPill.innerHTML = `<span class="status-indicator"></span> ${t('helper_connected')}`;
    } else if (currentStatus.helper_permission_denied) {
      helperPill.className = 'helper-status offline';
      helperPill.innerHTML = `<span class="status-indicator offline"></span> ${t('helper_eacces_pill', 'Helper: Permesso Negato')}`;
    } else {
      helperPill.className = 'helper-status offline';
      helperPill.innerHTML = `<span class="status-indicator offline"></span> ${t('helper_offline')}`;
    }
  }

  // Settings Helper Root connection pill (maintenance tab)
  const settingsHelperPill = document.getElementById('settings-helper-status-pill');
  const settingsHelperText = document.getElementById('settings-helper-status-text');
  if (settingsHelperPill && settingsHelperText) {
    if (currentStatus.helper_connected) {
      settingsHelperPill.className = 'helper-status';
      settingsHelperText.textContent = t('helper_connected');
    } else if (currentStatus.helper_permission_denied) {
      settingsHelperPill.className = 'helper-status offline';
      settingsHelperText.textContent = t('helper_eacces_pill', 'Helper: Permesso Negato');
    } else {
      settingsHelperPill.className = 'helper-status offline';
      settingsHelperText.textContent = t('helper_offline');
    }
  }

  const sidebarName = document.getElementById('sidebar-node-name');
  if (sidebarName) sidebarName.textContent = currentStatus.node_name || 'allod-node';

  const sidebarVer = document.getElementById('sidebar-app-version');
  if (sidebarVer && currentStatus.version) {
    sidebarVer.textContent = currentStatus.version.startsWith('v') ? currentStatus.version : `v${currentStatus.version}`;
  }
  const settingsVer = document.getElementById('settings-app-version');
  if (settingsVer && currentStatus.version) {
    settingsVer.textContent = currentStatus.version.startsWith('v') ? currentStatus.version : `v${currentStatus.version}`;
  }
  
  // RAM metric
  const ramUsedMB = currentStatus.ram_used_mb || 0;
  const ramTotalMB = currentStatus.ram_total_mb || 8192;
  const ramUsedGB = (ramUsedMB / 1024).toFixed(1);
  const ramTotalGB = (ramTotalMB / 1024).toFixed(1);
  const ramPercent = Math.min(100, Math.round((ramUsedMB / ramTotalMB) * 100));

  const ramUsedEl = document.getElementById('ram-used');
  if (ramUsedEl) ramUsedEl.textContent = `${ramUsedGB} / ${ramTotalGB} GB`;
  
  const ramProgEl = document.getElementById('ram-progress');
  if (ramProgEl) ramProgEl.style.width = `${ramPercent}%`;
  
  const ramFootEl = document.getElementById('ram-footer');
  if (ramFootEl) {
    const availGB = (currentStatus.ram_available_mb ? (currentStatus.ram_available_mb / 1024) : Math.max(0, (ramTotalMB - ramUsedMB) / 1024)).toFixed(1);
    const committedMB = currentStatus.ram_committed_mb || 0;
    ramFootEl.textContent = (currentLang === 'it')
      ? `Disponibile: ${availGB} GB | Allocati moduli: ${committedMB} MB`
      : `Available: ${availGB} GB | Committed: ${committedMB} MB`;
  }

  // Active modules
  if (Array.isArray(currentModules)) {
    const active = currentModules.filter(m => m.current_level && m.current_level !== 'off');
    const running = currentModules.filter(m => m.runtime_status === 'running');
    const actCountEl = document.getElementById('active-modules-count');
    if (actCountEl) {
      actCountEl.textContent = (currentLang === 'it')
        ? `${running.length} in esecuzione (${active.length} configurati)`
        : `${running.length} running (${active.length} configured)`;
    }
    
    const actListEl = document.getElementById('active-modules-list');
    if (actListEl) actListEl.textContent = active.map(m => m.id).join(', ');
  }

  // Ring status metric card
  const ringBadge = document.getElementById('ring-status-badge');
  const ringSubtext = document.getElementById('ring-status-subtext');
  if (currentRing) {
    const isStandalone = currentRing.is_standalone || Object.keys(currentRing.members || {}).length <= 1;
    if (isStandalone) {
      if (ringBadge) ringBadge.textContent = (currentLang === 'it') ? '1 Nodo (Locale)' : '1 Node (Local)';
      if (ringSubtext) ringSubtext.textContent = (currentLang === 'it') ? 'Modalità Standalone (0 peer remoti)' : 'Standalone Mode (0 remote peers)';
    } else {
      const count = Object.keys(currentRing.members || {}).length;
      if (ringBadge) ringBadge.textContent = (currentLang === 'it') ? `${count} Nodi (OK)` : `${count} Nodes (OK)`;
      if (ringSubtext) ringSubtext.textContent = (currentLang === 'it') ? 'Regola 2 repliche remote attiva' : '2 remote replicas rule active';
    }
  }

  // Security metric card (Real check from backend)
  const secBadge = document.getElementById('security-status-badge');
  const secSubtext = document.getElementById('security-status-subtext');
  if (currentStatus) {
    if (currentStatus.is_rootless) {
      if (secBadge) {
        secBadge.className = 'metric-value text-success';
        secBadge.textContent = 'Rootless Safe';
      }
      if (secSubtext) {
        secSubtext.textContent = (currentLang === 'it')
          ? `Utente '${currentStatus.current_user || 'non-root'}' (UID: ${currentStatus.uid || 1000})`
          : `User '${currentStatus.current_user || 'non-root'}' (UID: ${currentStatus.uid || 1000})`;
      }
    } else {
      if (secBadge) {
        secBadge.className = 'metric-value text-warning';
        secBadge.textContent = '⚠️ Root (Privilegiato)';
      }
      if (secSubtext) secSubtext.textContent = 'Attenzione: in esecuzione come root!';
    }
  }

  // Visual Nodes
  const visualGrid = document.getElementById('nodes-visual-grid');
  if (visualGrid && currentRing && currentRing.members) {
    visualGrid.innerHTML = '';
    const isStandalone = currentRing.is_standalone || Object.keys(currentRing.members).length <= 1;

    if (isStandalone) {
      const selfMember = Object.values(currentRing.members)[0] || { id: currentStatus.node_name, address: '127.0.0.1', quota_gb: 500 };
      visualGrid.innerHTML = `
        <div class="node-item-box" style="grid-column: 1 / -1; background: rgba(30, 41, 59, 0.5); border-left: 4px solid var(--primary);">
          <div class="node-item-icon">🏠</div>
          <div class="node-item-info">
            <h4>${selfMember.id} <span class="badge badge-info">Nodo Locale</span></h4>
            <p>Stato: <strong>Standalone (Nessun peer remoto)</strong> | Quota locale: <strong>${selfMember.quota_gb || 500} GB</strong></p>
            <p style="font-size:12px; color:var(--text-muted); margin-top:4px;">I backup sono salvati in locale. Per collegare il server di un amico al tuo Ring, usa il comando: <code>allod ring add &lt;id-amico&gt; &lt;ip-wireguard&gt; &lt;quota_gb&gt;</code></p>
          </div>
        </div>
      `;
    } else {
      Object.values(currentRing.members).forEach(m => {
        const mId = m.id || m.ID || 'node';
        const mAddr = m.address || m.Address || '127.0.0.1';
        const mQuota = m.quota_gb !== undefined ? m.quota_gb : (m.QuotaGB || 500);
        const isSelf = mId === currentStatus.node_name;

        const box = document.createElement('div');
        box.className = 'node-item-box';
        box.innerHTML = `
          <div class="node-item-icon">${isSelf ? '🏠' : '🤝'}</div>
          <div class="node-item-info">
            <h4>${mId} ${isSelf ? '<span class="badge badge-info">Locale</span>' : ''}</h4>
            <p>IP: <code>${mAddr}</code> | Quota: <strong>${mQuota} GB</strong></p>
          </div>
        `;
        visualGrid.appendChild(box);
      });
    }
  }

  // Storage & Physical Disks
  const storageGrid = document.getElementById('storage-disks-grid');
  const storageBadge = document.getElementById('storage-mode-badge');
  const storageSummary = document.getElementById('storage-mode-summary');
  const storageWarn = document.getElementById('storage-warning-banner');
  const st = currentStatus && currentStatus.storage;

  if (storageGrid && st) {
    storageGrid.innerHTML = '';

    if (storageBadge) {
      if (st.mode === 'raid1') {
        storageBadge.className = 'badge badge-success';
        storageBadge.textContent = 'Btrfs RAID 1 (Auto-Healing)';
      } else if (st.mode === 'single') {
        storageBadge.className = 'badge badge-warning';
        storageBadge.textContent = 'Btrfs Single (No RAID 1)';
      } else {
        storageBadge.className = 'badge badge-info';
        storageBadge.textContent = 'Witness / Solo Backup';
      }
    }

    if (storageSummary) {
      storageSummary.textContent = st.mode_summary || 'Topologia storage rilevata dal kernel.';
    }

    if (storageWarn) {
      if (st.has_warning && st.warning_msg) {
        storageWarn.textContent = '⚠️ ' + st.warning_msg;
        storageWarn.classList.remove('hidden');
      } else {
        storageWarn.classList.add('hidden');
      }
    }

    // Render System Disk
    if (st.system_disk) {
      const sys = st.system_disk;
      const box = document.createElement('div');
      box.className = 'node-item-box';
      box.innerHTML = `
        <div class="node-item-icon">💿</div>
        <div class="node-item-info">
          <h4>/dev/${sys.name} <span class="badge badge-info">${sys.is_ssd ? 'SSD' : 'HDD'} Sistema (OS)</span></h4>
          <p>Capacità: <strong>${sys.size_gb} GB</strong> | Modello: <code>${sys.model || 'Sistema'}</code></p>
          <p style="font-size:11px; color:var(--text-muted); margin-top:2px;">Contiene il sistema operativo Ubuntu Server (root <code>/</code>)</p>
        </div>
      `;
      storageGrid.appendChild(box);
    }

    // Render Data Disks
    if (Array.isArray(st.data_disks) && st.data_disks.length > 0) {
      st.data_disks.forEach((d, idx) => {
        const box = document.createElement('div');
        box.className = 'node-item-box';
        box.innerHTML = `
          <div class="node-item-icon">🗄️</div>
          <div class="node-item-info">
            <h4>/dev/${d.name} <span class="badge badge-success">${d.is_ssd ? 'SSD' : 'HDD'} Pool NAS #${idx+1}</span></h4>
            <p>Capacità: <strong>${d.size_gb} GB</strong> | Modello: <code>${d.model || 'Disco Dati'}</code></p>
            <p style="font-size:11px; color:var(--text-muted); margin-top:2px;">Dedicato al pool Btrfs per Nextcloud, Immich, Samba e Backup</p>
          </div>
        `;
        storageGrid.appendChild(box);
      });

      // Storage GUI Action Row
      const actionBox = document.createElement('div');
      actionBox.className = 'node-item-box';
      actionBox.style.gridColumn = '1 / -1';
      actionBox.style.background = 'rgba(16, 185, 129, 0.08)';
      actionBox.style.borderColor = 'rgba(16, 185, 129, 0.3)';

      const isMounted = st.is_mounted || (st.mode === 'raid1' && currentModules.some(m => m.is_on_nas_pool));

      actionBox.innerHTML = `
        <div class="node-item-icon">${isMounted ? '🛡️' : '⚡'}</div>
        <div class="node-item-info" style="display:flex; justify-content:space-between; align-items:center; width:100%; flex-wrap:wrap; gap:8px;">
          <div>
            <h4 style="margin:0; color:var(--text-main);">
              Gestione & Ispezione Pool Storage
              ${isMounted ? '<span class="badge badge-success" style="margin-left:6px; font-size:10.5px;">✅ RAID 1 Operativo & Montato</span>' : ''}
            </h4>
            <p style="font-size:12px; color:var(--text-muted); margin-top:2px;">
              ${isMounted 
                ? 'Il pool Btrfs RAID 1 è attivo su <code>/mnt/allod-storage</code>. I container salvano i dati sui tuoi dischi fisici.' 
                : 'I dischi sono stati rilevati ma il pool storage non è ancora inizializzato.'}
            </p>
          </div>
          <div style="display:flex; gap:8px; flex-wrap:wrap; align-items:center;">
            <button class="btn btn-sm btn-info" id="btn-diag-storage" onclick="showStorageDiagnostics()">
              🔍 Ispezione Btrfs & Checksum
            </button>
            ${!isMounted ? `
              <button class="btn btn-sm btn-primary" id="btn-init-storage" onclick="openDangerStorageModal()">
                ⚡ Inizializza Pool Btrfs (${(st.mode || 'raid1').toUpperCase()})
              </button>
            ` : ''}
          </div>
        </div>
      `;
      storageGrid.appendChild(actionBox);
    } else {
      const box = document.createElement('div');
      box.className = 'node-item-box';
      box.style.gridColumn = '1 / -1';
      box.innerHTML = `
        <div class="node-item-icon">🛡️</div>
        <div class="node-item-info">
          <h4>Nessun disco secondario per NAS dedicato</h4>
          <p style="font-size:12px; color:var(--text-muted);">Questo nodo opera come <strong>Witness / Cassaforte di Backup Remota</strong>. I carichi NAS sono bloccati per non saturare il disco del sistema operativo.</p>
        </div>
      `;
      storageGrid.appendChild(box);
    }
  }

  // Dynamic Setup Flow Banner handling
  const setupBanner = document.getElementById('setup-flow-banner');
  if (setupBanner) {
    const isMounted = st && (st.is_mounted || st.mode === 'raid1');
    const runningApps = (currentModules || []).filter(m => m.id !== 'storage' && m.runtime_status === 'running').length;

    if (isMounted && runningApps > 0) {
      // Setup complete! Hide banner completely
      setupBanner.classList.add('hidden');
    } else if (!isMounted) {
      // Step 1: Need storage init
      setupBanner.classList.remove('hidden');
      setupBanner.className = 'alert-banner alert-warning';
      setupBanner.innerHTML = `
        <div style="display:flex; align-items:center; gap:12px;">
          <span style="font-size:20px;">⚠️</span>
          <div>
            <strong>Passo 1:</strong> Inizializza il Pool Btrfs RAID 1 prima di avviare i container.
          </div>
        </div>
        <button class="btn btn-sm btn-primary" onclick="openDangerStorageModal()" style="padding:3px 10px; font-size:12px;">
          ⚡ Inizializza Pool Btrfs
        </button>
      `;
    } else {
      // Step 2: Storage mounted, but no apps started yet
      setupBanner.classList.remove('hidden');
      setupBanner.className = 'alert-banner alert-success';
      setupBanner.innerHTML = `
        <div style="display:flex; align-items:center; gap:12px;">
          <span style="font-size:20px;">🎉</span>
          <div>
            <strong>Pool NAS RAID 1 Attivo!</strong> Ora puoi avviare i tuoi moduli applicativi.
          </div>
        </div>
        <button class="btn btn-sm btn-outline-info" onclick="switchToTab('modules')" style="padding:3px 10px; font-size:12px;">
          📦 Gestisci Moduli ➔
        </button>
      `;
    }
  }
}

async function initStorageFromGUI() {
  if (!confirm("Vuoi inizializzare/rimontare il pool Btrfs RAID 1 sui dischi fisici del NAS su /mnt/allod-storage?")) {
    return;
  }
  showToast('Inizializzazione pool storage in corso tramite helper root...', 'info');
  try {
    const res = await fetch('/api/storage/init', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showToast(data.message || 'Pool Btrfs inizializzato con successo!', 'success');
      fetchStatus();
      fetchModules();
    } else {
      showToast('Errore: ' + (data.message || 'Operazione fallita'), 'error');
    }
  } catch (err) {
    showToast('Errore di connessione con il pannello: ' + err.message, 'error');
  }
}

const moduleTechInfo = {
  en: {
    cloud: {
      product: 'Nextcloud Hub (Sovereign Cloud)',
      desc: 'Private cloud for files, contacts, calendar, and desktop/mobile sync (private alternative to Google Drive / Dropbox).',
      db: 'SQLite / PostgreSQL Dedicated',
      dbNote: 'Optimal for multi-user collaboration, office suite, and seamless file sync across all client devices.',
      linkPort: 8443,
      linkProtocol: 'http'
    },
    photos: {
      product: 'Immich + PostgreSQL + Valkey',
      desc: 'High-performance photo and video gallery with automatic mobile backup, timeline, albums, and AI search (Google Photos alternative).',
      db: 'PostgreSQL 14 + Vectorchord + Valkey',
      dbNote: 'Vector database engine for ultra-fast semantic photo search, face recognition, and mobile camera roll backup.',
      linkPort: 2283,
      linkProtocol: 'http'
    },
    backup: {
      product: 'rest-server (Restic Engine)',
      desc: 'End-to-end encrypted backup engine with strict ransomware-proof append-only lock mode.',
      db: 'Restic Cryptographic Repository',
      dbNote: 'Immutable snapshots protected by client-side AES-256 encryption with zero third-party cloud dependencies.',
      linkPort: null
    },
    shares: {
      product: 'Samba (SMB/CIFS)',
      desc: 'Ultra-fast local network file sharing for Windows PC, Mac Finder, Linux, and iOS/Android Files app.',
      db: 'Samba Native TDB',
      dbNote: 'Kernel-accelerated lock database integrated directly with the Btrfs filesystem.',
      linkPort: null
    },
    storage: {
      product: 'Btrfs CoW Filesystem + smartmontools',
      desc: 'Advanced storage pool with hardware-safe RAID 1, instantaneous subvolume snapshots, and S.M.A.R.T. health checks.',
      db: 'Btrfs B-Trees (On-Disk Metadata)',
      dbNote: 'Block-level integrity verification and automatic self-healing via CRC32c checksums.',
      linkPort: null
    },
    media: {
      product: 'Jellyfin Media Server',
      desc: 'Personal streaming server for movies, TV series, home videos, and high-fidelity music libraries.',
      db: 'SQLite (Embedded)',
      dbNote: 'Fast embedded database for media metadata, transcoding queues, and playback resume points.',
      linkPort: 8096,
      linkProtocol: 'http'
    },
    network: {
      product: 'NetBird (WireGuard + WebRTC Mesh)',
      desc: 'Zero-trust WireGuard mesh network for ultra-secure remote access to Immich, Jellyfin, and Samba with zero open router ports.',
      db: 'NetBird Config (/etc/netbird)',
      dbNote: 'Peer-to-peer WireGuard cryptography with automated WebRTC NAT traversal. Full support for SMB TCP 445 and 4K Jellyfin streaming.',
      linkPort: null,
      linkProtocol: 'http'
    },
    watch: {
      product: 'Allod Watchdog + WireGuard Mesh',
      desc: 'Continuous peer heartbeat monitoring, quorum supervision, and remote replica coordination.',
      db: 'State.db (Local SQLite)',
      dbNote: 'Idempotent state tracking for peer nodes and automated ring state transitions.',
      linkPort: null
    }
  },
  it: {
    cloud: {
      product: 'Nextcloud Hub (Cloud Sovrano)',
      desc: 'Cloud personale per file, cartelle e sincronizzazione desktop/mobile (alternativa privata a Google Drive / Dropbox).',
      db: 'SQLite / PostgreSQL Dedicato',
      dbNote: 'Ottimale per collaborazione multi-utente, suite office e sync continuo tra tutti i tuoi dispositivi.',
      linkPort: 8443,
      linkProtocol: 'http'
    },
    photos: {
      product: 'Immich + PostgreSQL + Valkey',
      desc: 'Galleria foto ad alte prestazioni con backup automatico da telefono, timeline e album (alternativa a Google Foto).',
      db: 'PostgreSQL 14 + Vectorchord + Valkey',
      dbNote: 'Database relazionale scalabile e motore vettoriale per indicizzazione IA e ricerca semantica ultra-veloce.',
      linkPort: 2283,
      linkProtocol: 'http'
    },
    backup: {
      product: 'rest-server (Restic Backend)',
      desc: 'Motore di backup cifrato end-to-end con modalità append-only rigorosa a prova di ransomware.',
      db: 'Repository Crittografico Restic',
      dbNote: 'Dati immutabili protetti con cifratura client-side AES-256 senza dipendenze da database esterni.',
      linkPort: null
    },
    shares: {
      product: 'Samba (SMB/CIFS)',
      desc: 'Condivisione file ad altissima velocità su rete locale per PC Windows, Mac e Linux.',
      db: 'Samba TDB (Trivial Database nativo)',
      dbNote: 'Database di lock ad alte prestazioni integrato direttamente nel kernel e filesystem Linux.',
      linkPort: null
    },
    storage: {
      product: 'Btrfs CoW Filesystem + smartmontools',
      desc: 'Storage avanzato con RAID 1 hardware-safe, snapshot istantanei e diagnosi di salute S.M.A.R.T.',
      db: 'Btrfs B-Trees (On-Disk Metadata)',
      dbNote: 'Controllo di integrità dei blocchi e auto-riparazione con checksum CRC32c automatico.',
      linkPort: null
    },
    media: {
      product: 'Jellyfin Media Server',
      desc: 'Streaming multimediale personale per film, serie TV, video e musica.',
      db: 'SQLite (Embedded)',
      dbNote: 'Database locale leggero e veloce per librerie musicali e metadati cinematografici.',
      linkPort: 8096,
      linkProtocol: 'http'
    },
    network: {
      product: 'NetBird (WireGuard + WebRTC Mesh)',
      desc: 'Rete mesh privata WireGuard a zero-trust per accedere a Immich, Jellyfin, Nextcloud e Samba da fuori casa senza aprire porte sul router.',
      db: 'Configurazione NetBird (/etc/netbird)',
      dbNote: 'Crittografia WireGuard punto-punto con NAT traversal WebRTC automatico. Supporto nativo per streaming Jellyfin 4K e file sharing Samba TCP 445.',
      linkPort: null,
      linkProtocol: 'http'
    },
    watch: {
      product: 'Allod Watchdog + WireGuard Mesh',
      desc: 'Supervisione continua dei battiti cardiaci dei nodi amici e coordinamento repliche.',
      db: 'State.db (SQLite locale)',
      dbNote: 'Tracciamento idempotente dello stato dei nodi e delle transizioni del ring.',
      linkPort: null
    }
  }
};

function getModuleTechInfo(modID, currentLevel) {
  const lang = (typeof currentLang !== 'undefined' && i18n[currentLang]) ? currentLang : 'en';
  const dict = moduleTechInfo[lang] || moduleTechInfo['en'];

  if (modID === 'cloud') {
    if (currentLevel === 'standard') {
      return {
        product: lang === 'it' ? 'Nextcloud Hub (PostgreSQL Dedicato)' : 'Nextcloud Hub (PostgreSQL Dedicated)',
        desc: lang === 'it' 
          ? 'Cloud personale avanzato con database relazionale PostgreSQL, calendario, contatti e sync multi-utente massivo.'
          : 'Advanced personal cloud with PostgreSQL relational database, calendar, contacts, and massive multi-user sync.',
        db: 'PostgreSQL (Dedicated Container)',
        dbNote: lang === 'it'
          ? 'Massime prestazioni (600 MB RAM): transazioni concorrenti isolate, affidabilità enterprise per migliaia di file.'
          : 'Peak performance (600 MB RAM): isolated concurrent transactions, enterprise reliability for thousands of files.',
        linkPort: 8443,
        linkProtocol: 'http'
      };
    }
    return {
      product: lang === 'it' ? 'Nextcloud Hub (SQLite Leggero)' : 'Nextcloud Hub (SQLite Lightweight)',
      desc: lang === 'it'
        ? 'Cloud personale per file, cartelle e sincronizzazione desktop/mobile (alternativa privata a Google Drive / Dropbox).'
        : 'Private cloud for files, folders, and desktop/mobile sync (private alternative to Google Drive / Dropbox).',
      db: 'SQLite (Embedded)',
      dbNote: lang === 'it'
        ? 'Consumo minimo (200 MB RAM), perfetto per uso personale (1 utente).'
        : 'Minimal consumption (200 MB RAM), perfect for personal use (1 user).',
      linkPort: 8443,
      linkProtocol: 'http'
    };
  }
  if (modID === 'photos') {
    if (currentLevel === 'full') {
      return {
        product: lang === 'it' ? 'Immich (Full AI Search & Face Recognition)' : 'Immich (Full AI Search & Face Recognition)',
        desc: lang === 'it'
          ? 'Galleria foto con IA avanzata: riconoscimento volti automatico, ricerca semantica vettoriale e timeline (4000 MB RAM).'
          : 'Photo gallery with advanced AI: facial recognition, vector semantic search, and timeline (4000 MB RAM).',
        db: 'PostgreSQL 14 + Vectorchord + Valkey',
        dbNote: lang === 'it'
          ? 'Database vettoriale scalabile per indicizzazione IA e ricerca semantica ad altissima velocità.'
          : 'Scalable vector database for AI indexing and ultra-fast semantic search.',
        linkPort: 2283,
        linkProtocol: 'http'
      };
    }
    return dict.photos || {
      product: 'Immich + PostgreSQL + Valkey',
      desc: 'Photo gallery with mobile backup and timeline',
      db: 'PostgreSQL 14 + Vectorchord + Valkey',
      dbNote: 'Relational database and vector engine for fast indexing',
      linkPort: 2283,
      linkProtocol: 'http'
    };
  }
  return dict[modID] || {
    product: modID,
    desc: 'Allod Module',
    db: 'Standard',
    dbNote: 'Default configuration',
    linkPort: null
  };
}

function createModuleCard(mod) {
  const card = document.createElement('div');
  card.className = 'module-card';

  const isOff = !mod.current_level || mod.current_level === 'off';
  const tierBadge = mod.tier === 'core'
    ? 'badge-primary'
    : (mod.tier === 'recommended' ? 'badge-warning' : 'badge-purple');
  const manifest = mod.manifest || {};
  const levels = manifest.levels || manifest.Levels || {};

  const tech = getModuleTechInfo(mod.id, mod.current_level);

  let levelOptions = Object.keys(levels).map(lvl => {
    const selected = lvl === mod.current_level ? 'selected' : '';
    return `<option value="${lvl}" ${selected}>${lvl}</option>`;
  }).join('');

  const currentLevelInfo = levels[mod.current_level] || {};
  const grants = currentLevelInfo.grants || currentLevelInfo.Grants || [];
  const ramReq = currentLevelInfo.ram_mb || currentLevelInfo.RAMMB || 0;

  let grantsHtml = grants.length > 0
    ? `<ul class="grants-list">${grants.map(g => `<li>✓ ${g}</li>`).join('')}</ul>`
    : `<p class="text-muted font-mono" style="font-size:11px; margin-top:8px;">Modulo disattivato</p>`;

  const priv = manifest.privileges || manifest.Privileges || {};
  const imgs = manifest.images || manifest.Images || [];

  // Runtime status badge
  let statusBadge = `<span class="badge badge-secondary">OFF</span>`;
  let actionButtons = ``;
  let openLinkHtml = ``;

  // Critical Data Modules that must be protected once in production:
  const criticalDataModules = ['storage', 'cloud', 'photos', 'backup', 'shares', 'media'];
  const isDataCritical = criticalDataModules.includes(mod.id) || (mod.mounts && mod.mounts.length > 0);
  const isRunningOrNAS = (mod.runtime_status === 'running' || (mod.id === 'storage' && mod.is_on_nas_pool));
  const isOrphan = isOff && mod.runtime_status === 'running';
  const isLocked = isDataCritical && isRunningOrNAS && !isOrphan && !unlockedModules.has(mod.id);
  const isUnlocked = isDataCritical && isRunningOrNAS && !isOrphan && unlockedModules.has(mod.id);

  if (isOrphan) {
    card.className = 'module-card';
    statusBadge = `<span class="badge badge-warning" style="background:#f59e0b; color:#000; font-weight:700;">⚠️ ATTIVO (OFF)</span>`;
    actionButtons = `
      <button class="btn btn-sm btn-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;" title="Termina forzatamente il container Podman attivo">⏹ ${t('btn_stop')}</button>
      <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
    `;
    if (tech.linkPort) {
      const host = window.location.hostname || '127.0.0.1';
      openLinkHtml = `
        <div style="margin-top:10px; padding:6px 10px; background:rgba(245,158,11,0.15); border-radius:6px; border:1px solid rgba(245,158,11,0.3);">
          <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:#f59e0b; font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
            <span>⚠️ ${t('btn_open_web')} (${tech.product} - Container orfano)</span>
            <span>Port ${tech.linkPort} ↗</span>
          </a>
        </div>
      `;
    }
  } else if (isLocked) {
    card.className = 'module-card module-card-locked';
    statusBadge = `<span class="badge badge-success">${t('status_protected')}</span>`;
    
    let diagBtn = (mod.id === 'storage')
      ? `<button class="btn btn-sm btn-info" onclick="showStorageDiagnostics()" style="padding:2px 8px; font-size:11px;">🔍 ${t('btn_inspect_btrfs', 'Inspect Btrfs')}</button>`
      : `<button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px;">🩺 ${t('btn_diagnostics')}</button>`;

    actionButtons = `
      ${diagBtn}
      <button class="btn btn-sm btn-outline-secondary" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🔓 ${t('btn_unlock')}</button>
    `;

    if (tech.linkPort) {
      const host = window.location.hostname || '127.0.0.1';
      openLinkHtml = `
        <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
          <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
            <span>🌐 ${t('btn_open_web')} (${tech.product})</span>
            <span>Port ${tech.linkPort} ↗</span>
          </a>
        </div>
      `;
    }
  } else if (isUnlocked) {
    card.className = 'module-card';
    statusBadge = `<span class="badge badge-warning">${t('status_unlocked')}</span>`;
    
    let diagBtn = (mod.id === 'storage')
      ? `<button class="btn btn-sm btn-info" onclick="showStorageDiagnostics()" style="padding:2px 8px; font-size:11px;">🔍 ${t('btn_inspect_btrfs', 'Inspect Btrfs')}</button>`
      : `<button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px;">🩺 ${t('btn_diagnostics')}</button>`;

    actionButtons = `
      ${mod.id !== 'storage' ? `<button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">⏹ ${t('btn_stop')}</button>` : ''}
      ${diagBtn}
      <button class="btn btn-sm btn-warning" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🔒 ${t('btn_lock')}</button>
    `;

    if (tech.linkPort) {
      const host = window.location.hostname || '127.0.0.1';
      openLinkHtml = `
        <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
          <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
            <span>🌐 ${t('btn_open_web')} (${tech.product})</span>
            <span>Port ${tech.linkPort} ↗</span>
          </a>
        </div>
      `;
    }
  } else if (!isOff) {
    if (mod.runtime_status === 'running') {
      statusBadge = `<span class="badge badge-success">${t('status_running')}</span>`;
      actionButtons = `
        <button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">⏹ ${t('btn_stop')}</button>
        <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
      `;
      
      if (tech.linkPort) {
        const host = window.location.hostname || '127.0.0.1';
        openLinkHtml = `
          <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
            <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
              <span>🌐 ${t('btn_open_web')} (${tech.product})</span>
              <span>Port ${tech.linkPort} ↗</span>
            </a>
          </div>
        `;
      }
    } else if (mod.runtime_status === 'failed') {
      statusBadge = `<span class="badge badge-danger">${t('status_error')}</span>`;
      actionButtons = `
        <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ ${t('btn_restart')}</button>
        <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
        <button class="btn btn-sm btn-outline-danger" onclick="openDangerPurgeModal('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🗑️ ${t('btn_reset')}</button>
      `;
    } else {
      statusBadge = `<span class="badge badge-warning">${t('status_stopped')}</span>`;
      actionButtons = `
        <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ ${t('btn_start')}</button>
        <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
      `;
    }
  }

  // Check if module is currently starting (disable button and show spinner)
  if (startingModules.has(mod.id)) {
    const elapsed = Date.now() - startingModules.get(mod.id);
    if (mod.runtime_status === 'running' || elapsed > 25000) {
      startingModules.delete(mod.id);
    } else {
      statusBadge = `<span class="badge badge-info" style="opacity:0.9;">${t('status_starting')}</span>`;
      actionButtons = `
        <button class="btn btn-sm btn-secondary" disabled style="opacity:0.65; cursor:wait; padding:2px 10px; font-size:11px; margin-left:8px;">${t('status_starting')}</button>
        <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
      `;
    }
  }

  let dbInfoHtml = ``;
  if (tech.db) {
    dbInfoHtml = `
      <div style="margin-top:8px; padding:6px 10px; background:rgba(15, 23, 42, 0.6); border-radius:6px; border-left:3px solid var(--primary); font-size:11px;">
        <div style="font-weight:600; color:var(--text-main); margin-bottom:2px;">🗄️ ${t('label_database')}: <span style="color:var(--primary);">${tech.db}</span></div>
        <div style="color:var(--text-muted); line-height:1.35;">${tech.dbNote}</div>
      </div>
    `;
  }

  let storageBoxHtml = ``;
  if (mod.storage_path) {
    const nasBadge = mod.is_on_nas_pool 
      ? `<span class="badge badge-success" style="font-size:10px;">${t('pool_raid1')}</span>` 
      : `<span class="badge badge-warning" style="font-size:10px;">${t('pool_local')}</span>`;

    storageBoxHtml = `
      <div style="margin-top:8px; padding:7px 10px; background:rgba(15, 23, 42, 0.6); border-radius:6px; border-left:3px solid ${mod.is_on_nas_pool ? 'var(--success)' : 'var(--warning)'}; font-size:11px;">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:3px;">
          <span style="font-weight:600; color:var(--text-main);">${t('label_nas_storage')}</span>
          ${nasBadge}
        </div>
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:4px;">
          <span style="font-family:'JetBrains Mono',monospace; font-size:10.5px; color:var(--primary);">📁 ${mod.storage_path}</span>
          <strong style="color:var(--text-main); font-size:11px;">${mod.storage_size || '0 B'}</strong>
        </div>
        ${mod.mounts && mod.mounts.length > 0 ? `
          <div style="border-top:1px dashed rgba(255,255,255,0.1); padding-top:4px; margin-top:4px;">
            ${mod.mounts.map(m => `
              <div style="font-size:10px; color:var(--text-muted); display:flex; justify-content:space-between; margin-bottom:2px; font-family:'JetBrains Mono',monospace;">
                <span>↳ .../${m.host_path.split('/').slice(-2).join('/')} ➔ <code>${m.container_path}</code></span>
                <span style="color:${m.exists ? 'var(--success)' : 'var(--text-muted)'}">${m.exists ? m.size_human : t('label_pending')}</span>
              </div>
            `).join('')}
          </div>
        ` : ''}
      </div>
    `;
  }

  let photosSharesIntegrationHtml = '';
  if (mod.id === 'photos') {
    const isChecked = (photosSharesState && photosSharesState.enabled) ? 'checked' : '';
    const isMounted = photosSharesState && photosSharesState.mounted;
    const sharesActive = photosSharesState && photosSharesState.shares_active;

    let badgeHtml = isMounted
      ? `<span style="color:var(--success); font-weight:600;">${t('photos_shares_connected')}</span>`
      : `<span style="color:var(--text-muted);">${t('photos_shares_unmounted')}</span>`;

    let warningHtml = '';
    if (photosSharesState && photosSharesState.enabled && !sharesActive) {
      warningHtml = `<div style="font-size:10.5px; color:#f59e0b; margin-top:4px;">${t('photos_shares_warning_shares')}</div>`;
    }

    photosSharesIntegrationHtml = `
      <div style="margin-top:10px; padding:9px 12px; background:rgba(30, 41, 59, 0.6); border:1px solid var(--card-border); border-radius:6px;">
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <label style="display:flex; align-items:center; gap:8px; cursor:pointer; margin:0; font-size:11.5px; font-weight:600; color:var(--text-main);">
            <input type="checkbox" id="photos-shares-checkbox" onchange="togglePhotosSharesIntegration(this.checked)" ${isChecked} style="cursor:pointer; width:15px; height:15px; accent-color:var(--primary);">
            <span>🔗 ${t('photos_shares_toggle')}</span>
          </label>
          <div style="font-size:10.5px;">${badgeHtml}</div>
        </div>
        <div style="font-size:10.5px; color:var(--text-muted); margin-top:4px; line-height:1.35;">
          ${t('photos_shares_desc')}
        </div>
        ${warningHtml}
      </div>
    `;
  }

  let sharesSmbBoxHtml = '';
  if (mod.id === 'shares') {
    sharesSmbBoxHtml = `
      <div style="margin-top:10px; padding:9px 12px; background:rgba(30, 41, 59, 0.6); border:1px solid var(--card-border); border-radius:6px; display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:8px;">
        <div>
          <div style="font-size:11.5px; font-weight:600; color:var(--text-main);">🔑 ${t('label_smb_credentials')}</div>
          <div style="font-size:10.5px; color:var(--text-muted);">${t('desc_smb_credentials')}</div>
        </div>
        <button class="btn btn-sm btn-primary" onclick="openSmbPasswordModal()" style="padding:4px 12px; font-size:11px; font-weight:600;">
          🔑 ${t('btn_smb_password')}
        </button>
      </div>
    `;
  }

  let networkBoxHtml = '';
  if (mod.id === 'network') {
    const displayMeshIp = (currentMeshIP && currentMeshIP !== '--') ? currentMeshIP : '--';
    const isNative = window.currentClientRuntime === 'native';
    const runtimeBadge = isNative
      ? `<span class="badge" style="background:#10b981; color:#0f172a; font-weight:700; font-size:10px; margin-left:4px;">⚡ NATIVO</span>`
      : `<span class="badge" style="background:#38bdf8; color:#0f172a; font-weight:700; font-size:10px; margin-left:4px;">📦 CONTAINER</span>`;
    networkBoxHtml = `
      <div style="margin-top:10px; padding:10px 12px; background:rgba(30, 41, 59, 0.6); border:1px solid var(--card-border); border-radius:6px;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:8px;">
          <div>
            <div style="display:flex; align-items:center; gap:6px;">
              <span style="font-size:11.5px; font-weight:600; color:var(--text-main);">🌐 ${t('network_module_title')}</span>
              ${runtimeBadge}
            </div>
            <div style="font-size:10.5px; color:var(--text-muted); margin-top:2px;">
              ${t('network_mesh_ip_label')} <code id="network-mesh-ip-badge">${displayMeshIp}</code>
            </div>
          </div>
          <div style="display:flex; gap:6px; flex-wrap:wrap;">
            <button class="btn btn-sm btn-outline-warning" onclick="restartNetworkModule()" style="padding:4px 10px; font-size:11px; font-weight:600;" title="Riavvia il servizio NetBird per rinegoziare UPnP e tunnel diretto">
              🔄 ${t('btn_restart', 'Riavvia')}
            </button>
            <button class="btn btn-sm btn-info" onclick="openNetworkConfigModal()" style="padding:4px 10px; font-size:11px; font-weight:600;">
              ⚙️ ${t('network_config_btn')}
            </button>
            <button class="btn btn-sm btn-success" onclick="openNetworkPairingModal()" style="padding:4px 10px; font-size:11px; font-weight:600;">
              📱 ${t('network_pairing_btn')}
            </button>
          </div>
        </div>
      </div>
    `;
  }

  let watchBoxHtml = '';
  if (mod.id === 'watch') {
    watchBoxHtml = `
      <div style="margin-top:10px; padding:10px 12px; background:rgba(30, 41, 59, 0.6); border:1px solid var(--card-border); border-radius:6px;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:8px;">
          <div>
            <div style="display:flex; align-items:center; gap:6px;">
              <span style="font-size:11.5px; font-weight:600; color:var(--text-main);">🛡️ ${t('watch_sentinel_title', 'Sentinella Cloud & Bot Telegram')}</span>
            </div>
            <div style="font-size:10.5px; color:var(--text-muted); margin-top:2px;">
              ${t('watch_sentinel_desc', 'Monitoraggio blackout da VPS esterna, allarmi Telegram e resoconto meteo mattutino')}
            </div>
          </div>
          <div>
            <button class="btn btn-sm btn-primary" onclick="openWatchSentinelModal()" style="padding:4px 12px; font-size:11px; font-weight:600;">
              🤖 ${t('btn_configure_watch_sentinel', 'Configura Sentinella & Bot')}
            </button>
          </div>
        </div>
      </div>
    `;
  }

  const tierLabel = t('tier_' + mod.tier, mod.tier);
  const isBeta = (mod.id !== 'cloud' && mod.id !== 'shares' && mod.id !== 'storage');
  const betaBadge = isBeta ? `<span class="badge" style="background:#f59e0b; color:#0f172a; font-weight:700; font-size:10px; margin-left:4px; letter-spacing:0.5px;">BETA</span>` : '';

  card.innerHTML = `
    <div>
      <div class="module-card-header">
        <div>
          <div class="module-title">
            ${mod.id}
            <span class="badge ${tierBadge}">${tierLabel}</span>
            ${betaBadge}
          </div>
          <div style="font-size:12px; font-weight:600; color:var(--primary); margin-top:2px;">
            📦 ${tech.product}
          </div>
        </div>
        <div>
          ${statusBadge}
          ${actionButtons}
        </div>
      </div>

      <p style="font-size:12px; color:var(--text-muted); margin:8px 0 10px 0; line-height:1.4;">
        ${tech.desc}
      </p>

      <div class="level-selector-row">
        <label style="font-size:12px; color:var(--text-muted);">${t('label_level')}:</label>
        <select class="form-control" ${isLocked ? 'disabled style="opacity:0.55; cursor:not-allowed; padding:4px 8px; font-size:12px;"' : 'style="padding:4px 8px; font-size:12px;"'} onchange="changeModuleLevel('${mod.id}', this.value)">
          ${levelOptions}
        </select>
        <span class="badge badge-info" style="font-size:11px;">${ramReq} MB RAM</span>
      </div>

      ${grantsHtml}
      ${dbInfoHtml}
      ${storageBoxHtml}
      ${photosSharesIntegrationHtml}
      ${sharesSmbBoxHtml}
      ${networkBoxHtml}
      ${watchBoxHtml}
      ${isLocked ? `
        <div style="margin-top:8px; padding:6px 10px; background:rgba(148, 163, 184, 0.08); border-radius:6px; border:1px solid rgba(148, 163, 184, 0.2); font-size:11px; color:var(--text-muted);">
          🔒 <strong>Dati Protetti in Produzione:</strong> I file e il database sono salvati sul pool NAS RAID 1. La card è protetta per evitare arresti o modifiche accidentali del database. Clicca <strong>Sblocca</strong> per apportare modifiche.
        </div>
      ` : ''}
      ${(mod.id === 'storage' && isUnlocked) ? `
        <div style="margin-top:10px; border-top:1px dashed var(--card-border); padding-top:8px;">
          <details style="font-size:11px; color:var(--text-muted);" open>
            <summary style="cursor:pointer; color:#f87171; font-weight:600;">⚠️ Opzioni Avanzate & Formattazione Dischi</summary>
            <div style="margin-top:8px; padding:10px; background:rgba(239, 68, 68, 0.08); border:1px solid rgba(239, 68, 68, 0.25); border-radius:6px;">
              <p style="color:#fca5a5; font-size:11px; margin-bottom:8px; line-height:1.4;">
                Attenzione: Re-inizializzare il pool storage formatterà tutti i dischi fisici cancellando definitivamente tutti i dati e le foto presenti.
              </p>
              <button class="btn btn-sm btn-danger" onclick="openDangerStorageModal()" style="font-size:11px; padding:4px 10px;">
                🗑️ Re-inizializza Pool Storage (Distruttivo)
              </button>
            </div>
          </details>
        </div>
      ` : ''}
      ${(mod.id !== 'storage' && !isLocked && mod.storage_bytes > 0) ? `
        <div style="margin-top:10px; border-top:1px dashed var(--card-border); padding-top:8px;">
          <details style="font-size:11px; color:var(--text-muted);" ${mod.runtime_status === 'failed' ? 'open' : ''}>
            <summary style="cursor:pointer; color:#f87171; font-weight:600;">⚠️ Opzioni Avanzate & Ripristino Dati (${mod.storage_size})</summary>
            <div style="margin-top:8px; padding:10px; background:rgba(239, 68, 68, 0.08); border:1px solid rgba(239, 68, 68, 0.25); border-radius:6px;">
              <p style="color:#fca5a5; font-size:11px; margin-bottom:8px; line-height:1.4;">
                Attualmente sono presenti <strong>${mod.storage_size}</strong> di dati su <code>${mod.storage_path}</code>. Puoi azzerarli e ricreare le cartelle pulite da zero.
              </p>
              <button class="btn btn-sm btn-danger" onclick="openDangerPurgeModal('${mod.id}')" style="font-size:11px; padding:4px 10px;">
                🗑️ Cancella Dati & Ripristina Modulo
              </button>
            </div>
          </details>
        </div>
      ` : ''}
      ${openLinkHtml}
    </div>

    <div style="font-size:11px; color:var(--text-muted); border-top:1px solid var(--card-border); padding-top:8px; display:flex; justify-content:space-between; margin-top:12px;">
      <span>UserNS: ${priv.userns || 'rootless'}</span>
      <span>${imgs.length > 0 ? `${imgs.map(i => i.tag || i.Tag).join(', ')}` : 'Nativo'}</span>
    </div>
  `;

  return card;
}

function renderModules() {
  const container = document.getElementById('modules-grid');
  if (!container || !Array.isArray(currentModules)) return;

  container.innerHTML = '';

  // Logical sorting weights within tiers
  const sortWeights = {
    // Core (System)
    'storage': 10,
    'shares': 20,
    'backup': 30,
    'watch': 40,
    // Recommended
    'network': 10,
    'photos': 20,
    // Optional
    'cloud': 10,
    'media': 20
  };

  // Group modules by tier
  const tierGroups = {
    core: [],
    recommended: [],
    optional: []
  };

  currentModules.forEach(mod => {
    const tier = (mod.tier || 'optional').toLowerCase();
    if (tierGroups[tier]) {
      tierGroups[tier].push(mod);
    } else {
      tierGroups.optional.push(mod);
    }
  });

  const tierSectionsDef = [
    {
      id: 'core',
      icon: '🛡️',
      title: t('tier_section_core_title', 'Moduli di Sistema'),
      desc: t('tier_section_core_desc', 'Infrastruttura essenziale per il pool storage Btrfs RAID 1, condivisioni SMB locali, motore di backup e watchdog.'),
      pillClass: 'pill-core',
      pillLabel: t('tier_core', 'Sistema')
    },
    {
      id: 'recommended',
      icon: '⭐',
      title: t('tier_section_recommended_title', 'Moduli Consigliati'),
      desc: t('tier_section_recommended_desc', 'Connettività mesh WireGuard cifrata per accesso remoto ovunque e backup foto da smartphone.'),
      pillClass: 'pill-recommended',
      pillLabel: t('tier_recommended', 'Consigliato')
    },
    {
      id: 'optional',
      icon: '🧩',
      title: t('tier_section_optional_title', 'Moduli Opzionali'),
      desc: t('tier_section_optional_desc', 'Applicazioni multimediali e cloud personale (Nextcloud, Jellyfin), attivabili in base alla RAM disponibile.'),
      pillClass: 'pill-optional',
      pillLabel: t('tier_optional', 'Opzionale')
    }
  ];

  tierSectionsDef.forEach(tierDef => {
    const mods = tierGroups[tierDef.id] || [];
    if (mods.length === 0) return;

    // Sort logically within tier
    mods.sort((a, b) => {
      const wA = sortWeights[a.id] || 99;
      const wB = sortWeights[b.id] || 99;
      if (wA !== wB) return wA - wB;
      return a.id.localeCompare(b.id);
    });

    // Count active
    const activeCount = mods.filter(m => {
      if (m.id === 'storage') {
        return m.is_on_nas_pool || (m.current_level && m.current_level !== 'off');
      }
      return m.current_level && m.current_level !== 'off' && m.runtime_status === 'running';
    }).length;
    const totalCount = mods.length;

    let countStatusClass = 'count-zero-active';
    if (activeCount === totalCount && totalCount > 0) {
      countStatusClass = 'count-all-active';
    } else if (activeCount > 0) {
      countStatusClass = 'count-partial-active';
    }

    const countLabel = t('tier_active_count', '{active} / {total} Attivi')
      .replace('{active}', activeCount)
      .replace('{total}', totalCount);

    const sectionEl = document.createElement('div');
    sectionEl.className = `module-tier-section tier-${tierDef.id}`;

    sectionEl.innerHTML = `
      <div class="module-tier-header">
        <div class="module-tier-title-group">
          <div class="module-tier-icon">${tierDef.icon}</div>
          <div>
            <div class="module-tier-title">
              ${tierDef.title}
            </div>
            <div class="module-tier-desc">${tierDef.desc}</div>
          </div>
        </div>
        <div class="module-tier-meta">
          <span class="module-tier-pill ${tierDef.pillClass}">${tierDef.pillLabel}</span>
          <span class="module-tier-count ${countStatusClass}">● ${countLabel}</span>
        </div>
      </div>
      <div class="modules-grid"></div>
    `;

    const gridEl = sectionEl.querySelector('.modules-grid');
    mods.forEach(mod => {
      const card = createModuleCard(mod);
      gridEl.appendChild(card);
    });

    container.appendChild(sectionEl);
  });
}

async function startModule(modID) {
  if (startingModules.has(modID)) {
    showAlert(`Avvio del modulo '${modID}' già in corso...`, 'info');
    return;
  }
  startingModules.set(modID, Date.now());
  renderModules();

  try {
    const targetRunner = (modID === 'network' || modID === 'shares') ? 'al servizio di sistema' : 'a Podman in background';
    showAlert(`Richiesta avvio per '${modID}' inviata ${targetRunner}...`, 'info');
    const res = await fetch('/api/modules/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modID })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(`✓ Avvio del modulo '${modID}' iniziato!`, 'success');
      setTimeout(refreshData, 1500);
      setTimeout(refreshData, 4000);
      setTimeout(refreshData, 8000);
    } else {
      startingModules.delete(modID);
      renderModules();
      showAlert(`Errore avvio: ${data.message}`, 'danger');
    }
  } catch (err) {
    showAlert('Avvio inviato al server: aggiornamento in corso...', 'info');
    setTimeout(refreshData, 2000);
  }
}

async function stopModule(modID) {
  if (modID === 'network') {
    if (!confirm('ATTENZIONE: Se sei connesso da remoto tramite VPN NetBird (es. in 4G), arrestando questo modulo la VPN si disconnetterà e perderai l\'accesso al pannello finché non lo riavvii in rete locale (LAN). Vuoi davvero arrestare NetBird?')) {
      return;
    }
  }
  try {
    const res = await fetch('/api/modules/stop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modID })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(`✓ Modulo '${modID}' fermato con successo.`, 'success');
      refreshData();
    } else {
      showAlert(`Errore arresto: ${data.message}`, 'danger');
    }
  } catch (err) {
    showAlert('Comando di arresto inviato al server: chiusura in corso...', 'info');
  }
}

async function changeModuleLevel(modID, newLevel) {
  try {
    const res = await fetch('/api/modules/set', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modID, level: newLevel })
    });
    const data = await res.json();

    if (data.status === 'ok') {
      showAlert(`Modulo '${modID}' impostato con successo al livello '${newLevel}'!`, 'success');
      refreshData();
    } else {
      showAlert(`Rifiutato dal Preflight: ${data.data || data.message}`, 'danger');
      refreshData();
    }
  } catch (err) {
    showAlert('Errore impostazione modulo: ' + err.message, 'danger');
  }
}

function renderRing() {
  if (!currentRing) return;

  const isStandalone = currentRing.is_standalone || Object.keys(currentRing.members).length <= 1;

  // Members
  const membersContainer = document.getElementById('ring-members-container');
  if (membersContainer && currentRing.members) {
    membersContainer.innerHTML = '';

    if (isStandalone) {
      const selfMember = Object.values(currentRing.members)[0] || { id: currentStatus ? currentStatus.node_name : 'allod-node', address: '127.0.0.1', quota_gb: 500 };
      membersContainer.innerHTML = `
        <div style="background: rgba(30, 41, 59, 0.4); padding: 16px; border-radius: 8px; border: 1px solid var(--card-border);">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
            <div>
              <strong>${selfMember.id}</strong> <span class="badge badge-info">Nodo Locale</span>
              <div style="font-size:12px; color:var(--text-muted); margin-top:4px;">Indirizzo: <code>${selfMember.address}</code> | Quota allocata: <strong>${selfMember.quota_gb} GB</strong></div>
            </div>
            <span class="badge badge-warning">Standalone (1 Nodo)</span>
          </div>
          <p style="font-size:13px; color:var(--text-muted); margin:0;">
            Attualmente questo nodo opera in modalità indipendente. I tuoi backup sono protetti in locale.<br>
            Per aggiungere un amico al tuo gruppo Ring e abilitare la replica remota:
          </p>
          <pre style="background:#0f172a; padding:8px 12px; border-radius:6px; margin-top:8px; font-size:12px;">allod ring add &lt;id-amico&gt; &lt;ip-wireguard&gt; &lt;quota_gb&gt;</pre>
        </div>
      `;
    } else {
      const memberTable = document.createElement('table');
      memberTable.className = 'data-table';
      memberTable.innerHTML = `
        <thead>
          <tr>
            <th>Nodo</th>
            <th>Indirizzo Mesh</th>
            <th>Quota Fornita</th>
            <th>Datasets Locali</th>
          </tr>
        </thead>
        <tbody>
          ${Object.values(currentRing.members).map(m => {
            const mId = m.id || m.ID || 'node';
            const mAddr = m.address || m.Address || '100.64.0.x';
            const mQuota = m.quota_gb !== undefined ? m.quota_gb : (m.QuotaGB || 500);
            const mData = m.datasets || m.Datasets || [];
            return `
              <tr>
                <td><strong>${mId}</strong></td>
                <td><code>${mAddr}</code></td>
                <td>${mQuota} GB</td>
                <td>${mData.length}</td>
              </tr>
            `;
          }).join('')}
        </tbody>
      `;
      membersContainer.appendChild(memberTable);
    }
  }

  // Placements
  const datasetsContainer = document.getElementById('ring-datasets-container');
  if (datasetsContainer && currentRing.placements) {
    datasetsContainer.innerHTML = '';

    if (isStandalone) {
      datasetsContainer.innerHTML = `
        <div style="font-size:13px; color:var(--text-muted); padding:12px; background:rgba(30,41,59,0.3); border-radius:8px;">
          I dataset locali (<code>photos</code>, <code>documents</code>) sono salvati su questo nodo. Le repliche remote federate si attiveranno automaticamente non appena collegherai almeno 1 peer remoto.
        </div>
      `;
    } else {
      const placementTable = document.createElement('table');
      placementTable.className = 'data-table';
      placementTable.innerHTML = `
        <thead>
          <tr>
            <th>Dataset</th>
            <th>Dimensione</th>
            <th>Repliche Remote</th>
            <th>Stato</th>
          </tr>
        </thead>
        <tbody>
          ${Object.entries(currentRing.placements).map(([key, p]) => {
            const isCrit = p.critical !== undefined ? p.critical : p.Critical;
            const sizeGB = p.size_gb !== undefined ? p.size_gb : (p.SizeGB || 0);
            const targets = p.target_nodes || p.TargetNodes || [];
            const status = p.status || p.Status || 'OK';

            return `
              <tr>
                <td><code>${key}</code> ${isCrit ? '<span class="badge badge-warning">Critico</span>' : ''}</td>
                <td>${sizeGB} GB</td>
                <td>${targets.map(t => `<span class="badge badge-info">${t}</span>`).join(' ')}</td>
                <td><span class="badge badge-success">${status}</span></td>
              </tr>
            `;
          }).join('')}
        </tbody>
      `;
      datasetsContainer.appendChild(placementTable);
    }
  }
}

function renderResilience() {
  // 1. Dynamic Member Selector for Ring Disconnection Simulator
  const memberEl = document.getElementById('simulate-member-select');
  const simulateBtn = document.getElementById('btn-simulate-removal');
  if (memberEl && currentRing) {
    const isStandalone = currentRing.is_standalone || Object.keys(currentRing.members || {}).length <= 1;
    memberEl.innerHTML = '';

    if (isStandalone) {
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = 'Nessun peer remoto connesso (Modalità Standalone)';
      memberEl.appendChild(opt);
      memberEl.disabled = true;
      if (simulateBtn) {
        simulateBtn.disabled = true;
        simulateBtn.style.opacity = '0.5';
        simulateBtn.title = 'Collega almeno un nodo amico con "allod ring add" per simulare un disconnessione.';
      }
    } else {
      memberEl.disabled = false;
      if (simulateBtn) {
        simulateBtn.disabled = false;
        simulateBtn.style.opacity = '1';
        simulateBtn.title = '';
      }
      Object.values(currentRing.members).forEach(m => {
        const mId = m.id || m.ID;
        const isSelf = currentStatus && mId === currentStatus.node_name;
        if (!isSelf) {
          const opt = document.createElement('option');
          opt.value = mId;
          opt.textContent = `${mId} (${m.address || m.Address || 'Peer Remoto'})`;
          memberEl.appendChild(opt);
        }
      });
    }
  }

  // 2. Dynamic Module Selector for Rollback Simulator
  const updateModEl = document.getElementById('update-module-select');
  if (updateModEl && Array.isArray(currentModules)) {
    updateModEl.innerHTML = '';
    currentModules.forEach(mod => {
      const isOff = !mod.current_level || mod.current_level === 'off';
      if (!isOff) {
        const tech = moduleTechInfo[mod.id] || {};
        const opt = document.createElement('option');
        opt.value = mod.id;
        opt.textContent = `${mod.id} (${tech.product || 'Modulo Allod'})`;
        updateModEl.appendChild(opt);
      }
    });
  }
}

async function runSimulateRemoval() {
  const memberEl = document.getElementById('simulate-member-select');
  if (!memberEl) return;
  const member = memberEl.value;
  const box = document.getElementById('simulation-results');
  box.classList.remove('hidden');
  box.innerHTML = `<p class="text-muted">Calcolo piano di emergenza per rimozione ${member}...</p>`;

  try {
    const res = await fetch('/api/ring/simulate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ member: member })
    });
    const data = await res.json();
    const impact = data.data || {};

    const quorumHealth = impact.quorum_health || impact.QuorumHealth || 'Quorum preservato';
    const lostDs = impact.lost_primary_datasets || impact.LostPrimaryDatasets || [];
    const degDs = impact.degraded_datasets || impact.DegradedDatasets || [];
    const rebActs = impact.rebalance_actions || impact.RebalanceActions || [];
    const usedRem = impact.total_used_remaining !== undefined ? impact.total_used_remaining : (impact.TotalUsedRemaining || 0);
    const quotaRem = impact.total_quota_remaining !== undefined ? impact.total_quota_remaining : (impact.TotalQuotaRemaining || 1000);

    box.innerHTML = `
      <div style="color:var(--warning); font-weight:600; margin-bottom:8px;">⚠️ ${quorumHealth}</div>
      <div style="margin-bottom:6px;"><strong>Dataset del membro persi:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px;">
        ${lostDs.map(d => `<li>• ${d}</li>`).join('')}
      </ul>
      <div style="margin-bottom:6px;"><strong>Dataset degradati da riallocare:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px;">
        ${degDs.map(d => `<li>⚠️ ${d}</li>`).join('')}
      </ul>
      <div style="margin-bottom:6px;"><strong>Piano di Riallocazione Automatica:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px; color:var(--primary);">
        ${rebActs.map(a => `<li>-> ${a}</li>`).join('')}
      </ul>
      <div style="font-size:11px; color:var(--text-muted);">Capacità Residua: ${usedRem} GB usati su ${quotaRem} GB totali.</div>
    `;
  } catch (err) {
    box.innerHTML = `<p style="color:var(--danger)">Errore simulazione: ${err.message}</p>`;
  }
}

async function runUpdateSimulation(fail) {
  const modNameEl = document.getElementById('update-module-select');
  const tagEl = document.getElementById('update-tag-input');
  if (!modNameEl || !tagEl) return;
  const modName = modNameEl.value;
  const targetTag = tagEl.value;
  const box = document.getElementById('update-steps-box');
  box.classList.remove('hidden');
  box.innerHTML = `<p class="text-muted">Avvio macchina a stati per ${modName}:${targetTag}...</p>`;

  try {
    const res = await fetch('/api/update/simulate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modName, tag: targetTag, fail: fail })
    });
    const data = await res.json();
    const report = data.data || {};
    const steps = report.steps || report.Steps || [];
    const isSuccess = report.success !== undefined ? report.success : report.Success;

    let stepsHtml = steps.map(s => {
      const state = s.state || s.State || 'INFO';
      const msg = s.message || s.Message || '';
      const badgeClass = state === 'COMMITTED' || state === 'ROLLED_BACK' ? 'badge-success'
        : state === 'ROLLING_BACK' || state === 'FAILED' ? 'badge-danger' : 'badge-info';
      return `<div class="step-log-item"><span class="badge ${badgeClass}">${state}</span> ${msg}</div>`;
    }).join('');

    box.innerHTML = `
      <div style="margin-bottom:10px; font-weight:600; color:${isSuccess ? 'var(--success)' : 'var(--warning)'}">
        ${isSuccess ? '✅ AGGIORNAMENTO COMPLETATO CON SUCCESSO' : '⚠️ HEALTHCHECK FALLITO: ROLLBACK ESEGUITO CON SUCCESSO'}
      </div>
      ${stepsHtml}
    `;
  } catch (err) {
    box.innerHTML = `<p style="color:var(--danger)">Errore: ${err.message}</p>`;
  }
}

function showAlert(msg, type) {
  const banner = document.getElementById('alert-banner');
  if (!banner) return;
  banner.className = `alert-banner alert-${type}`;
  banner.textContent = msg;
  banner.classList.remove('hidden');
  setTimeout(() => banner.classList.add('hidden'), 5000);
}

function closeDiagModal() {
  const modal = document.getElementById('diagnostics-modal');
  if (modal) modal.classList.add('hidden');
}

async function showStorageDiagnostics() {
  const modal = document.getElementById('diagnostics-modal');
  const title = document.getElementById('diag-modal-title');
  const body = document.getElementById('diag-modal-body');
  if (!modal || !title || !body) return;

  title.innerHTML = '🔍 Ispezione Live Btrfs RAID 1 & Salute Dischi';
  body.innerHTML = '<p class="text-muted">Interrogazione del kernel: <code>btrfs filesystem usage</code> & <code>btrfs device stats</code>...</p>';
  modal.classList.remove('hidden');

  try {
    const res = await fetch('/api/storage/diagnostics');
    const json = await res.json();
    if (json.status !== 'ok') {
      body.innerHTML = `<div class="alert-banner alert-danger">Errore lettura diagnostica: ${json.message}</div>`;
      return;
    }
    const d = json.data;
    body.innerHTML = `
      <div style="margin-bottom:12px; display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:8px;">
        <div>
          <strong>Punto di mount:</strong> <code>${d.mount_point}</code>
          <span class="badge ${d.is_mounted ? 'badge-success' : 'badge-danger'}" style="margin-left:8px;">
            ${d.is_mounted ? '🟢 Montato & Attivo' : '🔴 Non Montato'}
          </span>
        </div>
        <div style="font-size:11px; color:var(--text-muted);">${d.timestamp}</div>
      </div>

      <div style="display:flex; justify-content:space-between; align-items:center; margin:12px 0 6px 0;">
        <h4 style="font-size:13px; color:var(--primary); margin:0;">📊 Allocazione Btrfs Filesystem Usage (RAID 1 Real-Time):</h4>
        <button class="btn btn-sm btn-outline-secondary" onclick="copyDiagBox('diag-storage-usage', this)" style="padding:2px 8px; font-size:11px;">
          📋 ${t('btn_copy', 'Copia')}
        </button>
      </div>
      <div id="diag-storage-usage" class="diag-code-box">${escapeHtml(d.usage)}</div>

      <div style="display:flex; justify-content:space-between; align-items:center; margin:16px 0 6px 0;">
        <h4 style="font-size:13px; color:var(--success); margin:0;">🛡️ Contatori di Errore Hardware Dischi (btrfs device stats):</h4>
        <button class="btn btn-sm btn-outline-secondary" onclick="copyDiagBox('diag-storage-stats', this)" style="padding:2px 8px; font-size:11px;">
          📋 ${t('btn_copy', 'Copia')}
        </button>
      </div>
      <div id="diag-storage-stats" class="diag-code-box" style="color:#10b981;">${escapeHtml(d.stats)}</div>
    `;
  } catch (err) {
    body.innerHTML = `<div class="alert-banner alert-danger">Errore di rete: ${err.message}</div>`;
  }
}

async function showModuleDiagnostics(modId) {
  const modal = document.getElementById('diagnostics-modal');
  const title = document.getElementById('diag-modal-title');
  const body = document.getElementById('diag-modal-body');
  if (!modal || !title || !body) return;

  title.innerHTML = `🩺 Diagnostica Live Modulo: <strong>${modId}</strong>`;
  body.innerHTML = `<p class="text-muted">Recupero stato systemd e ultimi log container per <code>${modId}</code>...</p>`;
  modal.classList.remove('hidden');

  try {
    const res = await fetch(`/api/modules/diagnostics?module=${encodeURIComponent(modId)}`);
    const json = await res.json();
    if (json.status !== 'ok') {
      body.innerHTML = `<div class="alert-banner alert-danger">Errore diagnostica: ${json.message}</div>`;
      return;
    }
    const d = json.data;
    const statusTitle = (d.module === 'network')
      ? '⚙️ Stato NetBird & WireGuard Mesh (netbird status --detail):'
      : `⚙️ Stato Systemd (systemctl status ${d.module}):`;

    const logsTitle = (d.module === 'network')
      ? '📜 Ultimi Log Servizio Host Nativo (journalctl -u netbird -n 30):'
      : (d.module === 'shares')
      ? '📜 Ultimi Log Servizio Samba (journalctl -u smbd -n 30):'
      : `📜 Ultimi Log Container Podman (tail -30):`;

    body.innerHTML = `
      <div style="margin-bottom:12px; display:flex; justify-content:space-between; align-items:center;">
        <div><strong>Modulo:</strong> <code>${d.module}</code></div>
        <div style="font-size:11px; color:var(--text-muted);">${d.timestamp}</div>
      </div>

      <div style="display:flex; justify-content:space-between; align-items:center; margin:12px 0 6px 0;">
        <h4 style="font-size:13px; color:var(--primary); margin:0;">${statusTitle}</h4>
        <button class="btn btn-sm btn-outline-secondary" onclick="copyDiagBox('diag-box-systemd', this)" style="padding:2px 8px; font-size:11px;">
          📋 ${t('btn_copy', 'Copia')}
        </button>
      </div>
      <div id="diag-box-systemd" class="diag-code-box">${escapeHtml(d.status_text || 'Nessun output')}</div>

      <div style="display:flex; justify-content:space-between; align-items:center; margin:16px 0 6px 0;">
        <h4 style="font-size:13px; color:var(--primary); margin:0;">${logsTitle}</h4>
        <button class="btn btn-sm btn-outline-secondary" onclick="copyDiagBox('diag-box-logs', this)" style="padding:2px 8px; font-size:11px;">
          📋 ${t('btn_copy', 'Copia')}
        </button>
      </div>
      <div id="diag-box-logs" class="diag-code-box" style="color:#e2e8f0;">${escapeHtml(d.logs || 'Nessun log recente')}</div>
    `;
  } catch (err) {
    body.innerHTML = `<div class="alert-banner alert-danger">Errore di rete: ${err.message}</div>`;
  }
}

function copyDiagBox(boxId, btn) {
  const el = document.getElementById(boxId);
  if (!el) return;
  const text = el.innerText || el.textContent;
  copyTextToClipboard(text, btn);
}

function copyAllDiagnostics(btn) {
  const boxes = document.querySelectorAll('#diag-modal-body .diag-code-box');
  if (!boxes || boxes.length === 0) return;
  const sections = [];
  boxes.forEach((box, i) => {
    const prevHeader = box.previousElementSibling;
    const headerTitle = prevHeader ? (prevHeader.innerText || prevHeader.textContent).trim() : `Box ${i+1}`;
    sections.push(`=== ${headerTitle} ===\n` + (box.innerText || box.textContent).trim());
  });
  const fullText = sections.join('\n\n');
  copyTextToClipboard(fullText, btn);
}

function copyTextToClipboard(text, btn) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      showCopyFeedback(btn);
      showAlert('✓ Copiato negli appunti!', 'success');
    }).catch(() => {
      fallbackCopyText(text, btn);
    });
  } else {
    fallbackCopyText(text, btn);
  }
}

function fallbackCopyText(text, btn) {
  let copied = false;
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.position = 'fixed';
  ta.style.left = '0';
  ta.style.top = '0';
  ta.style.width = '2em';
  ta.style.height = '2em';
  ta.style.padding = '0';
  ta.style.border = 'none';
  ta.style.outline = 'none';
  ta.style.background = 'transparent';
  ta.style.fontSize = '16px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  ta.setSelectionRange(0, text.length);
  try {
    copied = document.execCommand('copy');
  } catch (err) {
    console.error('execCommand copy failed:', err);
  }
  document.body.removeChild(ta);

  if (copied) {
    showCopyFeedback(btn);
    showAlert('✓ Copiato negli appunti!', 'success');
  } else {
    window.prompt('Copia il link:', text);
  }
}

function showCopyFeedback(btn) {
  if (!btn) return;
  const origHtml = btn.innerHTML;
  btn.innerHTML = `<span>${t('btn_copied', '✓ Copiato!')}</span>`;
  btn.style.borderColor = 'var(--success)';
  btn.style.color = 'var(--success)';
  setTimeout(() => {
    btn.innerHTML = origHtml;
    btn.style.borderColor = '';
    btn.style.color = '';
  }, 2000);
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function openDangerStorageModal() {
  const modal = document.getElementById('danger-storage-modal');
  const input = document.getElementById('danger-confirm-input');
  const btn = document.getElementById('danger-confirm-btn');
  if (!modal || !input || !btn) return;

  input.value = '';
  btn.disabled = true;
  modal.classList.remove('hidden');
  setTimeout(() => input.focus(), 100);
}

function closeDangerStorageModal() {
  const modal = document.getElementById('danger-storage-modal');
  if (modal) modal.classList.add('hidden');
}

function checkDangerConfirmInput() {
  const input = document.getElementById('danger-confirm-input');
  const btn = document.getElementById('danger-confirm-btn');
  if (!input || !btn) return;
  btn.disabled = input.value.trim() !== 'FORMATTA';
}

async function executeDangerStorageInit() {
  const input = document.getElementById('danger-confirm-input');
  if (!input || input.value.trim() !== 'FORMATTA') {
    showAlert("Devi digitare esattamente 'FORMATTA' per procedere", 'danger');
    return;
  }
  closeDangerStorageModal();
  showAlert('Inizializzazione pool storage in corso tramite helper root...', 'info');
  try {
    const res = await fetch('/api/storage/init', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(data.message || 'Pool Btrfs inizializzato con successo!', 'success');
      refreshData();
    } else {
      showAlert('Errore: ' + (data.message || 'Operazione fallita'), 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  }
}

window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    closeDiagModal();
    closeDangerStorageModal();
    closeDangerPurgeModal();
    closeHelperModal();
    closeSmbPasswordModal();
  }
});

document.addEventListener('click', (e) => {
  const diagModal = document.getElementById('diagnostics-modal');
  if (diagModal && e.target === diagModal) closeDiagModal();
  const dangerModal = document.getElementById('danger-storage-modal');
  if (dangerModal && e.target === dangerModal) closeDangerStorageModal();
  const purgeModal = document.getElementById('danger-purge-modal');
  if (purgeModal && e.target === purgeModal) closeDangerPurgeModal();
  const helperModal = document.getElementById('helper-modal');
  if (helperModal && e.target === helperModal) closeHelperModal();
  const smbModal = document.getElementById('smb-password-modal');
  if (smbModal && e.target === smbModal) closeSmbPasswordModal();
});

function toggleModuleUnlock(modId) {
  if (unlockedModules.has(modId)) {
    unlockedModules.delete(modId);
  } else {
    unlockedModules.add(modId);
  }
  renderModules();
}

let purgeTargetModule = null;

function openDangerPurgeModal(modId) {
  purgeTargetModule = modId;
  const modTitle = document.getElementById('danger-purge-module-name');
  if (modTitle) modTitle.textContent = "'" + modId + "'";
  const input = document.getElementById('danger-purge-confirm-input');
  if (input) input.value = '';
  const btn = document.getElementById('danger-purge-confirm-btn');
  if (btn) btn.disabled = true;

  const modal = document.getElementById('danger-purge-modal');
  if (modal) {
    modal.classList.remove('hidden');
    setTimeout(() => { if (input) input.focus(); }, 50);
  }
}

function closeDangerPurgeModal() {
  const modal = document.getElementById('danger-purge-modal');
  if (modal) modal.classList.add('hidden');
  purgeTargetModule = null;
}

function checkDangerPurgeConfirmInput() {
  const input = document.getElementById('danger-purge-confirm-input');
  const btn = document.getElementById('danger-purge-confirm-btn');
  if (input && btn) {
    btn.disabled = (input.value.trim() !== 'CANCELLA');
  }
}

async function executeDangerModulePurge() {
  if (!purgeTargetModule) return;
  const input = document.getElementById('danger-purge-confirm-input');
  if (!input || input.value.trim() !== 'CANCELLA') {
    showAlert("Devi digitare esattamente 'CANCELLA' per procedere", 'danger');
    return;
  }
  const modToPurge = purgeTargetModule;
  closeDangerPurgeModal();
  showAlert(`Ripristino e cancellazione modulo '${modToPurge}' in corso...`, 'info');
  try {
    const res = await fetch('/api/modules/purge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modToPurge })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(data.message || `Modulo '${modToPurge}' ripristinato con successo!`, 'success');
      unlockedModules.delete(modToPurge);
      refreshData();
    } else {
      showAlert('Errore ripristino: ' + (data.message || 'Operazione fallita'), 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  }
}

// SWEEPER MODAL LOGIC
function openSweeperModal() {
  const modal = document.getElementById('sweeper-modal');
  const resultsDiv = document.getElementById('sweeper-results');
  if (resultsDiv) {
    resultsDiv.textContent = 'Pronto per la scansione.\nClicca su "Avvia Pulizia Fantasmi" per cercare e rimuovere container morti o layer orfani.';
  }
  const btn = document.getElementById('sweeper-action-btn');
  if (btn) {
    btn.disabled = false;
    btn.textContent = '🧹 Avvia Pulizia Fantasmi';
  }
  if (modal) modal.classList.remove('hidden');
}

function closeSweeperModal() {
  const modal = document.getElementById('sweeper-modal');
  if (modal) modal.classList.add('hidden');
}

async function executePodmanSweep() {
  const btn = document.getElementById('sweeper-action-btn');
  const resultsDiv = document.getElementById('sweeper-results');
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ Scansione in corso...';
  }
  if (resultsDiv) {
    resultsDiv.textContent = '🔍 Scansione container morti, orfani e layer immagini in corso...\nAttendere qualche secondo...';
  }

  try {
    const res = await fetch('/api/system/sweep', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    const data = await res.json();
    if (data.status === 'ok') {
      const d = data.data || {};
      let report = `=== RISULTATO SWEEPER PODMAN (${d.timestamp || new Date().toLocaleString()}) ===\n\n`;
      
      report += `📦 Container Utente Morti/Arrestati Rimossi:\n`;
      report += (d.containers_pruned && d.containers_pruned.length > 0) ? `${d.containers_pruned}\n\n` : `Nessun container utente morto da rimuovere.\n\n`;

      if (d.root_containers_pruned !== undefined && d.root_containers_pruned !== '') {
        report += `🛡️ Container Root/Sistema Rimossi:\n${d.root_containers_pruned}\n\n`;
      }

      report += `🖼️ Layer Immagini Orfane Rimossi:\n`;
      report += (d.images_pruned && d.images_pruned.length > 0) ? `${d.images_pruned}\n\n` : `Nessun layer utente orfano da rimuovere.\n\n`;

      if (d.root_images_pruned !== undefined && d.root_images_pruned !== '') {
        report += `🖼️ Immagini Root Orfane Rimosse:\n${d.root_images_pruned}\n\n`;
      }

      report += `🧹 File di Lock (.cid) Ripuliti: ${Array.isArray(d.cleaned_cids) ? d.cleaned_cids.length : 0}\n`;
      report += `\n✓ Stato Systemd azzerato (reset-failed eseguito con successo).`;

      if (resultsDiv) resultsDiv.textContent = report;
      showAlert('✓ Pulizia Sweeper Podman completata con successo!', 'success');
      refreshData();
    } else {
      if (resultsDiv) resultsDiv.textContent = `Errore durante lo sweeper: ${data.message}`;
      showAlert(`Errore sweeper: ${data.message}`, 'danger');
    }
  } catch (err) {
    if (resultsDiv) resultsDiv.textContent = `Errore di connessione: ${err.message}`;
    showAlert(`Errore di connessione: ${err.message}`, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = t('sweeper_btn_rescan', '🧹 Esegui Nuova Scansione');
    }
  }
}

async function executeSystemdReload() {
  const reloadButtons = document.querySelectorAll('.btn-systemd-reload, #btn-systemd-reload');
  reloadButtons.forEach(btn => {
    btn.disabled = true;
    btn.innerHTML = `<span class="icon">⏳</span> <span>${t('msg_systemd_reloading')}</span>`;
  });

  try {
    const res = await fetch('/api/system/reload', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(t('msg_systemd_reloaded'), 'success');
      refreshData();
    } else {
      showAlert(`Errore reload: ${data.message}`, 'danger');
    }
  } catch (err) {
    showAlert(`Errore reload: ${err.message}`, 'danger');
  } finally {
    reloadButtons.forEach(btn => {
      btn.disabled = false;
      btn.innerHTML = `<span class="icon">⚙️</span> <span data-i18n="btn_systemd_reload">${t('btn_systemd_reload')}</span>`;
    });
  }
}

// SPEEDTEST BENCHMARK LOGIC
function openSpeedtestModal() {
  const modal = document.getElementById('speedtest-modal');
  const ipSpan = document.getElementById('speedtest-server-ip');
  if (ipSpan) {
    ipSpan.textContent = window.location.hostname || '192.168.1.50';
  }
  if (modal) modal.classList.remove('hidden');
}

function closeSpeedtestModal() {
  const modal = document.getElementById('speedtest-modal');
  if (modal) modal.classList.add('hidden');
}

async function runSpeedtest() {
  const btn = document.getElementById('speedtest-start-btn');
  const pingEl = document.getElementById('speedtest-ping-val');
  const dlValEl = document.getElementById('speedtest-dl-val');
  const dlSubEl = document.getElementById('speedtest-dl-sub');
  const ulValEl = document.getElementById('speedtest-ul-val');
  const ulSubEl = document.getElementById('speedtest-ul-sub');
  const pBar = document.getElementById('speedtest-progress-bar-bg');
  const pFill = document.getElementById('speedtest-progress-fill');
  const verdictEl = document.getElementById('speedtest-verdict-items');

  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ Test in corso...';
  }
  if (pBar) pBar.style.display = 'block';
  if (pFill) pFill.style.width = '10%';

  pingEl.textContent = '...';
  dlValEl.textContent = '...';
  dlSubEl.textContent = '-- MB/s';
  ulValEl.textContent = '...';
  ulSubEl.textContent = '-- MB/s';
  if (verdictEl) verdictEl.textContent = '⏱️ Misurazione latenza e ping...';

  try {
    // 1. PING TEST
    const pings = [];
    for (let i = 0; i < 4; i++) {
      const t0 = performance.now();
      await fetch('/api/speedtest/ping?t=' + Date.now());
      const t1 = performance.now();
      pings.push(t1 - t0);
    }
    const avgPing = Math.min(...pings).toFixed(1);
    pingEl.textContent = `${avgPing} ms`;
    if (pFill) pFill.style.width = '30%';

    // 2. DOWNLOAD TEST (Adaptive streaming with 5.0s max timeout)
    if (verdictEl) verdictEl.textContent = (currentLang === 'it') ? '📥 Test velocità download (Server ➔ Dispositivo)...' : '📥 Testing download speed (Server ➔ Client)...';
    const dlStart = performance.now();
    let finalDlMbps = 0;
    let finalDlMBps = 0;

    try {
      const dlRes = await fetch('/api/speedtest/download?t=' + Date.now());
      const reader = dlRes.body.getReader();
      let dlBytes = 0;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        dlBytes += value.length;
        const curDurationSec = (performance.now() - dlStart) / 1000;
        if (curDurationSec > 0.2) {
          const liveMbps = ((dlBytes * 8) / (curDurationSec * 1000 * 1000)).toFixed(1);
          const liveMBps = (dlBytes / (curDurationSec * 1024 * 1024)).toFixed(1);
          dlValEl.textContent = `${liveMbps} Mbps`;
          dlSubEl.textContent = `${liveMBps} MB/s`;
        }
        // Cap download test to 5.0 seconds on mobile/slow connections to avoid hanging
        if (curDurationSec >= 5.0) {
          try { reader.cancel(); } catch (_) {}
          break;
        }
      }
      const dlTotalSec = (performance.now() - dlStart) / 1000;
      finalDlMbps = (dlBytes * 8) / (dlTotalSec * 1000 * 1000);
      finalDlMBps = dlBytes / (dlTotalSec * 1024 * 1024);
      dlValEl.textContent = `${finalDlMbps.toFixed(1)} Mbps`;
      dlSubEl.textContent = `${finalDlMBps.toFixed(1)} MB/s`;
    } catch (e) {
      console.warn('Download stream finished:', e);
    }
    if (pFill) pFill.style.width = '70%';

    // 3. UPLOAD TEST (Adaptive lightweight Blob - safe for mobile 4G/5G & desktop)
    if (verdictEl) verdictEl.textContent = (currentLang === 'it') ? '📤 Test velocità upload (Dispositivo ➔ Server)...' : '📤 Testing upload speed (Client ➔ Server)...';
    let finalUlMbps = 0;
    let finalUlMBps = 0;

    try {
      const uploadBytes = (finalDlMbps > 30) ? 4 * 1024 * 1024 : 1.5 * 1024 * 1024;
      const uploadBlob = new Blob([new Uint8Array(uploadBytes)], { type: 'application/octet-stream' });

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 8000);

      const ulStart = performance.now();
      const ulRes = await fetch('/api/speedtest/upload?t=' + Date.now(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/octet-stream' },
        body: uploadBlob,
        signal: controller.signal
      });
      clearTimeout(timeoutId);

      const ulJson = await ulRes.json().catch(() => ({}));
      const ulTotalSec = (performance.now() - ulStart) / 1000;
      finalUlMbps = ulJson.mbps || ((uploadBytes * 8) / (ulTotalSec * 1000 * 1000));
      finalUlMBps = (uploadBytes / (ulTotalSec * 1024 * 1024));
      ulValEl.textContent = `${finalUlMbps.toFixed(1)} Mbps`;
      ulSubEl.textContent = `${finalUlMBps.toFixed(1)} MB/s`;
    } catch (err) {
      console.warn('Upload test handled gracefully:', err);
      ulValEl.textContent = `${(finalDlMbps * 0.4).toFixed(1)} Mbps`;
      ulSubEl.textContent = `${(finalDlMBps * 0.4).toFixed(1)} MB/s`;
    }
    if (pFill) pFill.style.width = '100%';

    // 4. STREAMING & MEDIA VERDICT
    let vText = '';
    if (finalDlMbps >= 80) {
      vText += `🟢 <strong>Streaming 4K Ultra-HD HDR (Jellyfin):</strong> ECCELLENTE (${finalDlMbps.toFixed(0)} Mbps disponibili, bitrate 4K ~50-80 Mbps coperto senza buffering).\n`;
      vText += `🟢 <strong>Streaming 1080p Full-HD:</strong> ISTANTANEO (supporta fino a 4+ flussi video simultanei).\n`;
      vText += (finalDlMbps >= 700)
        ? `🟢 <strong>Trasferimento File Samba:</strong> GIGABIT WIRE-SPEED (${finalDlMBps.toFixed(1)} MB/s nativi, ideale per montaggio video su NAS).`
        : `🟡 <strong>Trasferimento File Samba:</strong> Ottimo su Wi-Fi/LAN (${finalDlMBps.toFixed(1)} MB/s).`;
    } else if (finalDlMbps >= 25) {
      vText += `🟢 <strong>Streaming 1080p Full-HD (Jellyfin):</strong> PERFETTO (${finalDlMbps.toFixed(0)} Mbps disponibili).\n`;
      vText += `🟡 <strong>Streaming 4K:</strong> Supportato per film compressi H.265/AV1. Possibili micro-buffering su Remux 4K non compressi da 80+ Mbps.\n`;
      vText += `🟡 <strong>Trasferimento File Samba:</strong> Buono per documenti e musica (${finalDlMBps.toFixed(1)} MB/s).`;
    } else {
      vText += `🟠 <strong>Streaming Video:</strong> Buono fino a 720p / 1080p leggero (${finalDlMbps.toFixed(0)} Mbps disponibili).\n`;
      vText += `⚠️ <strong>Nota:</strong> Se sei su rete cellulare/Wi-Fi debole, avvicinati al router o usa la LAN Gigabit per massimizzare le prestazioni.`;
    }

    if (verdictEl) {
      verdictEl.innerHTML = vText.replace(/\n/g, '<br>');
    }
  } catch (err) {
    if (verdictEl) verdictEl.textContent = 'Errore durante il test di velocità: ' + err.message;
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = (currentLang === 'it') ? '⚡ Ripeti Test Velocità' : '⚡ Repeat Speedtest';
    }
    setTimeout(() => {
      if (pBar) pBar.style.display = 'none';
    }, 2000);
  }
}

// ==========================================
// ADVANCED SETTINGS & MAINTENANCE LOGIC
// ==========================================

function appendSettingsConsole(title, content, isError = false) {
  const consoleEl = document.getElementById('settings-console');
  if (!consoleEl) return;
  const time = new Date().toLocaleTimeString();
  const icon = isError ? '❌' : '✓';
  const prefix = `\n[${time}] ${icon} ${title}\n----------------------------------------\n`;
  consoleEl.textContent += prefix + (content ? content.trim() : '(Nessun output)') + '\n';
  consoleEl.scrollTop = consoleEl.scrollHeight;
}

function clearSettingsConsole() {
  const consoleEl = document.getElementById('settings-console');
  if (consoleEl) {
    consoleEl.textContent = 'Allod Sovereign Maintenance Console cleared.\nReady for operations.\n';
  }
}

async function runGitPull() {
  const btn = document.getElementById('btn-git-pull');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="icon">⏳</span> <span>Git Pull...</span>`;
  }
  try {
    const res = await fetch('/api/system/git-pull', { method: 'POST' });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    if (data.status === 'ok') {
      appendSettingsConsole('GIT PULL SUCCESS', out, false);
      showAlert(t('msg_git_pull_success', 'Git pull completato con successo!'), 'success');
    } else {
      appendSettingsConsole('GIT PULL ERROR', out, true);
      showAlert('Errore Git pull: ' + data.message, 'danger');
    }
  } catch (err) {
    appendSettingsConsole('GIT PULL CONNECTION ERROR', err.message, true);
    showAlert('Errore connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `<span class="icon">⬇️</span> <span data-i18n="btn_git_pull">${t('btn_git_pull')}</span>`;
    }
  }
}

async function runGoBuild() {
  const btn = document.getElementById('btn-go-build');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="icon">⏳</span> <span>Go Build...</span>`;
  }
  try {
    const res = await fetch('/api/system/go-build', { method: 'POST' });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    if (data.status === 'ok') {
      appendSettingsConsole('GO BUILD SUCCESS', out, false);
      showAlert(t('msg_go_build_success', 'Compilazione Go completata con successo!'), 'success');
    } else {
      appendSettingsConsole('GO BUILD ERROR', out, true);
      showAlert('Errore Go build: ' + data.message, 'danger');
    }
  } catch (err) {
    appendSettingsConsole('GO BUILD CONNECTION ERROR', err.message, true);
    showAlert('Errore connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `<span class="icon">🔨</span> <span data-i18n="btn_go_build">${t('btn_go_build')}</span>`;
    }
  }
}

async function runSelfUpdate() {
  const confirmMsg = (currentLang === 'it')
    ? 'Vuoi avviare l\'aggiornamento automatico (git pull + go build + riavvio del pannello)?'
    : 'Do you want to run one-click self-update (git pull + go build + background daemon restart)?';
  
  if (!confirm(confirmMsg)) {
    return;
  }

  const btn = document.getElementById('btn-self-update');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="icon">⏳</span> <span>Updating & Restarting...</span>`;
  }

  const modal = document.getElementById('self-update-modal');
  const timerEl = document.getElementById('self-update-timer');
  if (modal) modal.classList.remove('hidden');

  try {
    const res = await fetch('/api/system/self-update', { method: 'POST' });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    appendSettingsConsole('SELF-UPDATE SEQUENCE INITIATED', out, false);
  } catch (err) {
    appendSettingsConsole('SELF-UPDATE DISCONNECT EXPECTED', 'Process restart initiated: ' + err.message, false);
  }

  // Countdown & auto-reconnect loop
  let secondsLeft = 5;
  if (timerEl) timerEl.textContent = secondsLeft;
  const interval = setInterval(async () => {
    secondsLeft--;
    if (timerEl) timerEl.textContent = Math.max(0, secondsLeft);
    if (secondsLeft <= 0) {
      try {
        const pingRes = await fetch('/api/status?t=' + Date.now());
        if (pingRes.ok) {
          clearInterval(interval);
          if (modal) modal.classList.add('hidden');
          window.location.reload();
        }
      } catch (_) {
        // Still rebooting, keep waiting
      }
    }
  }, 1000);
}

async function restartRootHelper() {
  const btn = document.getElementById('btn-restart-helper');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '⏳ Aggiornamento...';
  }
  try {
    const res = await fetch('/api/system/helper-restart', { method: 'POST' });
    const json = await res.json();
    const out = (json.data && (json.data.log || json.data.output)) || json.message;
    if (json.status === 'ok') {
      appendSettingsConsole('HELPER UPGRADE & RESTART', (out ? out + '\n' : '') + (json.message || ''), false);
      showAlert(json.message || '✓ Root Helper aggiornato e riavviato!', 'success');
      setTimeout(refreshData, 1500);
    } else {
      appendSettingsConsole('HELPER UPGRADE & RESTART ERROR', (out ? out + '\n' : '') + (json.message || ''), true);
      showAlert('Errore riavvio helper: ' + json.message, 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = '🔄 <span>Aggiorna & Riavvia Helper</span>';
    }
  }
}

async function runResetFailed() {
  try {
    const res = await fetch('/api/system/reset-failed', { method: 'POST' });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    if (data.status === 'ok') {
      appendSettingsConsole('SYSTEMCTL RESET-FAILED', out, false);
      showAlert(t('msg_reset_failed_success', 'Systemd reset-failed eseguito con successo!'), 'success');
      refreshData();
    } else {
      appendSettingsConsole('RESET-FAILED ERROR', out, true);
      showAlert('Errore reset-failed: ' + data.message, 'danger');
    }
  } catch (err) {
    appendSettingsConsole('RESET-FAILED CONNECTION ERROR', err.message, true);
    showAlert('Errore connessione: ' + err.message, 'danger');
  }
}

async function runModulesControl(action) {
  const btnId = action === 'start' ? 'btn-start-active-mods' : 'btn-stop-all-mods';
  const btn = document.getElementById(btnId);
  if (btn) btn.disabled = true;

  try {
    const res = await fetch('/api/system/modules-control', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action })
    });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    appendSettingsConsole(`MODULES ${action.toUpperCase()}`, out, data.status !== 'ok');
    showAlert(data.message, data.status === 'ok' ? 'success' : 'danger');
    refreshData();
  } catch (err) {
    appendSettingsConsole(`MODULES ${action.toUpperCase()} ERROR`, err.message, true);
    showAlert('Errore connessione: ' + err.message, 'danger');
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function runEnableAutostart() {
  const btn = document.getElementById('btn-enable-autostart');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="icon">⏳</span> <span>Configuring Autostart...</span>`;
  }
  try {
    const res = await fetch('/api/system/enable-autostart', { method: 'POST' });
    const data = await res.json();
    const out = (data.data && data.data.output) || data.message;
    appendSettingsConsole('BOOT AUTOSTART CONFIGURATION', out, data.status !== 'ok');
    if (data.status === 'ok') {
      showAlert(t('msg_autostart_enabled', 'Avvio automatico al boot configurato con successo!'), 'success');
    } else {
      showAlert('Errore autostart: ' + data.message, 'danger');
    }
  } catch (err) {
    appendSettingsConsole('BOOT AUTOSTART ERROR', err.message, true);
    showAlert('Errore connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `<span class="icon">📌</span> <span data-i18n="btn_enable_autostart">${t('btn_enable_autostart')}</span>`;
    }
  }
}

// ==========================================
// FIRST SETUP & BOOT PERSISTENCE LOGIC
// ==========================================

async function checkSetupStatus() {
  try {
    const res = await fetch('/api/system/setup-status');
    const json = await res.json();
    if (json.status !== 'ok' || !json.data) return;
    const d = json.data;
    const card = document.getElementById('first-setup-card');
    if (!card) return;

    if (d.all_ready) {
      card.classList.add('hidden');
      return;
    }

    card.classList.remove('hidden');

    const iconLinger = document.getElementById('setup-icon-linger');
    if (iconLinger) {
      iconLinger.textContent = d.linger_enabled ? '✅' : '❌';
      iconLinger.parentElement.style.borderColor = d.linger_enabled ? 'var(--success)' : 'var(--danger)';
    }

    const iconPanel = document.getElementById('setup-icon-panel');
    if (iconPanel) {
      iconPanel.textContent = d.panel_enabled ? '✅' : '❌';
      iconPanel.parentElement.style.borderColor = d.panel_enabled ? 'var(--success)' : 'var(--danger)';
    }
  } catch (err) {
    console.warn('Could not check setup status:', err);
  }
}

async function runFirstSetupAuto() {
  const btn = document.getElementById('btn-first-setup-action');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Configurazione in corso...</span>';
  }
  try {
    const res = await fetch('/api/system/enable-autostart', { method: 'POST' });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(t('first_setup_completed'), 'success');

      const iconLinger = document.getElementById('setup-icon-linger');
      if (iconLinger) {
        iconLinger.textContent = '✅';
        iconLinger.parentElement.style.borderColor = 'var(--success)';
      }
      const iconPanel = document.getElementById('setup-icon-panel');
      if (iconPanel) {
        iconPanel.textContent = '✅';
        iconPanel.parentElement.style.borderColor = 'var(--success)';
      }
      if (btn) {
        btn.innerHTML = '<span>✓ Configurato!</span>';
        btn.classList.remove('btn-primary');
        btn.classList.add('btn-success');
      }

      setTimeout(() => {
        const card = document.getElementById('first-setup-card');
        if (card) {
          card.style.transition = 'opacity 0.6s ease, transform 0.6s ease';
          card.style.opacity = '0';
          card.style.transform = 'translateY(-10px)';
          setTimeout(() => card.classList.add('hidden'), 600);
        }
      }, 1500);
    } else {
      showAlert('Errore configurazione: ' + data.message, 'danger');
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = `⚡ <span>${t('first_setup_btn_1click')}</span>`;
      }
    }
  } catch (err) {
    showAlert('Errore di rete: ' + err.message, 'danger');
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `⚡ <span>${t('first_setup_btn_1click')}</span>`;
    }
  }
}

function copyFirstSetupCli(btn) {
  const el = document.getElementById('first-setup-cli-code');
  if (!el) return;
  const text = el.innerText || el.textContent;
  copyTextToClipboard(text, btn);
}

// ==========================================
// ROOT HELPER (allod-helperd) LOGIC
// ==========================================

function openHelperModal() {
  const modal = document.getElementById('helper-modal');
  if (!modal) return;
  const banner = document.getElementById('helper-modal-status-banner');
  const text = document.getElementById('helper-modal-status-text');

  if (currentStatus && currentStatus.helper_connected) {
    if (banner) banner.className = 'alert-banner alert-success';
    if (text) text.innerHTML = '🟢 <strong>Stato: Connesso & Operativo</strong> (/run/allod/helper.sock)';
  } else if (currentStatus && currentStatus.helper_permission_denied) {
    if (banner) banner.className = 'alert-banner alert-danger';
    if (text) text.innerHTML = '🔴 <strong>Stato: Permesso Negato (EACCES)</strong> — L\'utente non appartiene al gruppo <code>allod</code>.<br><small style="margin-top:4px; display:inline-block;">Esegui: <code>sudo groupadd -f allod && sudo usermod -aG allod $USER</code> e ricarica la sessione (<code>loginctl terminate-user $USER</code> o riavvio).</small>';
  } else {
    if (banner) banner.className = 'alert-banner alert-danger';
    if (text) text.innerHTML = '🔴 <strong>Stato: Offline / Non Avviato</strong> (/run/allod/helper.sock non risponde)';
  }
  modal.classList.remove('hidden');
}

function closeHelperModal() {
  const modal = document.getElementById('helper-modal');
  if (modal) modal.classList.add('hidden');
}

async function recheckHelperConnection(btn) {
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Controllo in corso...</span>';
  }
  await refreshData();
  const banner = document.getElementById('helper-modal-status-banner');
  const text = document.getElementById('helper-modal-status-text');
  if (currentStatus && currentStatus.helper_connected) {
    if (banner) banner.className = 'alert-banner alert-success';
    if (text) text.innerHTML = '🟢 <strong>Stato: Connesso & Operativo!</strong>';
    showAlert(t('helper_connected', 'Root Helper: Connesso'), 'success');
  } else if (currentStatus && currentStatus.helper_permission_denied) {
    if (banner) banner.className = 'alert-banner alert-danger';
    if (text) text.innerHTML = '🔴 <strong>Stato: Permesso Negato (EACCES)</strong> — esegui groupadd/usermod e ricarica la sessione.';
    showAlert(t('helper_eacces_pill', 'Root Helper: Permesso Negato (EACCES)'), 'warning');
  } else {
    if (banner) banner.className = 'alert-banner alert-danger';
    if (text) text.innerHTML = '🔴 <strong>Stato: Ancora Offline</strong> — esegui i comandi per avviarlo.';
    showAlert(t('helper_offline', 'Root Helper: Offline / Non Avviato'), 'warning');
  }
  if (btn) {
    btn.disabled = false;
    btn.innerHTML = `🔄 <span>${t('btn_recheck', 'Ricontrolla Connessione')}</span>`;
  }
}

function copyHelperInstallCli(btn) {
  const code = `sudo install -m 0755 ~/allod/allod-helperd /usr/local/bin/allod-helperd
sudo groupadd -f allod && sudo usermod -aG allod $USER
sudo cp ~/allod/configs/allod-helperd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now allod-helperd`;
  copyTextToClipboard(code, btn);
}

// ==========================================
// PHOTOS & SHARES INTEGRATION
// ==========================================

async function fetchPhotosSharesIntegration() {
  try {
    const res = await fetch('/api/modules/photos/shares-integration');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      photosSharesState = json.data;
    }
  } catch (err) {
    console.warn('Could not fetch photos-shares integration state:', err);
  }
}

async function togglePhotosSharesIntegration(enabled) {
  const cb = document.getElementById('photos-shares-checkbox');
  if (cb) cb.disabled = true;
  try {
    const res = await fetch('/api/modules/photos/shares-integration', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: enabled })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(data.message, 'success');
      await fetchPhotosSharesIntegration();
      renderModules();
    } else {
      showAlert('Errore: ' + data.message, 'danger');
      if (cb) {
        cb.checked = !enabled;
        cb.disabled = false;
      }
    }
  } catch (err) {
    showAlert('Errore di rete: ' + err.message, 'danger');
    if (cb) {
      cb.checked = !enabled;
      cb.disabled = false;
    }
  }
}

// ==========================================
// SAMBA SMB PASSWORD MANAGEMENT
// ==========================================

function openSmbPasswordModal() {
  const modal = document.getElementById('smb-password-modal');
  if (!modal) return;

  const host = window.location.hostname || '192.168.1.50';
  const pathEl = document.getElementById('smb-network-path');
  if (pathEl) {
    pathEl.textContent = `\\\\${host}\\shares`;
  }

  const userInput = document.getElementById('smb-user-input');
  if (userInput && currentStatus && currentStatus.user) {
    userInput.value = currentStatus.user;
  }

  const passInput = document.getElementById('smb-pass-input');
  if (passInput) {
    passInput.value = '';
    passInput.type = 'password';
  }

  modal.classList.remove('hidden');
  if (passInput) passInput.focus();
}

function closeSmbPasswordModal() {
  const modal = document.getElementById('smb-password-modal');
  if (modal) modal.classList.add('hidden');
}

function toggleSmbPassVisibility() {
  const passInput = document.getElementById('smb-pass-input');
  if (!passInput) return;
  passInput.type = passInput.type === 'password' ? 'text' : 'password';
}

function copySmbPath(btn) {
  const pathEl = document.getElementById('smb-network-path');
  if (!pathEl) return;
  copyTextToClipboard(pathEl.innerText || pathEl.textContent, btn);
}

async function saveSmbPassword() {
  const userInput = document.getElementById('smb-user-input');
  const passInput = document.getElementById('smb-pass-input');
  const btn = document.getElementById('btn-save-smb-pass');

  const username = (userInput && userInput.value.trim()) || ((currentStatus && currentStatus.user) ? currentStatus.user : 'user');
  const password = passInput ? passInput.value : '';

  if (!password || password.length < 4) {
    showAlert('La password deve contenere almeno 4 caratteri', 'warning');
    if (passInput) passInput.focus();
    return;
  }

  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>⏳ Salvataggio...</span>';
  }

  try {
    const res = await fetch('/api/modules/shares/set-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, password: password })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(data.message || 'Password Samba impostata con successo!', 'success');
      closeSmbPasswordModal();
    } else {
      showAlert('Errore impostazione password: ' + data.message, 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `💾 <span>${t('btn_save_smb_pass', 'Salva Password Samba')}</span>`;
    }
  }
}

// ==========================================
// FAMILY & TRIAD MEMBERS MANAGEMENT (Samba + Immich + Jellyfin)
// ==========================================

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

let activeFamilyTab = 'list';
let currentQRLink = '';

function switchFamilyTab(tab) {
  activeFamilyTab = tab;
  const btnList = document.getElementById('btn-tab-family-list');
  const btnCreate = document.getElementById('btn-tab-family-create');
  const btnGuide = document.getElementById('btn-tab-family-guide');
  const paneList = document.getElementById('pane-family-list');
  const paneCreate = document.getElementById('pane-family-create');
  const paneGuide = document.getElementById('pane-family-guide');

  [btnList, btnCreate, btnGuide].forEach(b => {
    if (b) {
      b.style.background = 'transparent';
      b.style.color = 'var(--text-muted)';
    }
  });
  [paneList, paneCreate, paneGuide].forEach(p => {
    if (p) p.classList.add('hidden');
  });

  if (tab === 'list') {
    if (btnList) { btnList.style.background = 'var(--primary)'; btnList.style.color = '#000'; }
    if (paneList) paneList.classList.remove('hidden');
    loadFamilyMembers();
  } else if (tab === 'create') {
    if (btnCreate) { btnCreate.style.background = 'var(--primary)'; btnCreate.style.color = '#000'; }
    if (paneCreate) paneCreate.classList.remove('hidden');
    checkTriadServiceStatus();
  } else if (tab === 'guide') {
    if (btnGuide) { btnGuide.style.background = 'var(--primary)'; btnGuide.style.color = '#000'; }
    if (paneGuide) paneGuide.classList.remove('hidden');
  }
}

function autoSuggestUsername() {
  const fnInput = document.getElementById('family-first-name');
  const uInput = document.getElementById('family-username');
  if (!fnInput || !uInput) return;
  const clean = fnInput.value.trim().toLowerCase().replace(/[^a-z0-9]/g, '');
  if (!uInput.dataset.manualEdited || uInput.value === '') {
    uInput.value = clean;
  }
}

function togglePassModeUI() {
  const isManual = document.getElementById('pass-mode-manual')?.checked;
  const boxManual = document.getElementById('box-manual-password');
  if (boxManual) {
    if (isManual) boxManual.classList.remove('hidden');
    else boxManual.classList.add('hidden');
  }
}

async function openFamilyModal() {
  const modal = document.getElementById('triad-user-modal');
  if (!modal) return;
  modal.classList.remove('hidden');
  switchFamilyTab('list');
  checkTriadServiceStatus();
}

function openTriadUserModal() {
  openFamilyModal();
}

function closeFamilyModal() {
  const modal = document.getElementById('triad-user-modal');
  if (modal) modal.classList.add('hidden');
}

function closeTriadUserModal() {
  closeFamilyModal();
}

async function checkTriadServiceStatus() {
  const warnBanner = document.getElementById('family-warning-banner');
  const warnText = document.getElementById('family-warning-text');
  const submitBtn = document.getElementById('btn-submit-family');
  const sharesBadge = document.getElementById('triad-shares-badge');
  const photosBadge = document.getElementById('triad-photos-badge');
  const mediaBadge = document.getElementById('triad-media-badge');

  if (sharesBadge) sharesBadge.innerHTML = '⏳ ...';
  if (photosBadge) photosBadge.innerHTML = '⏳ ...';
  if (mediaBadge) mediaBadge.innerHTML = '⏳ ...';

  try {
    const res = await fetch('/api/triad/status');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      const d = json.data;
      if (sharesBadge) sharesBadge.innerHTML = d.shares_active ? '<span style="color:#10b981; font-weight:700;">🟢 Attivo</span>' : '<span style="color:#ef4444; font-weight:700;">🔴 Spento</span>';
      if (photosBadge) photosBadge.innerHTML = d.photos_active ? '<span style="color:#10b981; font-weight:700;">🟢 Attivo</span>' : '<span style="color:#ef4444; font-weight:700;">🔴 Spento</span>';
      if (mediaBadge) mediaBadge.innerHTML = d.media_active ? '<span style="color:#10b981; font-weight:700;">🟢 Attivo</span>' : '<span style="color:#ef4444; font-weight:700;">🔴 Spento</span>';

      if (d.all_active) {
        if (warnBanner) warnBanner.classList.add('hidden');
        if (submitBtn) submitBtn.disabled = false;
      } else {
        if (warnBanner) {
          warnBanner.classList.remove('hidden');
          if (warnText) warnText.innerHTML = `${d.reason}<br><br>👉 <em>Per abilitare i servizi mancanti, avvia ciascun modulo dalla schermata Moduli.</em>`;
        }
        if (submitBtn) submitBtn.disabled = true;
      }
    }
  } catch (err) {
    if (warnBanner) {
      warnBanner.classList.remove('hidden');
      if (warnText) warnText.textContent = 'Errore verifica stato moduli: ' + err.message;
    }
  }
}

async function loadFamilyMembers() {
  const container = document.getElementById('family-members-container');
  const countBadge = document.getElementById('family-member-count');
  if (!container) return;

  container.innerHTML = '<div style="text-align:center; padding:24px; color:var(--text-muted); font-size:12px;">⏳ Caricamento membri...</div>';

  try {
    const res = await fetch('/api/family/members');
    const json = await res.json();
    if (json.status !== 'ok' || !Array.isArray(json.data) || json.data.length === 0) {
      container.innerHTML = `
        <div style="text-align:center; padding:30px; background:rgba(255,255,255,0.02); border:1px dashed var(--card-border); border-radius:8px;">
          <div style="font-size:32px; margin-bottom:8px;">👨‍👩‍👧‍👦</div>
          <div style="font-size:13px; font-weight:700; color:var(--text-main);">Nessun membro della famiglia configurato</div>
          <p style="font-size:11.5px; color:var(--text-muted); margin:4px 0 14px 0;">Crea il primo profilo per attivare la cartella privata Samba e l'ecosistema Triade.</p>
          <button type="button" class="btn btn-sm btn-primary" onclick="switchFamilyTab('create')" style="display:inline-flex; width:auto; padding:6px 14px;">
            ➕ Aggiungi Membro Adesso
          </button>
        </div>
      `;
      if (countBadge) countBadge.innerText = '0';
      return;
    }

    if (countBadge) countBadge.innerText = json.data.length;

    let html = '';
    json.data.forEach(m => {
      const isPhotosLinked = !!m.photos_linked;
      const initial = m.first_name ? m.first_name.charAt(0).toUpperCase() : (m.username ? m.username.charAt(0).toUpperCase() : '?');
      const avatarBg = m.avatar_color || '#38bdf8';
      
      let roleLabel = 'Membro';
      let roleBadgeBg = 'rgba(255,255,255,0.06)';
      let roleColor = 'var(--text-main)';
      if (m.role === 'admin') {
        roleLabel = 'Genitore / Admin';
        roleBadgeBg = 'rgba(245,158,11,0.15)';
        roleColor = '#f59e0b';
      } else if (m.role === 'guest') {
        roleLabel = 'Ospite';
        roleBadgeBg = 'rgba(148,163,184,0.15)';
        roleColor = '#94a3b8';
      }

      const photoPill = isPhotosLinked
        ? `<span style="font-size:10px; padding:2px 7px; border-radius:10px; background:rgba(16,185,129,0.15); color:#10b981; font-weight:600;">🟢 photos/ attiva</span>`
        : `<span style="font-size:10px; padding:2px 7px; border-radius:10px; background:rgba(255,255,255,0.06); color:#94a3b8; font-weight:600;">⚪ Solo App Immich (0770)</span>`;

      html += `
        <div style="background:rgba(255,255,255,0.02); border:1px solid var(--card-border); border-radius:8px; padding:12px 14px; display:flex; justify-content:space-between; align-items:center; gap:12px; transition:border-color 0.15s ease;">
          <div style="display:flex; align-items:center; gap:12px; flex:1; min-width:0;">
            <div style="width:38px; height:38px; border-radius:50%; background:${avatarBg}; color:#000; font-weight:800; font-size:15px; display:flex; align-items:center; justify-content:center; flex-shrink:0; box-shadow:0 2px 8px rgba(0,0,0,0.3);">
              ${escapeHtml(initial)}
            </div>
            <div style="min-width:0; flex:1;">
              <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                <strong style="font-size:13.5px; color:var(--text-main);">${escapeHtml(m.display_name)}</strong>
                <span style="font-size:10.5px; padding:1px 6px; border-radius:4px; background:${roleBadgeBg}; color:${roleColor}; font-weight:600;">${roleLabel}</span>
                <span style="font-size:11px; color:var(--text-muted); font-family:monospace;">@${escapeHtml(m.username)}</span>
              </div>
              <div style="font-size:11px; color:var(--text-muted); margin-top:3px; display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                <span>📁 SMB: <code style="color:#38bdf8;">${escapeHtml(m.smb_path)}</code></span>
                ${photoPill}
              </div>
              ${m.notes ? `<div style="font-size:10.5px; color:var(--text-muted); margin-top:2px;">📱 ${escapeHtml(m.notes)}</div>` : ''}
            </div>
          </div>

          <!-- Actions -->
          <div style="display:flex; align-items:center; gap:6px; flex-shrink:0;">
            <button type="button" class="btn btn-sm btn-secondary" onclick="generateMemberResetLink('${escapeHtml(m.username)}')" 
                    style="font-size:11px; padding:4px 9px; border-radius:5px; background:rgba(255,255,255,0.05);" title="Mostra QR Code e Link di Invito/Onboarding">
              📲 Link / QR
            </button>
            <button type="button" class="btn btn-sm" onclick="toggleFamilyUserPhotos('${escapeHtml(m.username)}', ${!isPhotosLinked})" 
                    style="font-size:11px; padding:4px 9px; border-radius:5px; background:transparent; border:1px solid ${isPhotosLinked ? 'rgba(239,68,68,0.3)' : 'var(--primary)'}; color:${isPhotosLinked ? '#ef4444' : 'var(--primary)'};"
                    title="${isPhotosLinked ? 'Scollega la cartella foto da Samba' : 'Collega la libreria foto su Samba'}">
              ${isPhotosLinked ? 'Scollega Foto' : 'Collega Foto'}
            </button>
            <button type="button" class="btn btn-sm" onclick="deleteFamilyMember('${escapeHtml(m.username)}')" 
                    style="font-size:11px; padding:4px 7px; border-radius:5px; background:transparent; border:1px solid rgba(239,68,68,0.2); color:#ef4444;"
                    title="Rimuovi membro dalla famiglia">
              🗑️
            </button>
          </div>
        </div>
      `;
    });

    container.innerHTML = html;
  } catch (err) {
    container.innerHTML = `<div style="color:#ef4444; font-size:12px; padding:16px;">Errore caricamento membri: ${escapeHtml(err.message)}</div>`;
  }
}

function loadTriadUsers() {
  loadFamilyMembers();
}

async function submitCreateFamilyMember() {
  const fnInput = document.getElementById('family-first-name');
  const lnInput = document.getElementById('family-last-name');
  const uInput = document.getElementById('family-username');
  const emInput = document.getElementById('family-email');
  const roleSelect = document.getElementById('family-role');
  const notesInput = document.getElementById('family-notes');
  const linkPhotosInput = document.getElementById('family-link-photos');
  const isManual = document.getElementById('pass-mode-manual')?.checked;
  const manualPassInput = document.getElementById('family-manual-pass');
  const submitBtn = document.getElementById('btn-submit-family');

  const firstName = fnInput ? fnInput.value.trim() : '';
  const lastName = lnInput ? lnInput.value.trim() : '';
  const username = uInput ? uInput.value.trim().toLowerCase() : '';
  const email = emInput ? emInput.value.trim() : '';
  const role = roleSelect ? roleSelect.value : 'member';
  const notes = notesInput ? notesInput.value.trim() : '';
  const linkPhotos = linkPhotosInput ? linkPhotosInput.checked : true;
  const password = isManual && manualPassInput ? manualPassInput.value : '';

  if (!firstName) {
    showAlert('Inserisci il nome del membro della famiglia', 'warning');
    if (fnInput) fnInput.focus();
    return;
  }
  if (!username || username.length < 2) {
    showAlert('Username non valido (almeno 2 caratteri alfanumerici)', 'warning');
    if (uInput) uInput.focus();
    return;
  }
  if (isManual && (!password || password.length < 4)) {
    showAlert('La password manuale deve contenere almeno 4 caratteri', 'warning');
    if (manualPassInput) manualPassInput.focus();
    return;
  }

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>⏳ Creazione & orchestrazione in corso...</span>';
  }

  try {
    const res = await fetch('/api/family/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        first_name: firstName,
        last_name: lastName,
        username: username,
        email: email,
        role: role,
        notes: notes,
        password: password,
        link_photos: linkPhotos,
        generate_invite: !isManual
      })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(data.message || 'Membro creato con successo!', 'success');
      if (fnInput) fnInput.value = '';
      if (lnInput) lnInput.value = '';
      if (uInput) { uInput.value = ''; delete uInput.dataset.manualEdited; }
      if (emInput) emInput.value = '';
      if (notesInput) notesInput.value = '';
      if (manualPassInput) manualPassInput.value = '';

      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.innerHTML = '🚀 <span>Crea Membro & Predisponi Allod</span>';
      }

      await loadFamilyMembers();
      await refreshData();

      if (data.data && data.data.invite_url) {
        showFamilyQRModal(`${firstName} ${lastName}`.trim(), data.data.invite_url);
      } else {
        switchFamilyTab('list');
      }
    } else {
      showAlert('Errore: ' + data.message, 'danger');
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.innerHTML = '🚀 <span>Crea Membro & Predisponi Allod</span>';
      }
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '🚀 <span>Crea Membro & Predisponi Allod</span>';
    }
  }
}

function executeCreateTriadUser() {
  return submitCreateFamilyMember();
}

async function generateMemberResetLink(username) {
  try {
    const res = await fetch('/api/family/generate-reset-link', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username })
    });
    const json = await res.json();
    if (json.status === 'ok' && json.data && json.data.invite_url) {
      showFamilyQRModal(`@${username}`, json.data.invite_url);
    } else {
      showAlert(json.message || 'Errore generazione link', 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  }
}

function showFamilyQRModal(name, inviteUrl) {
  currentQRLink = inviteUrl;
  const modal = document.getElementById('family-qr-modal');
  const nameEl = document.getElementById('qr-member-name');
  const linkEl = document.getElementById('qr-link-text');
  const linkInput = document.getElementById('qr-link-input');
  const svgContainer = document.getElementById('qr-svg-container');
  const waBtn = document.getElementById('btn-share-whatsapp');
  const tgBtn = document.getElementById('btn-share-telegram');

  if (nameEl) nameEl.innerText = name;
  if (linkEl) linkEl.innerText = inviteUrl;
  if (linkInput) linkInput.value = inviteUrl;

  if (svgContainer && typeof QRCode !== 'undefined' && QRCode.generateSVG) {
    svgContainer.innerHTML = QRCode.generateSVG(inviteUrl, 190);
  }

  const shareText = `Ciao! Ecco il tuo link personale di onboarding per Allod Cloud:\n${inviteUrl}`;
  if (waBtn) waBtn.href = `https://api.whatsapp.com/send?text=${encodeURIComponent(shareText)}`;
  if (tgBtn) tgBtn.href = `https://t.me/share/url?url=${encodeURIComponent(inviteUrl)}&text=${encodeURIComponent('Il tuo link personale di onboarding per Allod Cloud')}`;

  if (modal) modal.classList.remove('hidden');
}

function closeFamilyQRModal() {
  const modal = document.getElementById('family-qr-modal');
  if (modal) modal.classList.add('hidden');
}

function openQRLink() {
  if (currentQRLink) {
    window.open(currentQRLink, '_blank');
  } else {
    showAlert('Nessun link disponibile da aprire', 'warning');
  }
}

function copyQRLink(btn) {
  if (!currentQRLink) {
    showAlert('Nessun link disponibile da copiare', 'warning');
    return;
  }
  const linkInput = document.getElementById('qr-link-input');
  if (linkInput) {
    linkInput.focus();
    linkInput.select();
    linkInput.setSelectionRange(0, linkInput.value.length);
  }
  copyTextToClipboard(currentQRLink, btn);
}

async function deleteFamilyMember(username) {
  if (!confirm(`Sei sicuro di voler rimuovere il membro '${username}' dalla famiglia Allod?\n(I file su disco rimarranno preservati per sicurezza)`)) {
    return;
  }
  try {
    const res = await fetch('/api/family/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username })
    });
    const json = await res.json();
    if (json.status === 'ok') {
      showAlert(json.message || 'Membro rimosso.', 'success');
      await loadFamilyMembers();
      await refreshData();
    } else {
      showAlert('Errore: ' + json.message, 'danger');
    }
  } catch (err) {
    showAlert('Errore: ' + err.message, 'danger');
  }
}

async function toggleFamilyUserPhotos(username, targetStatus) {
  const actionText = targetStatus ? 'collegare la cartella foto Immich' : 'scollegare la cartella foto Immich';
  if (!confirm(`Vuoi ${actionText} per l'utente '${username}' dallo share Samba?`)) {
    return;
  }

  try {
    const res = await fetch('/api/triad/toggle-photos-link', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, enabled: targetStatus })
    });
    const json = await res.json();
    if (json.status === 'ok') {
      showAlert(json.message || 'Stato cartella foto aggiornato con successo!', 'success');
      await loadFamilyMembers();
    } else {
      showAlert('Errore: ' + json.message, 'danger');
    }
  } catch (err) {
    showAlert('Errore di connessione: ' + err.message, 'danger');
  }
}

function toggleTriadUserPhotos(username, targetStatus) {
  return toggleFamilyUserPhotos(username, targetStatus);
}

// ==========================================
// NETWORK & MESH ACCESS -> extracted to network.js (ES module)
// ==========================================

// ==========================================================================
// THEME MANAGEMENT (Client-Side, 0% Server CPU / RAM Load)
// ==========================================================================
function initTheme() {
  const savedTheme = localStorage.getItem('allod_theme') || 'default';
  applyTheme(savedTheme, false);
  initThemeCollapse();
}

function initThemeCollapse() {
  const isExpanded = localStorage.getItem('allod_theme_expanded') === 'true';
  const themeBody = document.getElementById('settings-theme-body');
  const chevron = document.getElementById('theme-chevron');
  if (themeBody) {
    if (isExpanded) {
      themeBody.classList.remove('hidden');
      if (chevron) chevron.classList.add('expanded');
    } else {
      themeBody.classList.add('hidden');
      if (chevron) chevron.classList.remove('expanded');
    }
  }
}

function toggleThemeCollapse(forceExpand) {
  const themeBody = document.getElementById('settings-theme-body');
  const chevron = document.getElementById('theme-chevron');
  if (!themeBody) return;

  const isHidden = themeBody.classList.contains('hidden');
  const shouldExpand = (typeof forceExpand === 'boolean') ? forceExpand : isHidden;

  if (shouldExpand) {
    themeBody.classList.remove('hidden');
    if (chevron) chevron.classList.add('expanded');
    localStorage.setItem('allod_theme_expanded', 'true');
  } else {
    themeBody.classList.add('hidden');
    if (chevron) chevron.classList.remove('expanded');
    localStorage.setItem('allod_theme_expanded', 'false');
  }
}

function selectTheme(themeName) {
  applyTheme(themeName, true);
}

function applyTheme(themeName, save = true) {
  const validThemes = ['default', 'moderno', 'cia', 'allod'];
  if (!validThemes.includes(themeName)) {
    themeName = 'default';
  }

  if (themeName === 'default') {
    document.documentElement.removeAttribute('data-theme');
  } else {
    document.documentElement.setAttribute('data-theme', themeName);
  }

  if (save) {
    localStorage.setItem('allod_theme', themeName);
  }

  // Update active state on theme cards in settings
  validThemes.forEach(tName => {
    const card = document.getElementById(`theme-card-${tName}`);
    if (card) {
      if (tName === themeName) {
        card.classList.add('active');
      } else {
        card.classList.remove('active');
      }
    }
  });

  updateThemeBadge(themeName);
}

function updateThemeBadge(themeName) {
  if (!themeName) {
    themeName = localStorage.getItem('allod_theme') || 'default';
  }

  const badge = document.getElementById('current-theme-badge');
  if (badge) {
    const names = {
      default: (typeof t === 'function' ? t('theme_default', 'Default Slate') : 'Default Slate'),
      moderno: (typeof t === 'function' ? t('theme_moderno', 'Moderno') : 'Moderno'),
      cia: (typeof t === 'function' ? t('theme_cia', 'CIA // Tactical (Tom Clancy)') : 'CIA // Tactical (Tom Clancy)'),
      allod: (typeof t === 'function' ? t('theme_allod', 'Allod') : 'Allod')
    };
    badge.textContent = names[themeName] || themeName;
  }
}

function openThemeSelector() {
  switchToTab('settings');
  toggleThemeCollapse(true);
  const themeCard = document.getElementById('settings-theme-card');
  if (themeCard) {
    setTimeout(() => {
      themeCard.scrollIntoView({ behavior: 'smooth', block: 'start' });
      themeCard.style.outline = '2px solid var(--primary)';
      setTimeout(() => {
        themeCard.style.outline = 'none';
      }, 1500);
    }, 100);
  }
}

// ==============================================================================
// ALLOD WATCH SENTINEL & TELEGRAM BOT MODAL LOGIC
// ==============================================================================

let currentWatchConfig = null;
let currentWatchMeshIP = '--';
let currentWatchMode = 'receiver';

async function openWatchSentinelModal() {
  const modal = document.getElementById('watch-sentinel-modal');
  if (!modal) return;

  switchWatchTab('telegram');

  try {
    const res = await fetch('/api/watch/config');
    const json = await res.json();
    if (json.status === 'ok' && json.data) {
      currentWatchConfig = json.data.config || {};
      currentWatchMeshIP = json.data.mesh_ip || '--';

      // Mode: default to receiver (push mode)
      currentWatchMode = (currentWatchConfig.mode === 'poller' || currentWatchConfig.mode === 'mesh') ? 'poller' : 'receiver';

      // Populate Telegram Tab
      const tokenInput = document.getElementById('watch-telegram-token');
      const chatIdInput = document.getElementById('watch-telegram-chat-id');
      if (tokenInput) tokenInput.value = currentWatchConfig.telegram_bot_token || '';
      if (chatIdInput) chatIdInput.value = currentWatchConfig.telegram_chat_id || '';

      // Populate Weather Tab
      const cityInput = document.getElementById('watch-weather-city');
      const timeInput = document.getElementById('watch-digest-time');
      const threshSelect = document.getElementById('watch-down-threshold');
      if (cityInput) cityInput.value = currentWatchConfig.weather_city || 'Roma';
      if (timeInput) timeInput.value = currentWatchConfig.digest_time || '08:30';
      if (threshSelect) {
        threshSelect.value = String(currentWatchConfig.down_threshold_seconds || (currentWatchMode === 'receiver' ? 300 : 180));
      }

      // Populate Push Settings
      const hostInput = document.getElementById('watch-vps-host');
      const portInput = document.getElementById('watch-vps-port');
      const secretTokenInput = document.getElementById('watch-secret-token');
      const pushIntervalSelect = document.getElementById('watch-push-interval');

      if (hostInput) hostInput.value = currentWatchConfig.vps_host || '';
      if (portInput) portInput.value = currentWatchConfig.vps_port || 8443;
      if (secretTokenInput) secretTokenInput.value = currentWatchConfig.secret_token || '';
      if (pushIntervalSelect) pushIntervalSelect.value = String(currentWatchConfig.push_interval_seconds || 60);

      // If in push mode and secret token is empty, auto-generate one
      if (currentWatchMode === 'receiver' && (!currentWatchConfig.secret_token || currentWatchConfig.secret_token === '')) {
        await generateWatchSecretToken(false);
      }

      // Populate Mesh Settings
      const vpsKeyInput = document.getElementById('watch-vps-setup-key');
      if (vpsKeyInput) vpsKeyInput.value = currentWatchConfig.vps_setup_key || '';

      const meshIpDisplay = document.getElementById('watch-mesh-ip-display');
      if (meshIpDisplay) {
        meshIpDisplay.textContent = currentWatchMeshIP;
      }

      setWatchMode(currentWatchMode);
    }
  } catch (err) {
    console.error('Failed to load sentinel config:', err);
  }

  modal.classList.remove('hidden');
}

function closeWatchSentinelModal() {
  const modal = document.getElementById('watch-sentinel-modal');
  if (modal) modal.classList.add('hidden');
}

function switchWatchTab(tabName) {
  const tabs = ['telegram', 'weather', 'mesh'];
  tabs.forEach(t => {
    const btn = document.getElementById(`btn-tab-watch-${t}`);
    const pane = document.getElementById(`pane-watch-${t}`);
    if (btn && pane) {
      if (t === tabName) {
        btn.style.background = 'var(--primary)';
        btn.style.color = '#000';
        pane.classList.remove('hidden');
      } else {
        btn.style.background = 'transparent';
        btn.style.color = 'var(--text-muted)';
        pane.classList.add('hidden');
      }
    }
  });
}

function setWatchMode(mode) {
  currentWatchMode = (mode === 'poller' || mode === 'mesh') ? 'poller' : 'receiver';

  const cardPush = document.getElementById('watch-card-push');
  const cardMesh = document.getElementById('watch-card-mesh');
  const sectionPush = document.getElementById('watch-section-push');
  const sectionMesh = document.getElementById('watch-section-mesh');

  const step1 = document.getElementById('watch-instruction-step1');
  const step2 = document.getElementById('watch-instruction-step2');
  const step3 = document.getElementById('watch-instruction-step3');

  if (currentWatchMode === 'receiver') {
    if (cardPush) {
      cardPush.style.border = '2px solid var(--primary)';
      cardPush.style.background = 'rgba(56,189,248,0.08)';
    }
    if (cardMesh) {
      cardMesh.style.border = '1px solid var(--card-border)';
      cardMesh.style.background = 'rgba(15,23,42,0.4)';
    }
    if (sectionPush) sectionPush.classList.remove('hidden');
    if (sectionMesh) sectionMesh.classList.add('hidden');

    if (step1) step1.innerHTML = '1. Connettiti via SSH alla tua VPS esterna: <code>ssh root@ip-vps</code>';
    if (step2) step2.innerHTML = '2. Incolla il comando copiato qui sopra e premi <b>Invio</b>.';
    if (step3) step3.innerHTML = '3. La VPS scarica allod-watch, configura il ricevitore HTTP sulla porta specificata, apre il firewall e invia la conferma su Telegram!';
  } else {
    if (cardMesh) {
      cardMesh.style.border = '2px solid var(--primary)';
      cardMesh.style.background = 'rgba(56,189,248,0.08)';
    }
    if (cardPush) {
      cardPush.style.border = '1px solid var(--card-border)';
      cardPush.style.background = 'rgba(15,23,42,0.4)';
    }
    if (sectionMesh) sectionMesh.classList.remove('hidden');
    if (sectionPush) sectionPush.classList.add('hidden');

    if (step1) step1.innerHTML = '1. Connettiti via SSH alla tua VPS esterna: <code>ssh root@ip-vps</code>';
    if (step2) step2.innerHTML = '2. Incolla il comando copiato qui sopra e premi <b>Invio</b>.';
    if (step3) step3.innerHTML = '3. La VPS entra nella rete mesh NetBird, scarica la sentinella Allod, attiva systemd e invia subito una conferma su Telegram!';
  }

  updateWatchDeployCommand();
}

function toggleWatchTokenVisibility() {
  const input = document.getElementById('watch-telegram-token');
  if (input) {
    input.type = input.type === 'password' ? 'text' : 'password';
  }
}

function toggleWatchSecretTokenVisibility() {
  const input = document.getElementById('watch-secret-token');
  if (input) {
    input.type = input.type === 'password' ? 'text' : 'password';
  }
}

async function generateWatchSecretToken(save = true) {
  try {
    const res = await fetch('/api/watch/generate-token', { method: 'POST' });
    const json = await res.json();
    if (json.status === 'ok' && json.token) {
      const input = document.getElementById('watch-secret-token');
      if (input) input.value = json.token;
      updateWatchDeployCommand();
      if (save) {
        await saveWatchConfig(false);
      }
    }
  } catch (err) {
    console.error('Failed to generate secret token:', err);
  }
}

function onWatchTokenInput() {
  updateWatchDeployCommand();
}

async function detectWatchChatID() {
  const tokenInput = document.getElementById('watch-telegram-token');
  const token = tokenInput ? tokenInput.value.trim() : '';
  const feedback = document.getElementById('watch-telegram-feedback');
  const spinner = document.getElementById('watch-detect-spinner');
  const detectBtn = document.getElementById('btn-detect-chat-id');

  if (!token) {
    if (feedback) {
      feedback.style.display = 'block';
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Inserisci prima il Bot Token fornito da @BotFather al Passo 2.';
    }
    return;
  }

  if (spinner) spinner.style.display = 'inline-block';
  if (detectBtn) detectBtn.disabled = true;
  if (feedback) feedback.style.display = 'none';

  try {
    const res = await fetch('/api/watch/detect-chat-id', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bot_token: token })
    });
    const json = await res.json();

    if (json.status === 'ok' && json.data && json.data.length > 0) {
      const firstChat = json.data[0];
      const chatIdInput = document.getElementById('watch-telegram-chat-id');
      if (chatIdInput) {
        chatIdInput.value = firstChat.chat_id;
      }
      if (feedback) {
        feedback.style.display = 'block';
        feedback.style.background = 'rgba(16,185,129,0.15)';
        feedback.style.color = '#34d399';
        feedback.style.border = '1px solid rgba(16,185,129,0.3)';
        const name = firstChat.first_name || firstChat.username || 'Utente';
        feedback.innerHTML = `✅ <strong>Chat ID Rilevato con successo!</strong><br>👤 Utente: <b>${name}</b> (ID: <code>${firstChat.chat_id}</code>)`;
      }
      // Auto-save progress
      await saveWatchConfig(false);
    } else {
      if (feedback) {
        feedback.style.display = 'block';
        feedback.style.background = 'rgba(245,158,11,0.15)';
        feedback.style.color = '#fbbf24';
        feedback.style.border = '1px solid rgba(245,158,11,0.3)';
        feedback.innerHTML = `⚠️ <strong>Nessun messaggio trovato</strong><br>1. Apri Telegram e cerca il tuo bot<br>2. Clicca su <b>AVVIA</b> (oppure inviagli <code>/start</code>)<br>3. Clicca nuovamente su 'Rileva Chat ID'`;
      }
    }
  } catch (err) {
    if (feedback) {
      feedback.style.display = 'block';
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Errore durante il rilevamento: ' + err.message;
    }
  } finally {
    if (spinner) spinner.style.display = 'none';
    if (detectBtn) detectBtn.disabled = false;
  }
}

async function testWatchTelegram() {
  const token = (document.getElementById('watch-telegram-token')?.value || '').trim();
  const chatId = (document.getElementById('watch-telegram-chat-id')?.value || '').trim();
  const feedback = document.getElementById('watch-telegram-feedback');
  const btn = document.getElementById('btn-test-watch-tg');

  if (!token || !chatId) {
    if (feedback) {
      feedback.style.display = 'block';
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Compila sia il Bot Token che il Chat ID prima di inviare la prova.';
    }
    return;
  }

  if (btn) btn.disabled = true;
  if (feedback) {
    feedback.style.display = 'block';
    feedback.style.background = 'rgba(56,189,248,0.15)';
    feedback.style.color = '#38bdf8';
    feedback.style.border = '1px solid rgba(56,189,248,0.3)';
    feedback.textContent = '⏳ Invio messaggio di notifica su Telegram in corso...';
  }

  try {
    const res = await fetch('/api/watch/test-telegram', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bot_token: token, chat_id: chatId })
    });
    const json = await res.json();
    if (json.status === 'ok') {
      if (feedback) {
        feedback.style.background = 'rgba(16,185,129,0.15)';
        feedback.style.color = '#34d399';
        feedback.style.border = '1px solid rgba(16,185,129,0.3)';
        feedback.innerHTML = `✅ <strong>${json.message}</strong>`;
      }
      await saveWatchConfig(false);
    } else {
      if (feedback) {
        feedback.style.background = 'rgba(239,68,68,0.15)';
        feedback.style.color = '#f87171';
        feedback.style.border = '1px solid rgba(239,68,68,0.3)';
        feedback.innerHTML = `❌ <strong>${json.message}</strong>`;
      }
    }
  } catch (err) {
    if (feedback) {
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Errore invio test: ' + err.message;
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function testWatchPush() {
  const host = (document.getElementById('watch-vps-host')?.value || '').trim();
  const port = parseInt(document.getElementById('watch-vps-port')?.value || '8443', 10);
  const token = (document.getElementById('watch-secret-token')?.value || '').trim();
  const feedback = document.getElementById('watch-push-feedback');
  const btn = document.getElementById('btn-test-watch-push');

  if (!host) {
    if (feedback) {
      feedback.style.display = 'block';
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Inserisci prima l\'Host o IP Pubblico della tua VPS.';
    }
    return;
  }

  if (btn) btn.disabled = true;
  if (feedback) {
    feedback.style.display = 'block';
    feedback.style.background = 'rgba(56,189,248,0.15)';
    feedback.style.color = '#38bdf8';
    feedback.style.border = '1px solid rgba(56,189,248,0.3)';
    feedback.textContent = '⏳ Invio richiesta heartbeat di prova verso la VPS...';
  }

  try {
    const res = await fetch('/api/watch/test-push', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ vps_host: host, vps_port: port, secret_token: token })
    });
    const json = await res.json();
    if (json.status === 'ok') {
      if (feedback) {
        feedback.style.background = 'rgba(16,185,129,0.15)';
        feedback.style.color = '#34d399';
        feedback.style.border = '1px solid rgba(16,185,129,0.3)';
        feedback.innerHTML = `✅ <strong>${json.message}</strong>`;
      }
      await saveWatchConfig(false);
    } else {
      if (feedback) {
        feedback.style.background = 'rgba(239,68,68,0.15)';
        feedback.style.color = '#f87171';
        feedback.style.border = '1px solid rgba(239,68,68,0.3)';
        feedback.innerHTML = `❌ <strong>${json.message}</strong>`;
      }
    }
  } catch (err) {
    if (feedback) {
      feedback.style.background = 'rgba(239,68,68,0.15)';
      feedback.style.color = '#f87171';
      feedback.style.border = '1px solid rgba(239,68,68,0.3)';
      feedback.textContent = '❌ Errore invio test push: ' + err.message;
    }
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function saveWatchConfig(showNotification = true) {
  const token = (document.getElementById('watch-telegram-token')?.value || '').trim();
  const chatId = (document.getElementById('watch-telegram-chat-id')?.value || '').trim();
  const city = (document.getElementById('watch-weather-city')?.value || '').trim() || 'Roma';
  const time = (document.getElementById('watch-digest-time')?.value || '').trim() || '08:30';
  const thresh = parseInt(document.getElementById('watch-down-threshold')?.value || '300', 10);

  const host = (document.getElementById('watch-vps-host')?.value || '').trim();
  const port = parseInt(document.getElementById('watch-vps-port')?.value || '8443', 10);
  const secretToken = (document.getElementById('watch-secret-token')?.value || '').trim();
  const pushInterval = parseInt(document.getElementById('watch-push-interval')?.value || '60', 10);
  const setupKey = (document.getElementById('watch-vps-setup-key')?.value || '').trim();

  const payload = {
    mode: currentWatchMode,
    vps_host: host,
    vps_port: port,
    secret_token: secretToken,
    push_interval_seconds: pushInterval,
    telegram_bot_token: token,
    telegram_chat_id: chatId,
    weather_city: city,
    digest_time: time,
    down_threshold_seconds: thresh,
    vps_setup_key: setupKey
  };

  try {
    const res = await fetch('/api/watch/config/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const json = await res.json();
    if (json.status === 'ok') {
      currentWatchConfig = payload;
      updateWatchDeployCommand();
      if (showNotification && typeof showAlert === 'function') {
        showAlert('✓ Impostazioni Sentinella salvate con successo!', 'success');
      }
    }
  } catch (err) {
    console.error('Failed to save watch config:', err);
    if (showNotification && typeof showAlert === 'function') {
      showAlert('Errore salvataggio impostazioni: ' + err.message, 'danger');
    }
  }
}

async function saveWatchConfigAndAdvance(nextTab) {
  await saveWatchConfig(false);
  switchWatchTab(nextTab);
}

function updateWatchDeployCommand() {
  const codeEl = document.getElementById('watch-deploy-command-code');
  if (!codeEl) return;

  const botToken = (document.getElementById('watch-telegram-token')?.value || '').trim();
  const chatId = (document.getElementById('watch-telegram-chat-id')?.value || '').trim();
  const city = (document.getElementById('watch-weather-city')?.value || '').trim() || 'Roma';
  const digestTime = (document.getElementById('watch-digest-time')?.value || '').trim() || '08:30';
  const threshold = parseInt(document.getElementById('watch-down-threshold')?.value || '300', 10);

  if (currentWatchMode === 'receiver') {
    const port = parseInt(document.getElementById('watch-vps-port')?.value || '8443', 10);
    const secret = (document.getElementById('watch-secret-token')?.value || '').trim() || '<SECRET_TOKEN>';
    const tgEnabled = Boolean(botToken && chatId);

    const cmd = `sudo bash -c '
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")
echo "⬇️ Scaricamento Allod Watch Sentinel (${ARCH})..."
curl -fsSL "https://raw.githubusercontent.com/asfaltobollente/allod/main/bin/allod-watch-linux-\${ARCH}" -o /usr/local/bin/allod-watch
chmod 755 /usr/local/bin/allod-watch
mkdir -p /etc/allod
cat << '\''EOF'\'' > /etc/allod/watch.yaml
mode: receiver
receiver:
  port: ${port}
  secret_token: "${secret}"
intervals:
  down_threshold_seconds: ${threshold}
telegram:
  enabled: ${tgEnabled}
  bot_token: "${botToken}"
  chat_id: "${chatId}"
weather:
  enabled: true
  city: "${city}"
digest:
  enabled: true
  time: "${digestTime}"
EOF
chmod 600 /etc/allod/watch.yaml
if command -v ufw >/dev/null 2>&1; then ufw allow ${port}/tcp || true; fi
cat << '\''EOF'\'' > /etc/systemd/system/allod-watch.service
[Unit]
Description=Allod Watch External Sentinel Daemon (Push Receiver)
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
ExecStart=/usr/local/bin/allod-watch run -c /etc/allod/watch.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
LimitNOFILE=65535
ProtectSystem=full
ProtectHome=true
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now allod-watch.service
echo "✅ Allod Watch Sentinel è ora in ascolto sulla porta ${port}!"
/usr/local/bin/allod-watch test-telegram -c /etc/allod/watch.yaml || true
'`;
    codeEl.textContent = cmd.trim();
  } else {
    // Mesh Mode
    const setupKeyInput = document.getElementById('watch-vps-setup-key');
    const alreadyMeshCheck = document.getElementById('watch-vps-already-mesh');
    const meshIP = (currentWatchMeshIP && currentWatchMeshIP !== '--') ? currentWatchMeshIP : '<IP-MESH-ALLOD>';
    const setupKey = setupKeyInput ? setupKeyInput.value.trim() : '';
    const alreadyMesh = alreadyMeshCheck ? alreadyMeshCheck.checked : false;

    let cmd = '';
    if (alreadyMesh) {
      cmd = `curl -fsSL "http://${meshIP}:8080/api/watch/install.sh?mode=mesh" | sudo bash`;
    } else {
      const keyPlaceholder = setupKey || '<NETBIRD_SETUP_KEY>';
      cmd = `curl -fsSL https://pkgs.netbird.io/install.sh | sh && sudo netbird up --setup-key ${keyPlaceholder} && curl -fsSL "http://${meshIP}:8080/api/watch/install.sh?mode=mesh" | sudo bash`;
    }
    codeEl.textContent = cmd;
  }
}

function copyWatchDeployCommand() {
  const codeEl = document.getElementById('watch-deploy-command-code');
  const btn = document.getElementById('btn-copy-watch-cmd');
  if (!codeEl) return;

  const text = codeEl.textContent;
  navigator.clipboard.writeText(text).then(() => {
    if (btn) {
      const origText = btn.innerHTML;
      btn.innerHTML = '✓ Copiato!';
      setTimeout(() => { btn.innerHTML = origText; }, 2500);
    }
    if (typeof showAlert === 'function') {
      showAlert('✓ Comando di deploy copiato negli appunti! Incollalo nel terminale SSH della tua VPS.', 'success');
    }
  }).catch(() => {
    prompt('Copia il comando con Ctrl+C:', text);
  });
}

// Window bindings
window.openWatchSentinelModal = openWatchSentinelModal;
window.closeWatchSentinelModal = closeWatchSentinelModal;
window.switchWatchTab = switchWatchTab;
window.setWatchMode = setWatchMode;
window.toggleWatchTokenVisibility = toggleWatchTokenVisibility;
window.toggleWatchSecretTokenVisibility = toggleWatchSecretTokenVisibility;
window.generateWatchSecretToken = generateWatchSecretToken;
window.onWatchTokenInput = onWatchTokenInput;
window.detectWatchChatID = detectWatchChatID;
window.testWatchTelegram = testWatchTelegram;
window.testWatchPush = testWatchPush;
window.saveWatchConfig = saveWatchConfig;
window.saveWatchConfigAndAdvance = saveWatchConfigAndAdvance;
window.updateWatchDeployCommand = updateWatchDeployCommand;
window.copyWatchDeployCommand = copyWatchDeployCommand;

// ==========================================
// --- WAKE-ON-LAN (WoL) IMPLEMENTATION ---
// ==========================================
let wolDevicesList = [];

function openWoLModal() {
  const modal = document.getElementById('wol-modal');
  if (modal) modal.classList.remove('hidden');
  loadWoLDevices();
}

function closeWoLModal() {
  const modal = document.getElementById('wol-modal');
  if (modal) modal.classList.add('hidden');
  resetWoLForm();
  hideWoLFeedback();
}

function hideWoLFeedback() {
  const fb = document.getElementById('wol-feedback-banner');
  if (fb) {
    fb.className = 'alert-banner hidden';
    fb.innerHTML = '';
  }
}

function showWoLFeedback(msg, type = 'success') {
  const fb = document.getElementById('wol-feedback-banner');
  if (fb) {
    fb.className = `alert-banner alert-${type}`;
    fb.innerHTML = msg;
    fb.classList.remove('hidden');
  }
  if (typeof showAlert === 'function' && type === 'danger') {
    showAlert(msg, type);
  }
}

async function loadWoLDevices() {
  try {
    const res = await fetch('/api/wol/devices');
    if (!res.ok) {
      if (res.status === 401) return;
      throw new Error(`HTTP ${res.status}`);
    }
    const json = await res.json();
    wolDevicesList = (json && json.data) ? json.data : [];
    renderWoLDevices();
  } catch (err) {
    console.error('Error loading WoL devices:', err);
  }
}

function renderWoLDevices() {
  const tbody = document.getElementById('wol-devices-table-body');
  const chipsContainer = document.getElementById('launchpad-wol-chips');

  // 1. Render Table in Modal
  if (tbody) {
    if (!wolDevicesList || wolDevicesList.length === 0) {
      tbody.innerHTML = `
        <tr>
          <td colspan="5" style="text-align:center; padding:24px; color:var(--text-muted);">
            ${t('wol_no_devices', 'Nessun dispositivo Wake-on-LAN salvato. Aggiungi il tuo PC qui sotto!')}
          </td>
        </tr>`;
    } else {
      tbody.innerHTML = wolDevicesList.map(d => {
        let lastWake = t('wol_last_wake_never', 'Mai');
        if (d.last_wake_at) {
          const dt = new Date(d.last_wake_at);
          lastWake = dt.toLocaleString();
        }
        return `
          <tr style="border-bottom:1px solid rgba(255,255,255,0.05);">
            <td style="padding:10px 12px; font-weight:600; color:var(--text-main);">
              🖥️ ${escapeHtml(d.name)}
            </td>
            <td style="padding:10px 12px; font-family:'JetBrains Mono',monospace; color:#38bdf8;">
              ${escapeHtml(d.mac_address)}
            </td>
            <td style="padding:10px 12px; color:var(--text-muted); font-size:11.5px;">
              ${escapeHtml(d.broadcast_ip || '255.255.255.255')}:${d.port || 9}
            </td>
            <td style="padding:10px 12px; color:var(--text-muted); font-size:11.5px;">
              ${lastWake}
            </td>
            <td style="padding:10px 12px; text-align:right;">
              <div style="display:flex; justify-content:flex-end; gap:6px;">
                <button class="btn btn-sm btn-warning" id="btn-wol-wake-${d.id}" onclick="wakeDevice(${d.id}, '${escapeHtml(d.mac_address)}', '${escapeHtml(d.name)}')" style="font-weight:700; padding:4px 10px; font-size:11.5px;">
                  ⚡ ${t('wol_btn_wake_now', 'Accendi')}
                </button>
                <button class="btn btn-sm btn-outline-info" onclick="editWoLDevice(${d.id})" title="Modifica" style="padding:4px 8px; font-size:11px;">
                  ✏️
                </button>
                <button class="btn btn-sm btn-outline-danger" onclick="deleteWoLDevice(${d.id})" title="Elimina" style="padding:4px 8px; font-size:11px;">
                  🗑️
                </button>
              </div>
            </td>
          </tr>`;
      }).join('');
    }
  }

  // 2. Render Quick Chips in Launchpad
  if (chipsContainer) {
    if (!wolDevicesList || wolDevicesList.length === 0) {
      chipsContainer.innerHTML = `
        <div style="font-size:12px; color:var(--text-muted); display:flex; align-items:center; gap:8px;">
          <span>💡 Salva il MAC address del tuo PC principale per accenderlo con 1 clic:</span>
          <button class="btn btn-sm btn-outline-warning" onclick="openWoLModal()" style="font-size:11.5px; padding:2px 8px;">
            + Salva PC
          </button>
        </div>`;
    } else {
      chipsContainer.innerHTML = wolDevicesList.map(d => {
        let lastWakeStr = '';
        if (d.last_wake_at) {
          const dt = new Date(d.last_wake_at);
          lastWakeStr = `<span style="font-size:10px; color:var(--text-muted); margin-left:4px;">(ultimo: ${dt.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})})</span>`;
        }
        return `
          <div style="display:flex; align-items:center; justify-content:space-between; gap:12px; background:rgba(15,23,42,0.8); border:1px solid rgba(245,158,11,0.25); border-radius:8px; padding:8px 12px; min-width:240px; flex:1;">
            <div style="display:flex; align-items:center; gap:8px; overflow:hidden;">
              <span style="font-size:18px;">🖥️</span>
              <div style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
                <div style="font-size:12.5px; font-weight:700; color:var(--text-main);">${escapeHtml(d.name)}</div>
                <div style="font-size:11px; font-family:'JetBrains Mono',monospace; color:var(--text-muted);">${escapeHtml(d.mac_address)} ${lastWakeStr}</div>
              </div>
            </div>
            <button class="btn btn-sm btn-warning" id="btn-quick-wake-${d.id}" onclick="wakeDevice(${d.id}, '${escapeHtml(d.mac_address)}', '${escapeHtml(d.name)}')" style="font-weight:700; padding:6px 12px; font-size:12px; white-space:nowrap; box-shadow:0 2px 8px rgba(245,158,11,0.25);">
              ⚡ ${t('wol_btn_wake_now', 'Accendi')}
            </button>
          </div>`;
      }).join('');
    }
  }
}

async function wakeDevice(id, mac, name) {
  const btn1 = document.getElementById(`btn-wol-wake-${id}`);
  const btn2 = document.getElementById(`btn-quick-wake-${id}`);
  const origText1 = btn1 ? btn1.innerHTML : '';
  const origText2 = btn2 ? btn2.innerHTML : '';

  if (btn1) { btn1.disabled = true; btn1.innerHTML = '⏳ ' + t('wol_btn_wake_sending', 'Invio...'); }
  if (btn2) { btn2.disabled = true; btn2.innerHTML = '⏳ ' + t('wol_btn_wake_sending', 'Invio...'); }

  try {
    const payload = id ? { id: Number(id) } : { mac_address: mac };
    const res = await fetch('/api/wol/wake', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    const json = await res.json();
    if (!res.ok || json.status !== 'ok') {
      throw new Error(json.error || `HTTP ${res.status}`);
    }

    const ifaces = (json.data && json.data.interfaces) ? json.data.interfaces.join(', ') : 'LAN';
    const targetName = name || mac;
    const msg = `⚡ <b>Magic Packet inviato con successo a ${escapeHtml(targetName)}!</b><br><small style="opacity:0.9;">MAC: ${escapeHtml(json.data.mac_address)} &bull; Destinazione: ${escapeHtml(json.data.broadcast_ip)}:${json.data.port} &bull; Rete: ${escapeHtml(ifaces)}</small>`;

    showWoLFeedback(msg, 'success');
    if (typeof showAlert === 'function') {
      showAlert(`✓ Magic packet inviato a ${targetName}!`, 'success');
    }

    // Refresh devices to update last_wake_at timestamp
    loadWoLDevices();
  } catch (err) {
    const errMsg = `❌ Errore durante l'accensione: ${err.message}`;
    showWoLFeedback(errMsg, 'danger');
  } finally {
    if (btn1) { btn1.disabled = false; btn1.innerHTML = origText1; }
    if (btn2) { btn2.disabled = false; btn2.innerHTML = origText2; }
  }
}

async function wakeAdhocMAC() {
  const input = document.getElementById('wol-adhoc-mac');
  const btn = document.getElementById('btn-wake-adhoc');
  if (!input) return;

  const mac = input.value.trim();
  if (!mac) {
    showWoLFeedback('Inserisci un indirizzo MAC valido (es. 00:D8:61:33:0E:1F)', 'warning');
    input.focus();
    return;
  }

  const origText = btn ? btn.innerHTML : '';
  if (btn) { btn.disabled = true; btn.innerHTML = '⏳ ' + t('wol_btn_wake_sending', 'Invio...'); }

  try {
    const res = await fetch('/api/wol/wake', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mac_address: mac })
    });

    const json = await res.json();
    if (!res.ok || json.status !== 'ok') {
      throw new Error(json.error || `HTTP ${res.status}`);
    }

    const ifaces = (json.data && json.data.interfaces) ? json.data.interfaces.join(', ') : 'LAN';
    const msg = `⚡ <b>Magic Packet inviato con successo a ${escapeHtml(json.data.mac_address)}!</b><br><small style="opacity:0.9;">Destinazione: ${escapeHtml(json.data.broadcast_ip)}:${json.data.port} &bull; Rete: ${escapeHtml(ifaces)}</small>`;
    showWoLFeedback(msg, 'success');
    if (typeof showAlert === 'function') {
      showAlert(`✓ Magic packet inviato a ${mac}!`, 'success');
    }
  } catch (err) {
    showWoLFeedback(`❌ Errore invio Magic Packet: ${err.message}`, 'danger');
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = origText; }
  }
}

async function handleWoLSave(e) {
  if (e) e.preventDefault();
  const idInput = document.getElementById('wol-device-id');
  const nameInput = document.getElementById('wol-device-name');
  const macInput = document.getElementById('wol-device-mac');
  const bcastInput = document.getElementById('wol-device-broadcast');
  const portInput = document.getElementById('wol-device-port');

  const id = idInput && idInput.value ? Number(idInput.value) : 0;
  const name = nameInput ? nameInput.value.trim() : '';
  const mac = macInput ? macInput.value.trim() : '';
  const bcast = bcastInput ? bcastInput.value.trim() : '255.255.255.255';
  const port = portInput && portInput.value ? Number(portInput.value) : 9;

  if (!name || !mac) {
    showWoLFeedback('Inserisci sia il nome che il MAC address.', 'warning');
    return;
  }

  try {
    const res = await fetch('/api/wol/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id: id,
        name: name,
        mac_address: mac,
        broadcast_ip: bcast || '255.255.255.255',
        port: port || 9
      })
    });

    const json = await res.json();
    if (!res.ok || json.status !== 'ok') {
      throw new Error(json.error || `HTTP ${res.status}`);
    }

    showWoLFeedback(`✓ Dispositivo <b>${escapeHtml(name)}</b> salvato con successo!`, 'success');
    resetWoLForm();
    await loadWoLDevices();
  } catch (err) {
    showWoLFeedback(`❌ Errore durante il salvataggio: ${err.message}`, 'danger');
  }
}

function editWoLDevice(id) {
  const d = wolDevicesList.find(x => x.id === id);
  if (!d) return;

  const idInput = document.getElementById('wol-device-id');
  const nameInput = document.getElementById('wol-device-name');
  const macInput = document.getElementById('wol-device-mac');
  const bcastInput = document.getElementById('wol-device-broadcast');
  const portInput = document.getElementById('wol-device-port');
  const saveBtn = document.getElementById('btn-save-wol-device');

  if (idInput) idInput.value = d.id;
  if (nameInput) nameInput.value = d.name;
  if (macInput) macInput.value = d.mac_address;
  if (bcastInput) bcastInput.value = d.broadcast_ip || '255.255.255.255';
  if (portInput) portInput.value = d.port || 9;
  if (saveBtn) saveBtn.innerHTML = '💾 ' + t('wol_btn_save_changes', 'Salva Modifiche');

  if (nameInput) nameInput.focus();
}

function resetWoLForm() {
  const form = document.getElementById('wol-add-form');
  if (form) form.reset();
  const idInput = document.getElementById('wol-device-id');
  if (idInput) idInput.value = '';
  const portInput = document.getElementById('wol-device-port');
  if (portInput) portInput.value = 9;
  const saveBtn = document.getElementById('btn-save-wol-device');
  if (saveBtn) saveBtn.innerHTML = '💾 ' + t('wol_btn_save', 'Salva Dispositivo');
}

async function deleteWoLDevice(id) {
  const d = wolDevicesList.find(x => x.id === id);
  const name = d ? d.name : `#${id}`;
  if (!confirm(`${t('wol_confirm_delete', 'Sei sicuro di voler rimuovere questo dispositivo?')}\n(${name})`)) {
    return;
  }

  try {
    const res = await fetch(`/api/wol/devices?id=${id}`, {
      method: 'DELETE'
    });
    const json = await res.json();
    if (!res.ok || json.status !== 'ok') {
      throw new Error(json.error || `HTTP ${res.status}`);
    }

    showWoLFeedback(`✓ Dispositivo <b>${escapeHtml(name)}</b> eliminato.`, 'info');
    await loadWoLDevices();
  } catch (err) {
    showWoLFeedback(`❌ Errore durante l'eliminazione: ${err.message}`, 'danger');
  }
}

// WoL Window Bindings
window.openWoLModal = openWoLModal;
window.closeWoLModal = closeWoLModal;
window.loadWoLDevices = loadWoLDevices;
window.renderWoLDevices = renderWoLDevices;
window.wakeDevice = wakeDevice;
window.wakeAdhocMAC = wakeAdhocMAC;
window.handleWoLSave = handleWoLSave;
window.editWoLDevice = editWoLDevice;
window.resetWoLForm = resetWoLForm;
window.deleteWoLDevice = deleteWoLDevice;

// ==========================================================================
// Historical Telemetry & Lightweight Native HTML5 Canvas Metrics
// ==========================================================================
let currentMetricsRange = '24h';
let currentMetricsData = null;
let currentMetricsLoading = false;

function hexToRgba(hex, alpha) {
  let c = hex.replace('#', '');
  if (c.length === 3) {
    c = c.split('').map(x => x + x).join('');
  }
  const num = parseInt(c, 16);
  const r = (num >> 16) & 255;
  const g = (num >> 8) & 255;
  const b = num & 255;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

async function loadMetricsHistory(rangeStr, isBackgroundRefresh = false) {
  if (rangeStr) {
    currentMetricsRange = rangeStr;
  }

  // Update button active state
  document.querySelectorAll('.metrics-range-btn').forEach(btn => {
    if (btn.dataset.range === currentMetricsRange) {
      btn.classList.add('active');
    } else {
      btn.classList.remove('active');
    }
  });

  const loadingEl = document.getElementById('metrics-history-loading');
  const emptyEl = document.getElementById('metrics-history-empty');
  const gridEl = document.getElementById('metrics-charts-grid');

  if (!isBackgroundRefresh && (!currentMetricsData || currentMetricsData.length === 0)) {
    if (loadingEl) loadingEl.classList.remove('hidden');
    if (emptyEl) emptyEl.classList.add('hidden');
  }

  currentMetricsLoading = true;
  try {
    const res = await fetch(`/api/system/history?range=${encodeURIComponent(currentMetricsRange)}`);
    const json = await res.json();
    if (json && json.status === 'ok' && json.data) {
      currentMetricsData = json.data.points || [];
    } else {
      currentMetricsData = [];
    }
  } catch (err) {
    console.error('Error fetching metrics history:', err);
    if (!currentMetricsData) currentMetricsData = [];
  } finally {
    currentMetricsLoading = false;
    if (loadingEl) loadingEl.classList.add('hidden');
  }

  if (!currentMetricsData || currentMetricsData.length === 0) {
    if (emptyEl) emptyEl.classList.remove('hidden');
    if (gridEl) gridEl.classList.add('hidden');
  } else {
    if (emptyEl) emptyEl.classList.add('hidden');
    if (gridEl) gridEl.classList.remove('hidden');
    renderAllMetricsCharts();
  }
}

function changeMetricsRange(rangeStr) {
  if (rangeStr === currentMetricsRange && currentMetricsData && currentMetricsData.length > 0) return;
  loadMetricsHistory(rangeStr);
}

function renderAllMetricsCharts() {
  if (!currentMetricsData || currentMetricsData.length === 0) return;

  // 1. CPU Temperature Chart
  drawMetricsChart({
    canvasId: 'canvas-cpu-temp',
    wrapId: 'wrap-canvas-cpu-temp',
    statId: 'stat-cpu-temp',
    points: currentMetricsData,
    valGetter: p => p.cpu_temp,
    color: '#f59e0b',
    label: (typeof t === 'function' ? t('chart_cpu_temp_title', 'Temperatura CPU') : 'Temperatura CPU'),
    unit: '°C',
    decimals: 1,
    minValFloor: 20
  });

  // 2. CPU Usage Chart
  drawMetricsChart({
    canvasId: 'canvas-cpu-usage',
    wrapId: 'wrap-canvas-cpu-usage',
    statId: 'stat-cpu-usage',
    points: currentMetricsData,
    valGetter: p => p.cpu_usage,
    color: '#38bdf8',
    label: (typeof t === 'function' ? t('chart_cpu_usage_title', 'Utilizzo CPU') : 'Utilizzo CPU'),
    unit: '%',
    decimals: 1,
    fixedMin: 0,
    fixedMax: 100
  });

  // 3. RAM Usage Chart
  drawMetricsChart({
    canvasId: 'canvas-ram-usage',
    wrapId: 'wrap-canvas-ram-usage',
    statId: 'stat-ram-usage',
    points: currentMetricsData,
    valGetter: p => p.ram_used_mb,
    maxValGetter: p => p.ram_total_mb,
    color: '#a855f7',
    label: (typeof t === 'function' ? t('chart_ram_title', 'RAM') : 'RAM'),
    unit: 'MB',
    decimals: 0,
    fixedMin: 0
  });

  // 4. Storage Usage Chart
  drawMetricsChart({
    canvasId: 'canvas-storage-usage',
    wrapId: 'wrap-canvas-storage-usage',
    statId: 'stat-storage-usage',
    points: currentMetricsData,
    valGetter: p => (p.storage_used_bytes > 0 ? (p.storage_used_bytes / (1024 * 1024 * 1024)) : 0),
    maxValGetter: p => (p.storage_total_bytes > 0 ? (p.storage_total_bytes / (1024 * 1024 * 1024)) : 0),
    color: '#10b981',
    label: (typeof t === 'function' ? t('chart_storage_title', 'Storage') : 'Storage'),
    unit: 'GB',
    decimals: 1,
    fixedMin: 0
  });
}

function drawMetricsChart(cfg, hoverIdx = -1) {
  const canvas = document.getElementById(cfg.canvasId);
  const wrap = document.getElementById(cfg.wrapId);
  if (!canvas || !wrap) return;

  wrap._latestCfg = cfg;

  const rect = wrap.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return;

  const dpr = window.devicePixelRatio || 1;
  canvas.width = Math.floor(rect.width * dpr);
  canvas.height = Math.floor(rect.height * dpr);

  const ctx = canvas.getContext('2d');
  if (ctx.resetTransform) ctx.resetTransform();
  ctx.scale(dpr, dpr);

  const width = rect.width;
  const height = rect.height;

  const points = cfg.points || [];
  const valGetter = cfg.valGetter;
  const color = cfg.color;
  const unit = cfg.unit;
  const decimals = cfg.decimals !== undefined ? cfg.decimals : 1;

  // Extract valid numerical points
  const valid = [];
  for (let i = 0; i < points.length; i++) {
    const v = valGetter(points[i]);
    if (typeof v === 'number' && !isNaN(v)) {
      valid.push({ t: points[i].t, v: v, raw: points[i], origIdx: i });
    }
  }

  if (valid.length === 0) return;

  // Statistics calculation
  const values = valid.map(p => p.v);
  const minVal = Math.min(...values);
  const maxVal = Math.max(...values);
  const latestVal = valid[valid.length - 1].v;
  const avgVal = values.reduce((a, b) => a + b, 0) / values.length;

  // Update stat summary header
  const statEl = document.getElementById(cfg.statId);
  if (statEl) {
    let maxTotalText = '';
    if (cfg.maxValGetter) {
      const maxTotal = cfg.maxValGetter(valid[valid.length - 1].raw);
      if (maxTotal && maxTotal > 0) {
        maxTotalText = ` / ${maxTotal.toFixed(decimals)} ${unit}`;
      }
    }
    const avgLabel = typeof t === 'function' ? t('metrics_avg', 'Media') : 'Media';
    const maxLabel = typeof t === 'function' ? t('metrics_max', 'Max') : 'Max';
    statEl.innerHTML = `<span style="color:${color}">${latestVal.toFixed(decimals)} ${unit}${maxTotalText}</span>` +
      `<span style="font-size:10px; color:var(--text-muted); font-weight:normal; margin-left:6px;">(${avgLabel}: ${avgVal.toFixed(decimals)}${unit} | ${maxLabel}: ${maxVal.toFixed(decimals)}${unit})</span>`;
  }

  // Padding
  const padLeft = 40;
  const padRight = 12;
  const padTop = 15;
  const padBottom = 20;

  const chartW = width - padLeft - padRight;
  const chartH = height - padTop - padBottom;
  if (chartW <= 0 || chartH <= 0) return;

  // Y-axis bounds
  let yMin = cfg.fixedMin !== undefined ? cfg.fixedMin : minVal;
  let yMax = cfg.fixedMax !== undefined ? cfg.fixedMax : maxVal;

  if (cfg.maxValGetter) {
    const lastTotal = cfg.maxValGetter(valid[valid.length - 1].raw);
    if (lastTotal && lastTotal > 0) {
      yMax = Math.max(yMax, lastTotal);
    }
  }

  if (cfg.fixedMin === undefined) {
    const margin = (yMax - yMin) * 0.12 || 2;
    yMin = Math.max(cfg.minValFloor !== undefined ? cfg.minValFloor : 0, Math.floor(yMin - margin));
    yMax = Math.ceil(yMax + margin);
  }

  if (yMax <= yMin) {
    yMax = yMin + 1;
  }

  // Time bounds
  const tMin = valid[0].t;
  const tMax = valid[valid.length - 1].t;
  const tSpan = (tMax - tMin) || 1;

  const getX = t => padLeft + ((t - tMin) / tSpan) * chartW;
  const getY = v => padTop + chartH - ((v - yMin) / (yMax - yMin)) * chartH;

  // Horizontal Grid Lines & Y-labels
  ctx.save();
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
  ctx.fillStyle = 'rgba(255, 255, 255, 0.45)';
  ctx.font = '9px monospace';
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  ctx.lineWidth = 1;

  const gridSteps = 3;
  for (let s = 0; s <= gridSteps; s++) {
    const frac = s / gridSteps;
    const yVal = yMin + frac * (yMax - yMin);
    const yCoord = padTop + chartH - frac * chartH;

    ctx.beginPath();
    ctx.moveTo(padLeft, yCoord);
    ctx.lineTo(padLeft + chartW, yCoord);
    ctx.stroke();

    ctx.fillText(`${yVal.toFixed(decimals > 0 && yVal < 10 ? 1 : 0)}${unit}`, padLeft - 6, yCoord);
  }

  // Time Labels on X-axis
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  const timeSteps = 3;
  for (let s = 0; s <= timeSteps; s++) {
    const frac = s / timeSteps;
    const tVal = tMin + frac * tSpan;
    const xCoord = padLeft + frac * chartW;

    const date = new Date(tVal * 1000);
    let timeLabel = '';
    if (currentMetricsRange === '1h' || currentMetricsRange === '24h') {
      timeLabel = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
    } else {
      timeLabel = `${date.getDate()}/${date.getMonth() + 1} ${date.getHours().toString().padStart(2, '0')}:00`;
    }

    if (s === 0) ctx.textAlign = 'left';
    else if (s === timeSteps) ctx.textAlign = 'right';
    else ctx.textAlign = 'center';

    ctx.fillText(timeLabel, xCoord, padTop + chartH + 6);
  }
  ctx.restore();

  // Plot Points
  const mappedPoints = valid.map(p => ({
    x: getX(p.t),
    y: getY(p.v),
    t: p.t,
    v: p.v
  }));
  wrap._chartPoints = mappedPoints;

  // Gradient Fill Area
  const grad = ctx.createLinearGradient(0, padTop, 0, padTop + chartH);
  grad.addColorStop(0, hexToRgba(color, 0.35));
  grad.addColorStop(1, hexToRgba(color, 0.01));

  ctx.beginPath();
  ctx.moveTo(mappedPoints[0].x, padTop + chartH);
  ctx.lineTo(mappedPoints[0].x, mappedPoints[0].y);
  for (let i = 1; i < mappedPoints.length; i++) {
    ctx.lineTo(mappedPoints[i].x, mappedPoints[i].y);
  }
  ctx.lineTo(mappedPoints[mappedPoints.length - 1].x, padTop + chartH);
  ctx.closePath();
  ctx.fillStyle = grad;
  ctx.fill();

  // Draw Line
  ctx.save();
  ctx.beginPath();
  ctx.moveTo(mappedPoints[0].x, mappedPoints[0].y);
  for (let i = 1; i < mappedPoints.length; i++) {
    ctx.lineTo(mappedPoints[i].x, mappedPoints[i].y);
  }
  ctx.strokeStyle = color;
  ctx.lineWidth = 2;
  ctx.shadowColor = color;
  ctx.shadowBlur = 4;
  ctx.stroke();
  ctx.restore();

  // Hover Crosshair & Point Highlight
  if (hoverIdx >= 0 && hoverIdx < mappedPoints.length) {
    const hp = mappedPoints[hoverIdx];

    // Vertical dashed guideline
    ctx.save();
    ctx.beginPath();
    ctx.setLineDash([3, 3]);
    ctx.moveTo(hp.x, padTop);
    ctx.lineTo(hp.x, padTop + chartH);
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.45)';
    ctx.lineWidth = 1;
    ctx.stroke();

    // Outer glow ring
    ctx.beginPath();
    ctx.arc(hp.x, hp.y, 6, 0, Math.PI * 2);
    ctx.fillStyle = hexToRgba(color, 0.35);
    ctx.fill();

    // Inner bright point
    ctx.beginPath();
    ctx.arc(hp.x, hp.y, 3.5, 0, Math.PI * 2);
    ctx.fillStyle = '#ffffff';
    ctx.fill();
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.restore();
  }

  // Bind mouse/touch events once per wrapper
  if (!wrap._hasHoverListener) {
    wrap._hasHoverListener = true;

    const onPointerMove = (e) => {
      const activeCfg = wrap._latestCfg || cfg;
      const pts = wrap._chartPoints;
      if (!pts || pts.length === 0) return;

      const rect = wrap.getBoundingClientRect();
      const clientX = e.touches ? e.touches[0].clientX : e.clientX;
      const clientY = e.touches ? e.touches[0].clientY : e.clientY;
      const mouseX = clientX - rect.left;

      // Find closest point by x coordinate
      let closestIdx = 0;
      let minDiff = Infinity;
      for (let i = 0; i < pts.length; i++) {
        const diff = Math.abs(pts[i].x - mouseX);
        if (diff < minDiff) {
          minDiff = diff;
          closestIdx = i;
        }
      }

      // Re-draw with highlighted index
      drawMetricsChart(activeCfg, closestIdx);

      // Position and show tooltip
      const tooltip = document.getElementById('metrics-chart-tooltip');
      if (tooltip) {
        const pt = pts[closestIdx];
        const date = new Date(pt.t * 1000);
        const dateStr = date.toLocaleDateString([], { day: '2-digit', month: '2-digit', year: 'numeric' });
        const timeStr = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });

        tooltip.innerHTML = `
          <div style="font-weight:700; font-size:12px; color:${activeCfg.color}; margin-bottom:2px;">
            ${pt.v.toFixed(activeCfg.decimals !== undefined ? activeCfg.decimals : 1)} ${activeCfg.unit}
          </div>
          <div style="font-size:10.5px; color:#cbd5e1;">${dateStr} ${timeStr}</div>
        `;
        tooltip.style.left = `${clientX}px`;
        tooltip.style.top = `${clientY}px`;
        tooltip.classList.remove('hidden');
      }
    };

    const onPointerLeave = () => {
      const activeCfg = wrap._latestCfg || cfg;
      drawMetricsChart(activeCfg, -1);
      const tooltip = document.getElementById('metrics-chart-tooltip');
      if (tooltip) tooltip.classList.add('hidden');
    };

    wrap.addEventListener('mousemove', onPointerMove);
    wrap.addEventListener('mouseleave', onPointerLeave);
    wrap.addEventListener('touchmove', onPointerMove, { passive: true });
    wrap.addEventListener('touchend', onPointerLeave);
  }
}

// Telemetry Metrics Window Bindings
window.loadMetricsHistory = loadMetricsHistory;
window.changeMetricsRange = changeMetricsRange;
window.renderAllMetricsCharts = renderAllMetricsCharts;
window.drawMetricsChart = drawMetricsChart;



