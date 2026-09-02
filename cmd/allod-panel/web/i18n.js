// Allod Sovereign Panel - i18n Translation Dictionary (English First + Italian)

const i18n = {
  en: {
    app_title: "Allod — Sovereign Personal Cloud & NAS",
    tab_overview: "Overview",
    tab_modules: "Modules & Levels",
    tab_ring: "Ring Federation",
    tab_resilience: "Resilience Tests",
    tab_settings: "Settings & Maintenance",
    mesh_subtitle: "Encrypted WireGuard Mesh",

    page_overview_title: "Node Overview",
    page_overview_sub: "System state, hardware resources, and security boundary",
    page_modules_title: "Module & Resource Management",
    page_modules_sub: "Each module adjusts resource consumption based on the selected level with instant preflight checks.",
    page_ring_title: "Multi-Node Ring Topology",
    page_ring_sub: "Each critical dataset is stored across 2 independent remote peer replicas.",
    page_resilience_title: "Resilience Testing & Disaster Simulator",
    page_resilience_sub: "Simulate real critical failure scenarios to verify Allod orchestrator behavior.",
    page_settings_title: "Settings & Maintenance",
    page_settings_sub: "Manage Allod lifecycle, Git/Go updates, and global Systemd/Podman operations.",

    btn_speedtest: "LAN Speedtest",
    btn_sweeper: "Podman Sweeper",
    btn_systemd_reload: "Systemd Reload",
    btn_refresh: "Refresh",
    msg_systemd_reloading: "Reloading systemd & Quadlet...",
    msg_systemd_reloaded: "✓ Quadlet units regenerated and systemctl daemon-reload executed!",
    helper_connected: "Root Helper: Connected",
    helper_offline: "Root Helper: Offline / Not Started",

    // Metrics
    metric_ram: "RAM Memory",
    metric_ram_footer: "Core reserved: 600 MB",
    metric_modules: "Active Modules",
    metric_ring: "Federation Ring",
    metric_security: "Security & Boundary",
    storage_pool_title: "Physical Disks & NAS Storage Pool",
    federation_health_title: "Federation Ring Health",
    federation_active_badge: "Active Quorum",
    federation_desc: "Nodes in the Ring exchange heartbeats and supervise encrypted append-only backups.",

    modules_header_title: "Module & Resource Management",
    modules_header_desc: "Each module adjusts resource consumption based on the selected level with instant preflight checks.",
    ring_header_title: "Multi-Node Ring Topology",
    ring_header_desc: "Each critical dataset is stored across 2 independent remote peer replicas.",
    ring_members_title: "Group Members",
    ring_datasets_title: "Dataset Replicas",

    resilience_header_title: "Resilience Testing & Disaster Simulator",
    resilience_header_desc: "Simulate real critical failure scenarios to verify Allod orchestrator behavior.",
    sim_disconnect_title: "Simulate Member Outage / Removal",
    sim_disconnect_desc: "What happens if a friend turns off their NAS or leaves the federation?",
    sim_disconnect_select: "Select peer member to disconnect:",
    sim_btn_rebalance: "Simulate Outage & Calculate Rebalance",
    sim_update_title: "Upgrade State Machine & Rollback",
    sim_update_desc: "Test behavior when a new container image fails startup healthcheck.",
    sim_update_module: "Module to upgrade:",
    sim_update_tag: "New Image Tag:",
    sim_btn_healthy: "Healthy Upgrade",
    sim_btn_rollback: "Simulate Crash & Rollback",

    // Settings & Maintenance
    settings_lifecycle_title: "🚀 Software Lifecycle & Self-Update",
    settings_lifecycle_desc: "Fetch latest source code from GitHub, recompile Go binaries, or perform a zero-downtime background self-update.",
    btn_git_pull: "⬇️ Git Pull",
    btn_go_build: "🔨 Go Build",
    btn_self_update: "🚀 One-Click Update & Restart",
    settings_maintenance_title: "🛠️ Global Systemd & Podman Maintenance",
    settings_maintenance_desc: "Execute administrative system actions directly from the GUI without SSH access.",
    btn_reset_failed: "🔄 Reset-Failed State",
    btn_start_active: "▶️ Start Configured Modules",
    btn_stop_all: "⏹ Stop All Modules",
    settings_start_active_note: "Starts only modules configured with an active level (modules with level 'off' remain off).",
    settings_console_title: "💻 Live Maintenance Console Output",
    settings_console_clear: "Clear Console",

    // Actions & Badges
    status_protected: "🟢 PROTECTED (In Production)",
    status_running: "🟢 RUNNING",
    status_unlocked: "🔓 RUNNING (Unlocked)",
    status_stopped: "⏹ STOPPED",
    status_error: "🔴 ERROR",
    status_starting: "⏳ STARTING...",

    tier_core: "🧱 Core",
    tier_recommended: "⭐ Recommended",
    tier_optional: "🚀 Optional",

    btn_start: "▶ Start",
    btn_restart: "▶ Restart",
    btn_stop: "⏹ Stop",
    btn_diagnostics: "🩺 Diagnostics",
    btn_unlock: "🔓 Unlock",
    btn_lock: "🔒 Lock",
    btn_reset: "🗑️ Reset",
    btn_open_web: "🌐 Open Web Interface",

    label_level: "Level:",
    label_userns: "UserNS:",
    label_native: "Native",
    label_nas_storage: "💾 Physical NAS Storage:",
    label_database: "Database",
    label_pending: "Pending",
    pool_raid1: "🟢 Btrfs RAID 1 Pool",
    pool_local: "⚠️ Local Storage (Home)",

    protected_notice: "🔒 <strong>Production Data Protected:</strong> Data and databases are persisted on the RAID 1 pool. This card is locked to prevent accidental service disruption or database changes. Click <strong>Unlock</strong> to make modifications.",

    // Sweeper
    sweeper_title: "🧹 Podman Sweeper & Clean-up",
    sweeper_desc: "Safely scan and remove <strong>exited/dead containers</strong>, <strong>dangling image layers</strong>, and <strong>stale lock files (.cid)</strong> left by crashes or restarts, <u>without ever touching active running containers or RAID disk storage</u>.",
    sweeper_ready: "Ready to scan. Click below to run the sweeper.",
    sweeper_btn_start: "🧹 Start Ghost Clean-up",
    sweeper_btn_running: "⏳ Scanning...",
    sweeper_btn_rescan: "🧹 Run New Scan",
    sweeper_close: "Close",

    // Speedtest
    speedtest_title: "⚡ LAN Network Speedtest Benchmark",
    speedtest_desc: "Benchmark real-time network bandwidth and latency between this client device and the Allod NAS server to verify suitability for <strong>Jellyfin 4K/1080p video streaming</strong> and <strong>Samba file transfers</strong>.",
    speedtest_ping: "⏱️ Latency (Ping)",
    speedtest_dl: "📥 Download (Server➔Client)",
    speedtest_ul: "📤 Upload (Client➔Server)",
    speedtest_verdict_title: "🎬 Streaming Suitability & Media Performance:",
    speedtest_verdict_wait: "Waiting for test execution... Click 'Start Speedtest' to begin.",
    speedtest_iperf_info: "💡 For advanced terminal benchmarking:",
    speedtest_btn_start: "⚡ Start Speedtest",
    speedtest_btn_running: "⏳ Benchmarking...",
    speedtest_btn_repeat: "⚡ Repeat Speedtest",
    speedtest_close: "Close"
  },
  it: {
    app_title: "Allod — Cloud Personale in Piena Proprietà",
    tab_overview: "Panoramica",
    tab_modules: "Moduli & Livelli",
    tab_ring: "Federazione (Ring)",
    tab_resilience: "Test Resilienza",
    tab_settings: "Impostazioni & Manutenzione",
    mesh_subtitle: "Overlay WireGuard Mesh",

    page_overview_title: "Panoramica del Nodo",
    page_overview_sub: "Stato del sistema, risorse e sicurezza",
    page_modules_title: "Gestione Moduli & Servizi",
    page_modules_sub: "Adatta le risorse, avvia e ferma i container con controllo preflight istantaneo.",
    page_ring_title: "Federazione del Ring a Tre Nodi",
    page_ring_sub: "Ogni dataset critico viene salvato in 2 copie remote indipendenti sui nodi degli amici.",
    page_resilience_title: "Test di Resilienza & Simulatore",
    page_resilience_sub: "Simula scenari critici reali per verificare il comportamento dell'orchestratore Allod.",
    page_settings_title: "Impostazioni Avanzate & Manutenzione",
    page_settings_sub: "Gestione ciclo di vita di Allod, aggiornamenti Git/Go e manutenzione globale Systemd/Podman.",

    btn_speedtest: "Test Velocità LAN",
    btn_sweeper: "Sweeper Podman",
    btn_systemd_reload: "Ricarica Systemd",
    btn_refresh: "Aggiorna",
    msg_systemd_reloading: "Ricarica systemd & Quadlet in corso...",
    msg_systemd_reloaded: "✓ Unità Quadlet rigenerate e systemctl daemon-reload eseguito con successo!",
    helper_connected: "Helper Root: Connesso",
    helper_offline: "Helper Root: Non Avviato / Offline",

    // Metrics
    metric_ram: "Memoria RAM",
    metric_ram_footer: "Riservato core: 600 MB",
    metric_modules: "Moduli Attivi",
    metric_ring: "Gruppo Federato",
    metric_security: "Sicurezza & Confine",
    storage_pool_title: "Dischi Fisici & Pool Storage NAS",
    federation_health_title: "Stato di Salute della Federazione",
    federation_active_badge: "Sorveglianza Attiva",
    federation_desc: "I nodi nel Ring si scambiano battiti cardiaci e sorvegliano i backup cifrati in modalità append-only.",

    modules_header_title: "Gestione Moduli & Risorse",
    modules_header_desc: "Ogni modulo adatta il consumo di risorse in base al livello selezionato con controllo preflight istantaneo.",
    ring_header_title: "Topologia del Ring a Tre Nodi",
    ring_header_desc: "Ogni dataset critico viene salvato in 2 copie remote indipendenti sui nodi degli amici.",
    ring_members_title: "Membri del Gruppo",
    ring_datasets_title: "Repliche dei Dataset",

    resilience_header_title: "Test di Resilienza & Simulatore",
    resilience_header_desc: "Simula scenari critici reali per verificare il comportamento dell'orchestratore Allod.",
    sim_disconnect_title: "Simulazione Disconnessione/Uscita Membro",
    sim_disconnect_desc: "Cosa succede se un amico del gruppo stacca il NAS o esce dalla federazione?",
    sim_disconnect_select: "Seleziona membro da disconnettere:",
    sim_btn_rebalance: "Simula Rimozione & Calcola Rebalance",
    sim_update_title: "Macchina a Stati Aggiornamenti & Rollback",
    sim_update_desc: "Testa il comportamento quando una nuova immagine container fallisce l'healthcheck di avvio.",
    sim_update_module: "Modulo da aggiornare:",
    sim_update_tag: "Nuovo Tag Immagine:",
    sim_btn_healthy: "Aggiornamento Sano",
    sim_btn_rollback: "Simula Errore & Rollback",

    // Settings & Maintenance
    settings_lifecycle_title: "🚀 Ciclo di Vita & Aggiornamento Allod",
    settings_lifecycle_desc: "Scarica il codice sorgente aggiornato da GitHub, ricompila i binari Go o esegui un aggiornamento completo con riavvio automatico.",
    btn_git_pull: "⬇️ Git Pull",
    btn_go_build: "🔨 Go Build",
    btn_self_update: "🚀 Aggiorna & Riavvia (One-Click)",
    settings_maintenance_title: "🛠️ Manutenzione Globale Systemd & Podman",
    settings_maintenance_desc: "Esegui azioni amministrative di sistema direttamente dalla GUI senza dover accedere via SSH.",
    btn_reset_failed: "🔄 Reset Stati di Errore",
    btn_start_active: "▶️ Avvia Moduli Configurati",
    btn_stop_all: "⏹ Ferma Tutti i Moduli",
    settings_start_active_note: "Avvia solo i moduli configurati con un livello attivo (i moduli con livello 'off' restano spenti).",
    settings_console_title: "💻 Console di Output Esecuzione Comandi",
    settings_console_clear: "Pulisci Console",

    // Actions & Badges
    status_protected: "🟢 PROTETTO (In Produzione)",
    status_running: "🟢 IN ESECUZIONE",
    status_unlocked: "🔓 IN ESECUZIONE (Sbloccato)",
    status_stopped: "⏹ FERMATO",
    status_error: "🔴 ERRORE",
    status_starting: "⏳ AVVIO IN CORSO...",

    tier_core: "🧱 Sistema",
    tier_recommended: "⭐ Consigliato",
    tier_optional: "🚀 Opzionale",

    btn_start: "▶ Avvia",
    btn_restart: "▶ Riavvia",
    btn_stop: "⏹ Ferma",
    btn_diagnostics: "🩺 Diagnostica",
    btn_unlock: "🔓 Sblocca",
    btn_lock: "🔒 Blocca",
    btn_reset: "🗑️ Reset",
    btn_open_web: "🌐 Apri interfaccia web",

    label_level: "Livello:",
    label_userns: "UserNS:",
    label_native: "Nativo",
    label_nas_storage: "💾 Storage NAS Fisico:",
    label_database: "Database",
    label_pending: "In attesa",
    pool_raid1: "🟢 Pool Btrfs RAID 1",
    pool_local: "⚠️ Storage Locale (Home)",

    protected_notice: "🔒 <strong>Dati Protetti in Produzione:</strong> I file e il database sono salvati sul pool NAS RAID 1. La card è protetta per evitare arresti o modifiche accidentali del database. Clicca <strong>Sblocca</strong> per apportare modifiche.",

    // Sweeper
    sweeper_title: "🧹 Sweeper & Igiene Podman",
    sweeper_desc: "Scansiona e rimuove in modo sicuro <strong>container morti/terminati</strong>, <strong>layer di immagini orfane</strong> e <strong>file di lock (.cid)</strong> lasciati da crash o riavvii, <u>senza mai toccare i container attivi né i dati su disco</u>.",
    sweeper_ready: "Pronto per la scansione. Clicca sul pulsante sottostante per avviare lo sweeper.",
    sweeper_btn_start: "🧹 Avvia Pulizia Fantasmi",
    sweeper_btn_running: "⏳ Scansione in corso...",
    sweeper_btn_rescan: "🧹 Esegui Nuova Scansione",
    sweeper_close: "Chiudi",

    // Speedtest
    speedtest_title: "⚡ Benchmark Velocità Rete LAN (Speedtest)",
    speedtest_desc: "Misura in tempo reale la larghezza di banda e la latenza tra questo dispositivo e il server NAS Allod per verificare l'idoneità allo <strong>streaming video 4K/1080p Jellyfin</strong> e ai <strong>trasferimenti Samba</strong>.",
    speedtest_ping: "⏱️ Latenza (Ping)",
    speedtest_dl: "📥 Download (Server➔PC)",
    speedtest_ul: "📤 Upload (PC➔Server)",
    speedtest_verdict_title: "🎬 Idoneità Streaming & Prestazioni Media:",
    speedtest_verdict_wait: "In attesa di esecuzione test... Clicca su 'Avvia Test Velocità' per iniziare.",
    speedtest_iperf_info: "💡 Per test avanzati da terminale:",
    speedtest_btn_start: "⚡ Avvia Test Velocità",
    speedtest_btn_running: "⏳ Test in corso...",
    speedtest_btn_repeat: "⚡ Ripeti Test Velocità",
    speedtest_close: "Chiudi"
  }
};

let currentLang = localStorage.getItem('allod_lang') || 'en';

function t(key, fallback) {
  const dict = i18n[currentLang] || i18n['en'];
  return dict[key] || fallback || key;
}

function setLanguage(lang) {
  if (!i18n[lang]) lang = 'en';
  currentLang = lang;
  localStorage.setItem('allod_lang', lang);

  // Update button active state
  const btnEn = document.getElementById('lang-btn-en');
  const btnIt = document.getElementById('lang-btn-it');
  if (btnEn && btnIt) {
    if (lang === 'en') {
      btnEn.style.background = 'var(--primary)';
      btnEn.style.color = '#0f172a';
      btnIt.style.background = 'transparent';
      btnIt.style.color = 'var(--text-muted)';
    } else {
      btnIt.style.background = 'var(--primary)';
      btnIt.style.color = '#0f172a';
      btnEn.style.background = 'transparent';
      btnEn.style.color = 'var(--text-muted)';
    }
  }

  // Update HTML lang attribute
  document.documentElement.lang = lang;
  document.title = t('app_title', 'Allod — Sovereign Personal Cloud & NAS');

  // Update all elements with data-i18n
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    const val = t(key);
    if (val) el.innerHTML = val;
  });

  // Re-render UI components with new language
  if (typeof renderOverview === 'function') renderOverview();
  if (typeof renderModules === 'function') renderModules();
  if (typeof renderRing === 'function') renderRing();
  if (typeof renderResilience === 'function') renderResilience();
}