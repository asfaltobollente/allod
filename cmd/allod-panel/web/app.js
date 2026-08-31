// Allod Panel SPA Logic
let currentStatus = null;
let currentModules = null;
let currentRing = null;

document.addEventListener('DOMContentLoaded', () => {
  setupTabs();
  refreshData();
});

function setupTabs() {
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

  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.dataset.tab;

      tabButtons.forEach(b => b.classList.remove('active'));
      tabPanes.forEach(p => p.classList.remove('active'));

      btn.classList.add('active');
      const targetPane = document.getElementById(`tab-${tab}`);
      if (targetPane) targetPane.classList.add('active');

      if (titles[tab]) {
        pageTitle.textContent = titles[tab].title;
        pageSubtitle.textContent = titles[tab].sub;
      }
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
}

const moduleTechInfo = {
  cloud: {
    product: 'Nextcloud Hub 30',
    desc: 'Cloud personale per file, cartelle e sincronizzazione desktop/mobile (alternativa privata a Google Drive / Dropbox).',
    db: 'SQLite (Embedded)',
    dbNote: 'Ottimale per uso personale (1 utente) e minimo consumo RAM. Per carichi multi-utente e sync massivi (>100k file) è consigliato un backend PostgreSQL dedicato.',
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

    const tech = moduleTechInfo[mod.id] || {
      product: mod.id,
      desc: manifest.provides ? manifest.provides.join(', ') : 'Modulo Allod',
      db: 'Standard',
      dbNote: 'Configurazione predefinita Allod',
      linkPort: null
    };

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

    if (!isOff) {
      if (mod.runtime_status === 'running') {
        statusBadge = `<span class="badge badge-success">🟢 IN ESECUZIONE</span>`;
        actionButtons = `<button class="btn btn-sm btn-outline-danger" onclick="stopModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">⏹ Ferma</button>`;
        
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
        actionButtons = `<button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ Riavvia</button>`;
      } else {
        statusBadge = `<span class="badge badge-warning">⏹ FERMATO</span>`;
        actionButtons = `<button class="btn btn-sm btn-success" onclick="startModule('${mod.id}')" style="padding:2px 8px; font-size:11px; margin-left:8px;">▶ Avvia</button>`;
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
          <select class="form-control" onchange="changeModuleLevel('${mod.id}', this.value)" style="padding:4px 8px; font-size:12px;">
            ${levelOptions}
          </select>
          <span class="badge badge-info" style="font-size:11px;">${ramReq} MB RAM</span>
        </div>

        ${grantsHtml}
        ${dbInfoHtml}
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
