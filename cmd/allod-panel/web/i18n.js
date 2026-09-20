// Allod Sovereign Panel - i18n Translation Dictionary (English First + Italian)

const i18n = {
  en: {
    app_title: "Allod — Sovereign Personal Cloud & NAS",
    tab_launchpad: "Launchpad",
    tab_overview: "Overview",
    tab_modules: "Modules & Levels",
    tab_ring: "Ring Federation",
    tab_resilience: "Resilience Tests",
    tab_settings: "Settings & Maintenance",
    mesh_subtitle: "Encrypted WireGuard Mesh",

    page_launchpad_title: "App Launchpad & Hub",
    page_launchpad_sub: "One-click access to all your personal cloud applications, streaming services, and shares",
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
    btn_copy: "Copy",
    btn_copied: "✓ Copied!",
    btn_copy_all: "Copy All",
    btn_close: "Close",
    btn_cancel: "Cancel",
    msg_systemd_reloading: "Reloading systemd & Quadlet...",
    msg_systemd_reloaded: "✓ Quadlet units regenerated and systemctl daemon-reload executed!",
    helper_connected: "Root Helper: Connected",
    helper_offline: "Root Helper: Offline / Not Started",
    helper_eacces_pill: "Root Helper: Permission Denied",
    helper_status_eacces_title: "⚠️ Root Helper Permission Denied (EACCES)",
    helper_status_eacces_desc: "The web panel user does not belong to the 'allod' group. Add the user to the group and restart the session:",
    helper_eacces_session_note: "Note: Group membership requires a session reload to take effect (with linger active: loginctl terminate-user $USER or reboot).",
    hint_username_rules: "Lowercase letters and numbers only (e.g. mario, anna, luca).",

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
    tier_core: "System",
    tier_recommended: "Recommended",
    tier_optional: "Optional",
    tier_section_core_title: "Core System Modules",
    tier_section_core_desc: "Essential infrastructure for storage pool, network shares, backup engine, and node watchdog.",
    tier_section_recommended_title: "Recommended Modules",
    tier_section_recommended_desc: "Encrypted mesh VPN remote access and sovereign mobile photo backup.",
    tier_section_optional_title: "Optional Modules",
    tier_section_optional_desc: "Advanced personal cloud and media streaming apps, enabled on demand based on server RAM.",
    tier_active_count: "{active} / {total} Active",
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
    btn_git_pull: "Git Pull",
    btn_go_build: "Go Build",
    btn_self_update: "One-Click Update & Restart",
    settings_maintenance_title: "🛠️ Global Systemd & Podman Maintenance",
    settings_maintenance_desc: "Execute administrative system actions directly from the GUI without SSH access.",
    btn_reset_failed: "Reset-Failed State",
    btn_start_active: "Start Configured Modules",
    btn_stop_all: "Stop All Modules",
    btn_enable_autostart: "Enable Boot Autostart",
    msg_autostart_enabled: "✓ Systemd linger enabled and allod-panel registered for automatic boot startup!",
    settings_start_active_note: "Starts only modules configured with an active level (modules with level 'off' remain off).",
    settings_console_title: "💻 Live Maintenance Console Output",
    settings_console_clear: "Clear Console",

    // First Setup & Onboarding
    first_setup_title: "Welcome to Allod — 24/7 Boot Persistence Setup",
    first_setup_sub: "Ensure all sovereign services and Podman containers remain active after server reboots",
    first_setup_btn_1click: "Complete Boot Setup (1-Click)",
    first_setup_desc: "Allod and Podman operate in rootless mode for maximum security. On Ubuntu, user containers (Immich, Nextcloud, Samba) and the dashboard require systemd linger and service registration to start automatically at power-on without requiring an active SSH session.",
    first_setup_check_linger: "Systemd User Linger (Podman 24/7)",
    first_setup_check_panel: "allod-panel.service (Web UI at Boot)",
    first_setup_cli_title: "💻 Or run manually via CLI on your server:",
    first_setup_completed: "✓ Boot persistence configured! Allod is 100% ready for 24/7 unattended operation.",

    // Helper Modal & Maintenance
    helper_modal_title: "Root Helper Daemon (allod-helperd)",
    helper_modal_desc: "allod-helperd is the privileged micro-daemon of Allod (executes as root with a closed action whitelist). It allows the dashboard (100% rootless) to safely partition disks, format Btrfs RAID 1 pools, take subvolume snapshots, and reload Samba shares.",
    helper_service_box_title: "⚙️ Register & Run as Permanent Systemd Service (Recommended):",
    helper_restart_box_title: "🔄 Quick Service Restart (if already registered):",
    helper_manual_box_title: "⚡ Quick Manual Run (Foreground / Test):",
    helper_maintenance_title: "Root Helper Daemon (allod-helperd)",
    helper_maintenance_desc: "Privileged micro-daemon (closed action whitelist) for Btrfs RAID 1 formatting, SMART diagnostics, and Samba share reload without root web access.",
    btn_recheck: "Recheck Connection",

    // Photos & Shares Integration
    photos_shares_toggle: "Integrate with Shares (Samba LAN)",
    photos_shares_desc: "Creates a transparent pointer in \\allod\\shares\\photos to browse and copy photos directly from your PC without moving files.",
    photos_shares_connected: "🟢 Linked to \\\\allod\\shares\\photos",
    photos_shares_unmounted: "⚪ Not linked to Samba",
    photos_shares_warning_shares: "⚠️ Start the Shares module to browse this folder from Windows/Mac",
    photos_shares_helper_offline: "⚠️ Start Root Helper to manage the Samba bind mount",

    // SMB Password Modal
    smb_modal_title: "Set Samba Password (Windows / Mac Access)",
    smb_modal_desc: "Set your network password to access shared folders (\\\\IP\\shares) from Windows File Explorer, Mac Finder, or mobile apps without SSH.",
    label_smb_username: "Samba Username:",
    label_smb_password: "New Samba Password:",
    smb_win_path_label: "💻 Windows Network Path:",
    btn_save_smb_pass: "Save Samba Password",
    btn_smb_password: "Set SMB Password",
    label_smb_credentials: "Windows / Mac Network Credentials",
    desc_smb_credentials: "Set or change the Samba password to connect PCs and Macs to your shares.",

    // Network & Remote Access
    network_module_title: "NetBird Sovereign Mesh",
    network_config_btn: "Configure NetBird",
    network_pairing_btn: "Connect Devices",
    network_mesh_ip_label: "Private Mesh IP:",
    network_modal_title: "NetBird Sovereign Mesh Setup",
    network_modal_desc: "Connect your Allod node to NetBird for high-speed WireGuard remote access with zero open router ports.",
    network_modal_mode_label: "Operational Mode:",
    network_mode_cloud: "🌐 NetBird Cloud (Europe - Frankfurt, 100% GDPR, Zero Ports)",
    network_mode_selfhosted: "🏠 NetBird Self-Hosted (Sovereign Management Server)",
    network_modal_key_label: "NetBird Setup Key:",
    network_modal_key_sub: "Generate a Setup Key in your NetBird management dashboard (Cloud or Self-Hosted) with peer registration permissions.",
    network_modal_mgmt_label: "Management Server URL:",
    network_modal_save: "Save & Restart NetBird",
    network_pairing_title: "Connect Devices to NetBird Mesh",
    network_pairing_desc: "Connect your smartphones, PCs, and tablets to your private WireGuard mesh using the free official NetBird app.",
    network_status_mode_label: "Mode:",
    network_status_mgmt_label: "Management URL:",

    // Family Hub & Triad Orchestration
    btn_family_hub: "Family Members & Triad",
    family_modal_title: "Family Members & Sovereign Triad",
    family_modal_sub: "Manage personal accounts, private SMB shares, Immich photo libraries, and autonomous onboarding",
    tab_family_members: "Family Members",
    tab_family_new: "New Member",
    tab_family_guide: "Modules Guide & Immich",
    family_list_desc: "Each member has a private protected SMB share (0770) and can manage their password autonomously.",
    label_first_name: "First Name *",
    label_last_name: "Last Name",
    label_username: "Username (SMB share & storage label) *",
    label_role: "Family Role",
    label_email: "Email (optional)",
    label_notes: "Devices / Notes",
    label_initial_password: "Initial password (min. 4 characters):",
    btn_triad_user: "Create Triad User",
    triad_modal_title: "Create Triad User (Samba + Immich + Jellyfin)",
    triad_modal_desc: "Automatically orchestrate the entire ecosystem for a new user: private SMB share (\\\\allod\\<user>), Immich Photos bind-mount, and Jellyfin media readiness.",
    triad_status_title: "Triad Services Status Check:",
    label_triad_username: "Triad Username:",
    label_triad_password: "User Password (Samba & Recommended for Immich/Jellyfin):",
    btn_create_triad: "Create User & Orchestrate Triad",
    triad_link_photos_label: "Link Immich photos library to private Samba share (photos/)",
    triad_link_photos_sub: "If disabled, photos will remain accessible only through the Immich app with 2FA, isolated on the server filesystem (strict 0770 permissions reserved for root and container).",
    triad_users_title: "👥 Triad Users Detected on Server",
    triad_guide_summary: "Guide: How to configure Storage Label & Template in Immich",
    network_pairing_steps_title: "How to connect from your smartphone or PC:",
    network_step1: "1. Install the official NetBird app from Google Play, Apple App Store, or netbird.io/install.",
    network_step2: "2. If using NetBird Self-Hosted: in app settings enter your Management Server URL.",
    network_step3: "3. Log in with your NetBird account or register your device with a Setup Key.",
    network_step4: "4. Enable connection: access Immich, Jellyfin 4K, Samba, and Allod Panel in direct encrypted P2P!",

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

    btn_start: "Start",
    btn_restart: "Restart",
    btn_stop: "Stop",
    btn_diagnostics: "Diagnostics",
    btn_unlock: "Unlock",
    btn_lock: "Lock",
    btn_reset: "Reset",
    btn_open_web: "Open Web Interface",

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
    speedtest_btn_start: "⚡ Start Speedtest",
    speedtest_btn_running: "⏳ Benchmarking...",
    speedtest_btn_repeat: "⚡ Repeat Speedtest",
    speedtest_close: "Close",

    // Themes
    settings_theme_title: "Appearance & Interface Theme",
    settings_theme_desc: "Customize the visual appearance of Allod. Fully rendered in your browser with zero server CPU or RAM overhead.",
    settings_theme_active_label: "Active Theme:",
    theme_default: "Default Slate",
    theme_default_sub: "Cyber Dark & Neon Cyan",
    theme_moderno: "Moderno",
    theme_moderno_sub: "Material Dashboard Creative Tim, rounded cards & pink glow",
    theme_cia: "CIA // Tactical",
    theme_cia_sub: "Jack Ryan Langley, radar stealth & NVG phosphor green",
    theme_allod: "Allod",
    theme_allod_sub: "Medieval freehold, aged oak, parchment & forged brass",
    btn_theme: "Theme"
  },
  it: {
    app_title: "Allod — Cloud Personale in Piena Proprietà",
    tab_launchpad: "Launchpad",
    tab_overview: "Panoramica",
    tab_modules: "Moduli & Livelli",
    tab_ring: "Federazione (Ring)",
    tab_resilience: "Test Resilienza",
    tab_settings: "Impostazioni & Manutenzione",
    mesh_subtitle: "Overlay WireGuard Mesh",

    page_launchpad_title: "Launchpad & Centro Applicazioni",
    page_launchpad_sub: "Accesso rapido con un click a tutte le tue app cloud, streaming e cartelle condivise",
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
    btn_copy: "Copia",
    btn_copied: "✓ Copiato!",
    btn_copy_all: "Copia Tutto",
    btn_close: "Chiudi",
    btn_cancel: "Annulla",
    msg_systemd_reloading: "Ricarica systemd & Quadlet in corso...",
    msg_systemd_reloaded: "✓ Unità Quadlet rigenerate e systemctl daemon-reload eseguito con successo!",
    helper_connected: "Helper Root: Connesso",
    helper_offline: "Helper Root: Non Avviato / Offline",
    helper_eacces_pill: "Helper Root: Permesso Negato",
    helper_status_eacces_title: "⚠️ Permesso Helper Negato (EACCES)",
    helper_status_eacces_desc: "L'utente del pannello web non appartiene al gruppo 'allod'. Aggiungi l'utente al gruppo ed esegui un nuovo login:",
    helper_eacces_session_note: "Nota: L'appartenenza al gruppo diventa effettiva solo dopo un nuovo login della sessione (con linger attivo: loginctl terminate-user $USER oppure riavvio).",
    hint_username_rules: "Solo lettere minuscole e numeri (es. mario, anna, luca).",

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
    tier_core: "Sistema",
    tier_recommended: "Consigliato",
    tier_optional: "Opzionale",
    tier_section_core_title: "Moduli di Sistema",
    tier_section_core_desc: "Infrastruttura essenziale per il pool storage Btrfs RAID 1, condivisioni SMB locali, motore di backup e watchdog.",
    tier_section_recommended_title: "Moduli Consigliati",
    tier_section_recommended_desc: "Connettività mesh WireGuard cifrata per accesso remoto ovunque e backup foto da smartphone.",
    tier_section_optional_title: "Moduli Opzionali",
    tier_section_optional_desc: "Applicazioni multimediali e cloud personale (Nextcloud, Jellyfin), attivabili in base alla RAM disponibile.",
    tier_active_count: "{active} / {total} Attivi",
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
    btn_git_pull: "Git Pull",
    btn_go_build: "Go Build",
    btn_self_update: "Aggiorna & Riavvia (One-Click)",
    settings_maintenance_title: "🛠️ Manutenzione Globale Systemd & Podman",
    settings_maintenance_desc: "Esegui azioni amministrative di sistema direttamente dalla GUI senza dover accedere via SSH.",
    btn_reset_failed: "Reset Stati di Errore",
    btn_start_active: "Avvia Moduli Configurati",
    btn_stop_all: "Ferma Tutti i Moduli",
    btn_enable_autostart: "Abilita Avvio Automatico al Boot",
    msg_autostart_enabled: "✓ Linger abilitato e allod-panel registrato per l'avvio automatico al boot!",
    settings_start_active_note: "Avvia solo i moduli configurati con un livello attivo (i moduli con livello 'off' restano spenti).",
    settings_console_title: "💻 Console di Output Esecuzione Comandi",
    settings_console_clear: "Pulisci Console",

    // First Setup & Onboarding
    first_setup_title: "Benvenuto su Allod — Configurazione Iniziale Boot 24/7",
    first_setup_sub: "Garantisci la persistenza di tutti i servizi e dei container Podman anche dopo il riavvio del server",
    first_setup_btn_1click: "Completa Configurazione Boot (1-Click)",
    first_setup_desc: "Allod e Podman operano in modalità rootless per la massima sicurezza. Su Ubuntu, i container utente (Immich, Nextcloud, Samba) e la dashboard necessitano dell'abilitazione del linger e del servizio systemd per ripartire in autonomia all'accensione della macchina senza bisogno di login SSH.",
    first_setup_check_linger: "Systemd User Linger (Container 24/7)",
    first_setup_check_panel: "Servizio allod-panel (Web UI al Boot)",
    first_setup_cli_title: "💻 Oppure esegui manualmente via CLI sul server:",
    first_setup_completed: "✓ Persistenza al boot configurata! Allod è pronto al 100% per operare 24/7 senza interventi.",

    // Helper Modal & Maintenance
    helper_modal_title: "Demone Root Helper (allod-helperd)",
    helper_modal_desc: "allod-helperd è il micro-demone privilegiato di Allod (eseguito come root con whitelist chiusa di azioni). Permette al pannello (100% rootless) di gestire in sicurezza i dischi fisici, formattare il pool Btrfs RAID 1, creare snapshot e ricaricare Samba.",
    helper_service_box_title: "⚙️ Avvia o Registra come Servizio Systemd Permanente (Consigliato):",
    helper_restart_box_title: "🔄 Riavvio Rapido Servizio (se già installato):",
    helper_manual_box_title: "⚡ Esecuzione Rapida Manuale (Foreground / Test):",
    helper_maintenance_title: "Demone Root Helper (allod-helperd)",
    helper_maintenance_desc: "Micro-demone con privilegi root (whitelist chiusa di azioni) per la formattazione Btrfs RAID 1, controlli SMART e configurazione Samba senza dare permessi root al web server.",
    btn_recheck: "Ricontrolla Connessione",

    // Photos & Shares Integration
    photos_shares_toggle: "Esponi libreria in Shares (Samba LAN)",
    photos_shares_desc: "Crea un puntamento trasparente in \\allod\\shares\\photos per sfogliare e copiare le foto dal PC senza spostare i file.",
    photos_shares_connected: "🟢 Collegato a \\\\allod\\shares\\photos",
    photos_shares_unmounted: "⚪ Non collegato a Samba",
    photos_shares_warning_shares: "⚠️ Avvia il modulo Shares per visualizzare la cartella da Windows/Mac",
    photos_shares_helper_offline: "⚠️ Avvia l'Helper Root per gestire il puntamento Samba",

    // SMB Password Modal
    smb_modal_title: "Imposta Password Samba (Accesso Windows / Mac)",
    smb_modal_desc: "Imposta la password di rete per accedere alle cartelle condivise (\\\\IP\\shares) da Esplora Risorse di Windows, Mac Finder o app mobili senza usare la console SSH.",
    label_smb_username: "Nome Utente Samba:",
    label_smb_password: "Nuova Password Samba:",
    smb_win_path_label: "💻 Percorso di Rete Windows:",
    btn_save_smb_pass: "Salva Password Samba",
    btn_smb_password: "Imposta Password SMB",
    label_smb_credentials: "Credenziali di Rete Windows / Mac",
    desc_smb_credentials: "Imposta o modifica la password Samba per connettere PC e Mac alle cartelle condivise.",

    // Network & Remote Access
    network_module_title: "Rete Mesh Sovrana NetBird",
    network_config_btn: "Configura NetBird",
    network_pairing_btn: "Connetti Dispositivi",
    network_mesh_ip_label: "IP Mesh Privato:",
    network_modal_title: "Configurazione Rete Sovrana NetBird",
    network_modal_desc: "Collega il tuo nodo Allod a NetBird per accesso remoto sicuro WireGuard punto-punto senza aprire porte sul router.",
    network_modal_mode_label: "Modalità Operativa:",
    network_mode_cloud: "🌐 NetBird Cloud (Europa - Francoforte, 100% GDPR, Zero Porte)",
    network_mode_selfhosted: "🏠 NetBird Self-Hosted (Server di Coordinamento Sovrano)",
    network_modal_key_label: "NetBird Setup Key:",
    network_modal_key_sub: "Genera una Setup Key dalla dashboard NetBird (Cloud o Self-Hosted) con permessi di registrazione peer.",
    network_modal_mgmt_label: "URL Server NetBird:",
    network_modal_save: "Salva e Riavvia NetBird",
    network_pairing_title: "Connetti Dispositivi alla Rete NetBird",
    network_pairing_desc: "Connetti smartphone, computer e tablet alla tua rete mesh WireGuard privata con l'app ufficiale gratuita NetBird.",
    network_status_mode_label: "Modalità:",
    network_status_mgmt_label: "Management URL:",

    // Family Hub & Triad Orchestration
    btn_family_hub: "Membri Famiglia & Triade",
    family_modal_title: "Membri della Famiglia & Triade Sovereign",
    family_modal_sub: "Gestisci account personali, cartelle private SMB, librerie Immich e onboarding trasparente",
    tab_family_members: "Membri Famiglia",
    tab_family_new: "Nuovo Membro",
    tab_family_guide: "Guida Moduli & Immich",
    family_list_desc: "Ciascun membro ha uno share SMB privato protetto (0770) e può gestire la propria password in piena autonomia.",
    label_first_name: "Nome *",
    label_last_name: "Cognome",
    label_username: "Username (cartelle SMB & storage label) *",
    label_role: "Ruolo nella Famiglia",
    label_email: "Email (opzionale)",
    label_notes: "Dispositivi / Note",
    label_initial_password: "Password iniziale (min. 4 caratteri):",
    btn_triad_user: "Crea Utente Triade",
    triad_modal_title: "Crea Utente Triade (Samba + Immich + Jellyfin)",
    triad_modal_desc: "Predispone automaticamente l'intero ecosistema per un nuovo utente: cartella privata SMB (\\\\allod\\<utente>), bind-mount per Immich Photos e predisposizione streaming Jellyfin.",
    triad_status_title: "Verifica Stato Servizi della Triade:",
    label_triad_username: "Nome Utente Triade:",
    label_triad_password: "Password Utente (Samba & Consigliata per Immich/Jellyfin):",
    btn_create_triad: "Crea Utente & Predisponi Triade",
    triad_link_photos_label: "Collega libreria foto Immich allo share privato Samba (photos/)",
    triad_link_photos_sub: "Se disattivato, le foto rimarranno protette e visibili unicamente dall'app Immich con autenticazione/2FA, isolate sul filesystem del server (permessi 0770 con accesso riservato a root e al container).",
    triad_users_title: "👥 Utenti Triade Rilevati sul Server",
    triad_guide_summary: "Guida: Come impostare lo Storage Label & Template su Immich",
    network_pairing_steps_title: "Come collegarsi dal tuo smartphone o PC:",
    network_step1: "1. Scarica l'app ufficiale NetBird da Google Play, Apple App Store o netbird.io/install.",
    network_step2: "2. In caso di NetBird Self-Hosted: nelle impostazioni dell'app inserisci il Management Server URL.",
    network_step3: "3. Accedi al tuo account NetBird o registra il dispositivo con una Setup Key.",
    network_step4: "4. Attiva la connessione: il dispositivo accede a Immich, Jellyfin 4K, Samba e Allod Panel in P2P diretto!",

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

    btn_start: "Avvia",
    btn_restart: "Riavvia",
    btn_stop: "Ferma",
    btn_diagnostics: "Diagnostica",
    btn_unlock: "Sblocca",
    btn_lock: "Blocca",
    btn_reset: "Ripristina",
    btn_open_web: "Apri interfaccia web",

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
    speedtest_close: "Chiudi",

    // Themes
    settings_theme_title: "Aspetto & Tema dell'Interfaccia",
    settings_theme_desc: "Personalizza l'esperienza visiva di Allod. Il rendering viene gestito interamente nel browser a zero consumo di CPU e RAM sul server.",
    settings_theme_active_label: "Tema Attivo:",
    theme_default: "Default Slate",
    theme_default_sub: "Cyber dark ardesia e accenti ciano neon",
    theme_moderno: "Moderno",
    theme_moderno_sub: "Material Dashboard Creative Tim, card arrotondate e gradiente rosa fucsia",
    theme_cia: "CIA // Tactical",
    theme_cia_sub: "Jack Ryan Langley, radar stealth e fosforo verde operativo",
    theme_allod: "Allod",
    theme_allod_sub: "Terre libere medievali, rovere antico, pergamena e ottone battuto",
    btn_theme: "Tema"
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
      btnEn.style.color = 'var(--primary-text, #0f172a)';
      btnIt.style.background = 'transparent';
      btnIt.style.color = 'var(--text-muted)';
    } else {
      btnIt.style.background = 'var(--primary)';
      btnIt.style.color = 'var(--primary-text, #0f172a)';
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
  if (typeof updateThemeBadge === 'function') updateThemeBadge();
}