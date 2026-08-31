// Allod Panel SPA Logic
let currentStatus = null;
let currentModules = null;
let currentRing = null;
let unlockedModules = new Set();

document.addEventListener('DOMContentLoaded', () => {
  setupTabs();
  refreshData();
});

function switchToTab(tab) {
  const tabButtons = document.querySelectorAll('.nav-item');
  const tabPanes = document.querySelectorAll('.tab-pane');
  const pageTitle = document.getElementById('page-title');
  const pageSubtitle = document.getElementById('page-subtitle');

  const titles = {
    overview: { title: 'Panoramica del Nodo', sub: 'Stato del sistema, risorse e sicurezza' },
    modules: { title: 'Gestione Moduli & Servizi', sub: 'Adatta le risorse, avvia e ferma i container' },
    ring: { title: 'Federazione del Ring', sub: 'Topologia del gruppo e repliche remote per dataset' },
    resilience: { title: 'Test di Resilienza', sub: 'Simulatore di guasti, disconnessioni e rollback automatico' }
  };

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

  if (titles[tab]) {
    if (pageTitle) pageTitle.textContent = titles[tab].title;
    if (pageSubtitle) pageSubtitle.textContent = titles[tab].sub;
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

    renderOverview();
    renderModules();
    renderRing();
    renderResilience();
  } catch (err) {
    showAlert('Errore di comunicazione con il backend Allod: ' + err.message, 'danger');
  }
}

function renderOverview() {
  if (!currentStatus) return;

  // Helper Root connection pill
  const helperPill = document.getElementById('helper-status-pill');
  if (helperPill) {
    if (currentStatus.helper_connected) {
      helperPill.className = 'helper-status';
      helperPill.innerHTML = '<span class="status-indicator"></span> Helper Root: Connesso';
    } else {
      helperPill.className = 'helper-status offline';
      helperPill.innerHTML = '<span class="status-indicator offline"></span> Helper Root: Non Avviato';
    }
  }

  const sidebarName = document.getElementById('sidebar-node-name');
  if (sidebarName) sidebarName.textContent = currentStatus.node_name || 'allod-node';
  
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
    const freeGB = Math.max(0, (ramTotalMB - ramUsedMB) / 1024).toFixed(1);
    ramFootEl.textContent = `Riservato core: ${currentStatus.core_reserved_mb || 600} MB | Disponibile: ${freeGB} GB`;
  }

  // Active modules
  if (Array.isArray(currentModules)) {
    const active = currentModules.filter(m => m.current_level && m.current_level !== 'off');
    const running = currentModules.filter(m => m.runtime_status === 'running');
    const actCountEl = document.getElementById('active-modules-count');
    if (actCountEl) actCountEl.textContent = `${running.length} in esecuzione (${active.length} configurati)`;
    
    const actListEl = document.getElementById('active-modules-list');
    if (actListEl) actListEl.textContent = active.map(m => m.id).join(', ');
  }

  // Ring status metric card
  const ringBadge = document.getElementById('ring-status-badge');
  const ringSubtext = document.getElementById('ring-status-subtext');
  if (currentRing) {
    const isStandalone = currentRing.is_standalone || Object.keys(currentRing.members || {}).length <= 1;
    if (isStandalone) {
      if (ringBadge) ringBadge.textContent = '1 Nodo (Locale)';
      if (ringSubtext) ringSubtext.textContent = 'Modalità Standalone (0 peer remoti)';
    } else {
      const count = Object.keys(currentRing.members || {}).length;
      if (ringBadge) ringBadge.textContent = `${count} Nodi (OK)`;
      if (ringSubtext) ringSubtext.textContent = 'Regola 2 repliche remote attiva';
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
      if (secSubtext) secSubtext.textContent = `Utente '${currentStatus.current_user || 'non-root'}' (UID: ${currentStatus.uid || 1000})`;
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
  cloud: {
    product: 'Nextcloud Hub 30',
    desc: 'Cloud personale per file, cartelle e sincronizzazione desktop/mobile (alternativa privata a Google Drive / Dropbox).',
    db: 'SQLite (Embedded)',
    dbNote: 'Ottimale per uso personale (1 utente) e minimo consumo RAM. Per carichi multi-utente e sync massivi (>100k file) seleziona il livello \'standard\' (PostgreSQL).',
    linkPort: 8443,
    linkProtocol: 'http'
  },
  photos: {
    product: 'Immich v3.1 + PostgreSQL + Valkey',
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
  watch: {
    product: 'Allod Watchdog + WireGuard Mesh',
    desc: 'Supervisione continua dei battiti cardiaci dei nodi amici e coordinamento repliche.',
    db: 'State.db (SQLite locale)',
    dbNote: 'Tracciamento idempotente dello stato dei nodi e delle transizioni del ring.',
    linkPort: null
  }
};

function getModuleTechInfo(modID, currentLevel) {
  if (modID === 'cloud') {
    if (currentLevel === 'standard') {
      return {
        product: 'Nextcloud Hub 30 (PostgreSQL Dedicated)',
        desc: 'Cloud personale avanzato con database relazionale PostgreSQL, calendario, contatti e sync multi-utente massivo.',
        db: 'PostgreSQL 16 (Container Dedicato)',
        dbNote: 'Massime prestazioni (600 MB RAM): transazioni concorrenti isolate, affidabilità enterprise per migliaia di file e più utenti simultanei.',
        linkPort: 8443,
        linkProtocol: 'http'
      };
    }
    return {
      product: 'Nextcloud Hub 30 (SQLite Lightweight)',
      desc: 'Cloud personale per file, cartelle e sincronizzazione desktop/mobile (alternativa privata a Google Drive / Dropbox).',
      db: 'SQLite (Embedded)',
      dbNote: 'Consumo minimo (200 MB RAM), perfetto per uso personale (1 utente). Per carichi multi-utente e database PostgreSQL ad alte prestazioni, imposta il livello su \'standard\'!',
      linkPort: 8443,
      linkProtocol: 'http'
    };
  }
  if (modID === 'photos') {
    if (currentLevel === 'full') {
      return {
        product: 'Immich v3.1 (Full AI Search & Face Recognition)',
        desc: 'Galleria foto con IA avanzata: riconoscimento volti automatico, ricerca semantica vettoriale e timeline (4000 MB RAM).',
        db: 'PostgreSQL 14 + Vectorchord + Valkey',
        dbNote: 'Database vettoriale scalabile per indicizzazione IA e ricerca semantica ad altissima velocità.',
        linkPort: 2283,
        linkProtocol: 'http'
      };
    }
    return {
      product: 'Immich v3.1 + PostgreSQL + Valkey',
      desc: 'Galleria foto ad alte prestazioni con backup automatico da telefono, timeline e album (alternativa a Google Foto).',
      db: 'PostgreSQL 14 + Vectorchord + Valkey',
      dbNote: 'Database relazionale scalabile e motore vettoriale per indicizzazione e timeline ultra-veloce.',
      linkPort: 2283,
      linkProtocol: 'http'
    };
  }
  return moduleTechInfo[modID] || {
    product: modID,
    desc: 'Modulo Allod',
    db: 'Standard',
    dbNote: 'Configurazione predefinita Allod',
    linkPort: null
  };
}

function renderModules() {
  const container = document.getElementById('modules-grid');
  if (!container || !Array.isArray(currentModules)) return;

  container.innerHTML = '';

  currentModules.forEach(mod => {
    const card = document.createElement('div');
    card.className = 'module-card';

    const isOff = !mod.current_level || mod.current_level === 'off';
    const tierBadge = mod.tier === 'core' ? 'badge-primary' : 'badge-info';
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
    const isLocked = isDataCritical && isRunningOrNAS && !unlockedModules.has(mod.id);
    const isUnlocked = isDataCritical && isRunningOrNAS && unlockedModules.has(mod.id);

    if (isLocked) {
      card.className = 'module-card module-card-locked';
      statusBadge = `<span class="badge badge-success">🟢 PROTETTO (In Produzione)</span>`;
      
      let diagBtn = (mod.id === 'storage')
        ? `<button class="btn btn-sm btn-info" onclick="showStorageDiagnostics()" style="padding:2px 8px; font-size:11px;">🔍 Ispezione Btrfs</button>`
        : `<button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px;">🩺 Diagnostica</button>`;

      actionButtons = `
        ${diagBtn}
        <button class="btn btn-sm btn-outline-secondary" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🔓 Sblocca</button>
      `;

      if (tech.linkPort) {
        const host = window.location.hostname || '127.0.0.1';
        openLinkHtml = `
          <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
            <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
              <span>🌐 Apri interfaccia web (${tech.product})</span>
              <span>Porta ${tech.linkPort} ↗</span>
            </a>
          </div>
        `;
      }
    } else if (isUnlocked) {
      card.className = 'module-card';
      statusBadge = `<span class="badge badge-warning">🔓 IN ESECUZIONE (Sbloccato)</span>`;
      
      let diagBtn = (mod.id === 'storage')
        ? `<button class="btn btn-sm btn-info" onclick="showStorageDiagnostics()" style="padding:2px 8px; font-size:11px;">🔍 Ispezione Btrfs</button>`
        : `<button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px;">🩺 Diagnostica</button>`;

      actionButtons = `
        ${mod.id !== 'storage' ? `<button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">⏹ Ferma</button>` : ''}
        ${diagBtn}
        <button class="btn btn-sm btn-warning" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🔒 Blocca</button>
      `;

      if (tech.linkPort) {
        const host = window.location.hostname || '127.0.0.1';
        openLinkHtml = `
          <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
            <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
              <span>🌐 Apri interfaccia web (${tech.product})</span>
              <span>Porta ${tech.linkPort} ↗</span>
            </a>
          </div>
        `;
      }
    } else if (!isOff) {
      if (mod.runtime_status === 'running') {
        statusBadge = `<span class="badge badge-success">🟢 IN ESECUZIONE</span>`;
        actionButtons = `
          <button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">⏹ Ferma</button>
          <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 Diagnostica</button>
        `;
        
        if (tech.linkPort) {
          const host = window.location.hostname || '127.0.0.1';
          openLinkHtml = `
            <div style="margin-top:10px; padding:6px 10px; background:rgba(56,189,248,0.1); border-radius:6px; border:1px solid rgba(56,189,248,0.2);">
              <a href="${tech.linkProtocol}://${host}:${tech.linkPort}" target="_blank" style="color:var(--primary); font-weight:600; font-size:12px; text-decoration:none; display:flex; justify-content:space-between; align-items:center;">
                <span>🌐 Apri interfaccia web (${tech.product})</span>
                <span>Porta ${tech.linkPort} ↗</span>
              </a>
            </div>
          `;
        }
      } else if (mod.runtime_status === 'failed') {
        statusBadge = `<span class="badge badge-danger">🔴 ERRORE</span>`;
        actionButtons = `
          <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ Riavvia</button>
          <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 Diagnostica</button>
          <button class="btn btn-sm btn-outline-danger" onclick="openDangerPurgeModal('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🗑️ Reset</button>
        `;
      } else {
        statusBadge = `<span class="badge badge-warning">⏹ FERMATO</span>`;
        actionButtons = `
          <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ Avvia</button>
          <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 Diagnostica</button>
        `;
      }
    }

    let dbInfoHtml = ``;
    if (tech.db) {
      dbInfoHtml = `
        <div style="margin-top:8px; padding:6px 10px; background:rgba(15, 23, 42, 0.6); border-radius:6px; border-left:3px solid var(--primary); font-size:11px;">
          <div style="font-weight:600; color:var(--text-main); margin-bottom:2px;">🗄️ Database: <span style="color:var(--primary);">${tech.db}</span></div>
          <div style="color:var(--text-muted); line-height:1.35;">${tech.dbNote}</div>
        </div>
      `;
    }

    let storageBoxHtml = ``;
    if (mod.storage_path) {
      const nasBadge = mod.is_on_nas_pool 
        ? `<span class="badge badge-success" style="font-size:10px;">🟢 Pool Btrfs RAID 1</span>` 
        : `<span class="badge badge-warning" style="font-size:10px;">⚠️ Storage Locale (Home)</span>`;

      storageBoxHtml = `
        <div style="margin-top:8px; padding:7px 10px; background:rgba(15, 23, 42, 0.6); border-radius:6px; border-left:3px solid ${mod.is_on_nas_pool ? 'var(--success)' : 'var(--warning)'}; font-size:11px;">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:3px;">
            <span style="font-weight:600; color:var(--text-main);">💾 Storage NAS Fisico:</span>
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
                  <span style="color:${m.exists ? 'var(--success)' : 'var(--text-muted)'}">${m.exists ? m.size_human : 'In attesa'}</span>
                </div>
              `).join('')}
            </div>
          ` : ''}
        </div>
      `;
    }

    card.innerHTML = `
      <div>
        <div class="module-card-header">
          <div>
            <div class="module-title">
              ${mod.id}
              <span class="badge ${tierBadge}">${mod.tier || 'module'}</span>
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
          <label style="font-size:12px; color:var(--text-muted);">Livello:</label>
          <select class="form-control" ${isLocked ? 'disabled style="opacity:0.55; cursor:not-allowed; padding:4px 8px; font-size:12px;"' : 'style="padding:4px 8px; font-size:12px;"'} onchange="changeModuleLevel('${mod.id}', this.value)">
            ${levelOptions}
          </select>
          <span class="badge badge-info" style="font-size:11px;">${ramReq} MB RAM</span>
        </div>

        ${grantsHtml}
        ${dbInfoHtml}
        ${storageBoxHtml}
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

    container.appendChild(card);
  });
}

async function startModule(modID) {
  try {
    showAlert(`Richiesta avvio per '${modID}' inviata a Podman in background...`, 'info');
    const res = await fetch('/api/modules/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ module: modID })
    });
    const data = await res.json();
    if (data.status === 'ok') {
      showAlert(`✓ Avvio del modulo '${modID}' iniziato!`, 'success');
      setTimeout(refreshData, 1000);
      setTimeout(refreshData, 3000);
    } else {
      showAlert(`Errore avvio: ${data.message}`, 'danger');
    }
  } catch (err) {
    showAlert('Avvio inviato al server: aggiornamento in corso...', 'info');
    setTimeout(refreshData, 2000);
  }
}

async function stopModule(modID) {
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
    showAlert('Errore arresto modulo: ' + err.message, 'danger');
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

      <h4 style="margin:12px 0 6px 0; font-size:13px; color:var(--primary);">📊 Allocazione Btrfs Filesystem Usage (RAID 1 Real-Time):</h4>
      <div class="diag-code-box">${escapeHtml(d.usage)}</div>

      <h4 style="margin:16px 0 6px 0; font-size:13px; color:var(--success);">🛡️ Contatori di Errore Hardware Dischi (btrfs device stats):</h4>
      <div class="diag-code-box" style="color:#10b981;">${escapeHtml(d.stats)}</div>
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
    body.innerHTML = `
      <div style="margin-bottom:12px; display:flex; justify-content:space-between; align-items:center;">
        <div><strong>Modulo:</strong> <code>${d.module}</code></div>
        <div style="font-size:11px; color:var(--text-muted);">${d.timestamp}</div>
      </div>

      <h4 style="margin:12px 0 6px 0; font-size:13px; color:var(--primary);">⚙️ Stato Systemd (systemctl status ${d.module}):</h4>
      <div class="diag-code-box">${escapeHtml(d.status_text || 'Nessun output')}</div>

      <h4 style="margin:16px 0 6px 0; font-size:13px; color:var(--primary);">📜 Ultimi Log Container Podman (tail -30):</h4>
      <div class="diag-code-box" style="color:#e2e8f0;">${escapeHtml(d.logs || 'Nessun log recente')}</div>
    `;
  } catch (err) {
    body.innerHTML = `<div class="alert-banner alert-danger">Errore di rete: ${err.message}</div>`;
  }
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
  }
});

document.addEventListener('click', (e) => {
  const diagModal = document.getElementById('diagnostics-modal');
  if (diagModal && e.target === diagModal) closeDiagModal();
  const dangerModal = document.getElementById('danger-storage-modal');
  if (dangerModal && e.target === dangerModal) closeDangerStorageModal();
  const purgeModal = document.getElementById('danger-purge-modal');
  if (purgeModal && e.target === purgeModal) closeDangerPurgeModal();
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
