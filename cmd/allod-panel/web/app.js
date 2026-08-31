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
    modules: { title: 'Gestione Moduli & Livelli', sub: 'Adatta le risorse e configura i servizi del nodo' },
    ring: { title: 'Federazione del Ring', sub: 'Topologia dei 3 nodi e repliche remote per dataset' },
    resilience: { title: 'Test di Resilienza', sub: 'Simulatore di guasti, disconnessioni e rollback automatico' }
  };

  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.dataset.tab;

      tabButtons.forEach(b => b.classList.remove('active'));
      tabPanes.forEach(p => p.classList.remove('active'));

      btn.classList.add('active');
      document.getElementById(`tab-${tab}`).classList.add('active');

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

    currentStatus = statusRes.data;
    currentModules = modulesRes.data;
    currentRing = ringRes.data;

    renderOverview();
    renderModules();
    renderRing();
  } catch (err) {
    showAlert('Errore di comunicazione con il backend Allod: ' + err.message, 'danger');
  }
}

function renderOverview() {
  if (!currentStatus) return;

  document.getElementById('sidebar-node-name').textContent = currentStatus.node_name || 'allod-node';
  
  // RAM metric
  const ramUsedGB = (currentStatus.ram_used_mb / 1024).toFixed(1);
  const ramTotalGB = (currentStatus.ram_total_mb / 1024).toFixed(1);
  const ramPercent = Math.min(100, Math.round((currentStatus.ram_used_mb / currentStatus.ram_total_mb) * 100));

  document.getElementById('ram-used').textContent = `${ramUsedGB} / ${ramTotalGB} GB`;
  document.getElementById('ram-progress').style.width = `${ramPercent}%`;
  document.getElementById('ram-footer').textContent = `Riservato core: ${currentStatus.core_reserved_mb} MB | Disponibile: ${((currentStatus.ram_total_mb - currentStatus.ram_used_mb)/1024).toFixed(1)} GB`;

  // Active modules
  if (currentModules) {
    const active = currentModules.filter(m => m.current_level !== 'off');
    document.getElementById('active-modules-count').textContent = `${active.length} / ${currentModules.length}`;
    document.getElementById('active-modules-list').textContent = active.map(m => m.id).join(', ');
  }

  // Visual Nodes
  const visualGrid = document.getElementById('nodes-visual-grid');
  visualGrid.innerHTML = '';

  if (currentRing && currentRing.members) {
    Object.values(currentRing.members).forEach(m => {
      const isSelf = m.id === currentStatus.node_name;
      const box = document.createElement('div');
      box.className = 'node-item-box';
      box.innerHTML = `
        <div class="node-item-icon">${isSelf ? '🏠' : '🤝'}</div>
        <div class="node-item-info">
          <h4>${m.id} ${isSelf ? '<span class="badge badge-info">Locale</span>' : ''}</h4>
          <p>IP: <code>${m.address}</code> | Quota: <strong>${m.quota_gb} GB</strong></p>
        </div>
      `;
      visualGrid.appendChild(box);
    });
  }
}

function renderModules() {
  const container = document.getElementById('modules-grid');
  if (!container || !currentModules) return;

  container.innerHTML = '';

  currentModules.forEach(mod => {
    const card = document.createElement('div');
    card.className = 'module-card';

    const isOff = mod.current_level === 'off';
    const tierBadge = mod.tier === 'core' ? 'badge-primary' : 'badge-info';

    let levelOptions = Object.keys(mod.manifest.levels).map(lvl => {
      const selected = lvl === mod.current_level ? 'selected' : '';
      return `<option value="${lvl}" ${selected}>${lvl}</option>`;
    }).join('');

    const currentLevelInfo = mod.manifest.levels[mod.current_level] || {};
    const grants = currentLevelInfo.grants || [];
    const ramReq = currentLevelInfo.ram_mb || 0;

    let grantsHtml = grants.length > 0
      ? `<ul class="grants-list">${grants.map(g => `<li>✓ ${g}</li>`).join('')}</ul>`
      : `<p class="text-muted font-mono" style="font-size:11px; margin-top:8px;">Modulo disattivato</p>`;

    card.innerHTML = `
      <div>
        <div class="module-card-header">
          <div>
            <div class="module-title">
              ${mod.id}
              <span class="badge ${tierBadge}">${mod.tier}</span>
            </div>
            <div class="module-meta">${mod.manifest.provides ? mod.manifest.provides.join(', ') : ''}</div>
          </div>
          <span class="badge ${isOff ? 'badge-warning' : 'badge-success'}">${isOff ? 'OFF' : 'ATTIVO'}</span>
        </div>

        <div class="level-selector-row">
          <label style="font-size:12px; color:var(--text-muted);">Livello:</label>
          <select class="form-control" onchange="changeModuleLevel('${mod.id}', this.value)" style="padding:4px 8px; font-size:12px;">
            ${levelOptions}
          </select>
          <span class="badge badge-info" style="font-size:11px;">${ramReq} MB RAM</span>
        </div>

        ${grantsHtml}
      </div>

      <div style="font-size:11px; color:var(--text-muted); border-top:1px solid var(--card-border); padding-top:8px; display:flex; justify-content:space-between;">
        <span>UserNS: ${mod.manifest.privileges ? mod.manifest.privileges.userns : 'rootless'}</span>
        <span>${mod.manifest.images && mod.manifest.images.length > 0 ? `${mod.manifest.images.length} Container` : 'Nativo'}</span>
      </div>
    `;

    container.appendChild(card);
  });
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

  // Members
  const membersContainer = document.getElementById('ring-members-container');
  membersContainer.innerHTML = '';

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
      ${Object.values(currentRing.members).map(m => `
        <tr>
          <td><strong>${m.id}</strong></td>
          <td><code>${m.address}</code></td>
          <td>${m.quota_gb} GB</td>
          <td>${m.datasets ? m.datasets.length : 0}</td>
        </tr>
      `).join('')}
    </tbody>
  `;
  membersContainer.appendChild(memberTable);

  // Placements
  const datasetsContainer = document.getElementById('ring-datasets-container');
  datasetsContainer.innerHTML = '';

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
      ${Object.entries(currentRing.placements).map(([key, p]) => `
        <tr>
          <td><code>${key}</code> ${p.Critical ? '<span class="badge badge-warning">Critico</span>' : ''}</td>
          <td>${p.SizeGB} GB</td>
          <td>${p.TargetNodes.map(t => `<span class="badge badge-info">${t}</span>`).join(' ')}</td>
          <td><span class="badge badge-success">${p.Status}</span></td>
        </tr>
      `).join('')}
    </tbody>
  `;
  datasetsContainer.appendChild(placementTable);
}

async function runSimulateRemoval() {
  const member = document.getElementById('simulate-member-select').value;
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
    const impact = data.data;

    box.innerHTML = `
      <div style="color:var(--warning); font-weight:600; margin-bottom:8px;">⚠️ ${impact.QuorumHealth}</div>
      <div style="margin-bottom:6px;"><strong>Dataset del membro persi:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px;">
        ${impact.LostPrimaryDatasets.map(d => `<li>• ${d}</li>`).join('')}
      </ul>
      <div style="margin-bottom:6px;"><strong>Dataset degradati da riallocare:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px;">
        ${impact.DegradedDatasets.map(d => `<li>⚠️ ${d}</li>`).join('')}
      </ul>
      <div style="margin-bottom:6px;"><strong>Piano di Riallocazione Automatica:</strong></div>
      <ul style="padding-left:16px; margin-bottom:8px; color:var(--primary);">
        ${impact.RebalanceActions.map(a => `<li>-> ${a}</li>`).join('')}
      </ul>
      <div style="font-size:11px; color:var(--text-muted);">Capacità Residua: ${impact.TotalUsedRemaining} GB usati su ${impact.TotalQuotaRemaining} GB totali.</div>
    `;
  } catch (err) {
    box.innerHTML = `<p style="color:var(--danger)">Errore simulazione: ${err.message}</p>`;
  }
}

async function runUpdateSimulation(fail) {
  const modName = document.getElementById('update-module-select').value;
  const targetTag = document.getElementById('update-tag-input').value;
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
    const report = data.data;

    let stepsHtml = report.steps.map(s => {
      const badgeClass = s.state === 'COMMITTED' || s.state === 'ROLLED_BACK' ? 'badge-success'
        : s.state === 'ROLLING_BACK' || s.state === 'FAILED' ? 'badge-danger' : 'badge-info';
      return `<div class="step-log-item"><span class="badge ${badgeClass}">${s.state}</span> ${s.message}</div>`;
    }).join('');

    box.innerHTML = `
      <div style="margin-bottom:10px; font-weight:600; color:${report.success ? 'var(--success)' : 'var(--warning)'}">
        ${report.success ? '✅ AGGIORNAMENTO COMPLETATO CON SUCCESSO' : '⚠️ HEALTHCHECK FALLITO: ROLLBACK ESEGUITO CON SUCCESSO'}
      </div>
      ${stepsHtml}
    `;
  } catch (err) {
    box.innerHTML = `<p style="color:var(--danger)">Errore: ${err.message}</p>`;
  }
}

function showAlert(msg, type) {
  const banner = document.getElementById('alert-banner');
  banner.className = `alert-banner alert-${type}`;
  banner.textContent = msg;
  banner.classList.remove('hidden');
  setTimeout(() => banner.classList.add('hidden'), 5000);
}
