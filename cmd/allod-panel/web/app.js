// Allod Panel SPA Logic
let currentStatus = null;
let currentModules = null;
let currentRing = null;
let unlockedModules = new Set();
let startingModules = new Map(); // modID -> timestamp

document.addEventListener('DOMContentLoaded', () => {
  setupTabs();
  if (typeof setLanguage === 'function') {
    setLanguage(currentLang);
  }
  refreshData();
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
      helperPill.innerHTML = `<span class="status-indicator"></span> ${t('helper_connected')}`;
    } else {
      helperPill.className = 'helper-status offline';
      helperPill.innerHTML = `<span class="status-indicator offline"></span> ${t('helper_offline')}`;
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
    ramFootEl.textContent = (currentLang === 'it')
      ? `Riservato core: ${currentStatus.core_reserved_mb || 600} MB | Disponibile: ${freeGB} GB`
      : `Core reserved: ${currentStatus.core_reserved_mb || 600} MB | Available: ${freeGB} GB`;
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
      statusBadge = `<span class="badge badge-success">${t('status_protected')}</span>`;
      
      let diagBtn = (mod.id === 'storage')
        ? `<button class="btn btn-sm btn-info" onclick="showStorageDiagnostics()" style="padding:2px 8px; font-size:11px;">🔍 ${t('btn_inspect_btrfs', 'Inspect Btrfs')}</button>`
        : `<button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px;">🩺 ${t('btn_diagnostics')}</button>`;

      actionButtons = `
        ${diagBtn}
        <button class="btn btn-sm btn-outline-secondary" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">${t('btn_unlock')}</button>
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
        ${mod.id !== 'storage' ? `<button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">${t('btn_stop')}</button>` : ''}
        ${diagBtn}
        <button class="btn btn-sm btn-warning" onclick="toggleModuleUnlock('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">${t('btn_lock')}</button>
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
          <button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">${t('btn_stop')}</button>
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
          <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">${t('btn_restart')}</button>
          <button class="btn btn-sm btn-outline-info" onclick="showModuleDiagnostics('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">🩺 ${t('btn_diagnostics')}</button>
          <button class="btn btn-sm btn-outline-danger" onclick="openDangerPurgeModal('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:4px;">${t('btn_reset')}</button>
        `;
      } else {
        statusBadge = `<span class="badge badge-warning">${t('status_stopped')}</span>`;
        actionButtons = `
          <button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">${t('btn_start')}</button>
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
  if (startingModules.has(modID)) {
    showAlert(`Avvio del modulo '${modID}' già in corso...`, 'info');
    return;
  }
  startingModules.set(modID, Date.now());
  renderModules();

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
      
      report += `📦 Container Morti/Arrestati Rimossi:\n`;
      report += (d.containers_pruned && d.containers_pruned.length > 0) ? `${d.containers_pruned}\n\n` : `Nessun container morto da rimuovere (Sistema pulito).\n\n`;

      report += `🖼️ Layer Immagini Orfane Rimossi:\n`;
      report += (d.images_pruned && d.images_pruned.length > 0) ? `${d.images_pruned}\n\n` : `Nessun layer orfano da rimuovere.\n\n`;

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
      btn.textContent = '🧹 Esegui Nuova Scansione';
    }
  }
}

// SPEEDTEST BENCHMARK LOGIC
function openSpeedtestModal() {
  const modal = document.getElementById('speedtest-modal');
  const ipSpan = document.getElementById('speedtest-server-ip');
  if (ipSpan) {
    ipSpan.textContent = window.location.hostname || '192.168.0.122';
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

    // 2. DOWNLOAD TEST
    if (verdictEl) verdictEl.textContent = '📥 Test velocità download (Server ➔ Questo dispositivo)...';
    const dlStart = performance.now();
    const dlRes = await fetch('/api/speedtest/download?t=' + Date.now());
    const reader = dlRes.body.getReader();
    let dlBytes = 0;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      dlBytes += value.length;
      const curDurationSec = (performance.now() - dlStart) / 1000;
      if (curDurationSec > 0.3) {
        const liveMbps = ((dlBytes * 8) / (curDurationSec * 1000 * 1000)).toFixed(1);
        const liveMBps = (dlBytes / (curDurationSec * 1024 * 1024)).toFixed(1);
        dlValEl.textContent = `${liveMbps} Mbps`;
        dlSubEl.textContent = `${liveMBps} MB/s`;
      }
    }
    const dlTotalSec = (performance.now() - dlStart) / 1000;
    const finalDlMbps = (dlBytes * 8) / (dlTotalSec * 1000 * 1000);
    const finalDlMBps = dlBytes / (dlTotalSec * 1024 * 1024);
    dlValEl.textContent = `${finalDlMbps.toFixed(1)} Mbps`;
    dlSubEl.textContent = `${finalDlMBps.toFixed(1)} MB/s`;
    if (pFill) pFill.style.width = '70%';

    // 3. UPLOAD TEST
    if (verdictEl) verdictEl.textContent = '📤 Test velocità upload (Questo dispositivo ➔ Server)...';
    const uploadSize = 12 * 1024 * 1024; // 12 MB payload
    const uploadData = new Uint8Array(uploadSize);
    for (let i = 0; i < uploadSize; i += 1024) {
      uploadData[i] = i % 255;
    }

    const ulStart = performance.now();
    const ulRes = await fetch('/api/speedtest/upload', {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body: uploadData
    });
    const ulJson = await ulRes.json();
    const ulTotalSec = (performance.now() - ulStart) / 1000;
    const finalUlMbps = ulJson.mbps || ((uploadSize * 8) / (ulTotalSec * 1000 * 1000));
    const finalUlMBps = (uploadSize / (ulTotalSec * 1024 * 1024));
    ulValEl.textContent = `${finalUlMbps.toFixed(1)} Mbps`;
    ulSubEl.textContent = `${finalUlMBps.toFixed(1)} MB/s`;
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
      vText += `⚠️ <strong>Nota:</strong> Se sei su Wi-Fi, avvicinati al router o usa un cavo Ethernet per massimizzare le prestazioni.`;
    }

    if (verdictEl) {
      verdictEl.innerHTML = vText.replace(/\n/g, '<br>');
    }
  } catch (err) {
    if (verdictEl) verdictEl.textContent = 'Errore durante il test di velocità: ' + err.message;
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = '⚡ Ripeti Test Velocità';
    }
    setTimeout(() => {
      if (pBar) pBar.style.display = 'none';
    }, 2000);
  }
}
