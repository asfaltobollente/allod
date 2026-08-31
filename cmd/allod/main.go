package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allod-project/allod/internal/cloudinit"
	"github.com/allod-project/allod/internal/config"
	"github.com/allod-project/allod/internal/helper"
	"github.com/allod-project/allod/internal/manifest"
	"github.com/allod-project/allod/internal/preflight"
	"github.com/allod-project/allod/internal/quadlet"
	"github.com/allod-project/allod/internal/ring"
	"github.com/allod-project/allod/internal/sbom"
	"github.com/allod-project/allod/internal/state"
)

var (
	cfgFile        string
	stateDB        string
	ringFile       string
	removeMember   string
	acceptRisk     bool
	useSystemd       bool
	outDirOverride   string
	storageInitMode  string
	storageInitMount string
	storageInitForce bool
)

var rootCmd = &cobra.Command{
	Use:   "allod",
	Short: "Allod - Orchestratore per cloud personale",
	Long:  `Allod gestisce un cloud personale basato su Podman tramite configurazioni dichiarative.`,
}

func computeCombinedHash(files map[string]string) string {
	if len(files) == 0 {
		return ""
	}
	var keys []string
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(":"))
		h.Write([]byte(files[k]))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func fileNamesList(files map[string]string) string {
	var keys []string
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func getSystemdDir() string {
	if os.Getenv("USER") == "root" {
		return "/etc/containers/systemd"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/etc/containers/systemd"
	}
	return filepath.Join(home, ".config", "containers", "systemd")
}

// planCmd displays differential plan comparing desired config with state.db
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Mostra il piano di configurazione differenziale rispetto a state.db",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore caricamento config: %v\n", err)
			os.Exit(1)
		}

		st, err := state.Open(stateDB)
		if err != nil {
			fmt.Printf("Errore apertura state.db: %v\n", err)
			os.Exit(1)
		}
		defer st.Close()

		appliedMods, err := st.ListModules()
		if err != nil {
			fmt.Printf("Errore lettura moduli da state.db: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Piano per il nodo %s:\n\n", cfg.Node.Name)

		processed := make(map[string]bool)

		for modName, modCfg := range cfg.Modules {
			processed[modName] = true
			manifestPath := filepath.Join("modules", modName, "module.yaml")
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				fmt.Printf("  ? %s: manifest mancante, ignorato\n", modName)
				continue
			}

			m, err := manifest.LoadManifest(manifestPath)
			if err != nil {
				fmt.Printf("  ? %s: errore manifest (%v), ignorato\n", modName, err)
				continue
			}

			applied, isApplied := appliedMods[modName]

			if modCfg.Level == "off" {
				if isApplied {
					fmt.Printf("  [-] %s: OFF (attualmente applicato al livello %s -> verrà rimosso)\n", modName, applied.Level)
				} else {
					fmt.Printf("  [=] %s: OFF (invariato)\n", modName)
				}
				continue
			}

			genRes, err := quadlet.Generate(modName, m, modCfg.Level)
			if err != nil {
				fmt.Printf("  ? %s: errore generazione Quadlet: %v\n", modName, err)
				continue
			}
			curHash := computeCombinedHash(genRes.Files)
			unitList := fileNamesList(genRes.Files)

			if !isApplied {
				fmt.Printf("  [+] %s: NUOVO (Livello %s) -> genererà [%s]\n", modName, modCfg.Level, unitList)
			} else if applied.Level != modCfg.Level {
				fmt.Printf("  [~] %s: MODIFICATO (Livello %s -> %s) -> aggiornerà [%s]\n", modName, applied.Level, modCfg.Level, unitList)
			} else if applied.ContentHash != curHash {
				fmt.Printf("  [~] %s: MODIFICATO (Configurazione interna variata) -> aggiornerà [%s]\n", modName, unitList)
			} else {
				fmt.Printf("  [=] %s: INVARIATO (Livello %s) [%s]\n", modName, modCfg.Level, unitList)
			}
		}

		// Check for modules in state.db that are no longer in config.yaml
		for appliedName, appliedMod := range appliedMods {
			if !processed[appliedName] {
				fmt.Printf("  [-] %s: NON IN CONFIGURAZIONE (attualmente al livello %s -> verrà rimosso)\n", appliedName, appliedMod.Level)
			}
		}
	},
}

// applyCmd applies desired config idempotently tracking changes in state.db
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Applica la configurazione e genera le unità Quadlet in modo idempotente",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore caricamento config: %v\n", err)
			os.Exit(1)
		}

		st, err := state.Open(stateDB)
		if err != nil {
			fmt.Printf("Errore apertura state.db: %v\n", err)
			os.Exit(1)
		}
		defer st.Close()

		appliedMods, err := st.ListModules()
		if err != nil {
			fmt.Printf("Errore lettura moduli da state.db: %v\n", err)
			os.Exit(1)
		}

		outDir := "out_quadlet"
		if outDirOverride != "" {
			outDir = outDirOverride
		} else if useSystemd {
			outDir = getSystemdDir()
		}

		if err := os.MkdirAll(outDir, 0755); err != nil {
			fmt.Printf("Errore creazione directory output (%s): %v\n", outDir, err)
			os.Exit(1)
		}

		fmt.Printf("Applicazione configurazione per il nodo %s [Destinazione: %s]...\n", cfg.Node.Name, outDir)
		applied := 0
		unchanged := 0
		removed := 0
		skipped := 0

		processed := make(map[string]bool)

		for modName, modCfg := range cfg.Modules {
			processed[modName] = true
			manifestPath := filepath.Join("modules", modName, "module.yaml")
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				fmt.Printf("  - %s: manifest non trovato, saltato\n", modName)
				skipped++
				continue
			}

			m, err := manifest.LoadManifest(manifestPath)
			if err != nil {
				fmt.Printf("  - %s: errore manifest: %v\n", modName, err)
				skipped++
				continue
			}

			appliedMod, isApplied := appliedMods[modName]

			if modCfg.Level == "off" {
				if isApplied {
					cleanModuleUnits(outDir, modName)
					st.DeleteModule(modName)
					fmt.Printf("  - %s: OFF → rimosse unità\n", modName)
					removed++
				}
				continue
			}

			genRes, err := quadlet.Generate(modName, m, modCfg.Level)
			if err != nil {
				fmt.Printf("  - %s: errore generazione: %v\n", modName, err)
				skipped++
				continue
			}

			curHash := computeCombinedHash(genRes.Files)

			// Check if already applied and all files exist on disk
			allFilesExist := true
			for fname := range genRes.Files {
				if !fileExists(filepath.Join(outDir, fname)) {
					allFilesExist = false
					break
				}
			}

			if isApplied && appliedMod.Level == modCfg.Level && appliedMod.ContentHash == curHash && allFilesExist {
				unitList := fileNamesList(genRes.Files)
				fmt.Printf("  = %s (livello %s) [INVARIATO] [%s]\n", modName, modCfg.Level, unitList)
				unchanged++
				continue
			}

			// Write all generated unit files
			var writtenFiles []string
			writeErr := false
			for fname, content := range genRes.Files {
				target := filepath.Join(outDir, fname)
				if err := os.WriteFile(target, []byte(content), 0644); err != nil {
					fmt.Printf("  - %s: errore scrittura file %s: %v\n", modName, target, err)
					writeErr = true
					break
				}
				writtenFiles = append(writtenFiles, fname)
			}

			if writeErr {
				skipped++
				continue
			}

			if err := st.SaveModule(modName, modCfg.Level, curHash); err != nil {
				fmt.Printf("  - %s: errore aggiornamento state.db: %v\n", modName, err)
				skipped++
				continue
			}

			unitList := strings.Join(writtenFiles, ", ")
			if !isApplied {
				fmt.Printf("  ✓ %s (livello %s) [CREATO] → [%s]\n", modName, modCfg.Level, unitList)
			} else {
				fmt.Printf("  ✓ %s (livello %s) [AGGIORNATO] → [%s]\n", modName, modCfg.Level, unitList)
			}
			applied++
		}

		// Handle orphaned modules in state.db
		for appliedName := range appliedMods {
			if !processed[appliedName] {
				cleanModuleUnits(outDir, appliedName)
				st.DeleteModule(appliedName)
				fmt.Printf("  - %s: NON IN CONFIG → rimosse unità\n", appliedName)
				removed++
			}
		}

		st.SetMeta("last_apply_node", cfg.Node.Name)

		fmt.Printf("\nApplicazione completata: %d create/aggiornate, %d invariate, %d rimosse", applied, unchanged, removed)
		if skipped > 0 {
			fmt.Printf(", %d saltate", skipped)
		}
		fmt.Println(".")

		if useSystemd || outDir == getSystemdDir() {
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
			fmt.Println("✓ Eseguito systemctl --user daemon-reload (unità registrate in systemd)")
			fmt.Println("  Per avviare tutti i servizi:  allod start")
			fmt.Println("  Per controllare lo stato:    allod status")
		}
	},
}

// startCmd starts module services using systemctl
var startCmd = &cobra.Command{
	Use:   "start [modulo|all]",
	Short: "Avvia i container o servizi dei moduli tramite systemd/Podman",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "all"
		if len(args) > 0 {
			target = args[0]
		}

		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore config: %v\n", err)
			os.Exit(1)
		}

		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

		if target == "all" {
			fmt.Printf("Avvio di tutti i moduli attivi per il nodo %s...\n", cfg.Node.Name)
			for modName, modCfg := range cfg.Modules {
				if modCfg.Level == "off" {
					continue
				}
				_ = exec.Command("systemctl", "--user", "start", "--no-block", modName+"-postgres").Run()
				_ = exec.Command("systemctl", "--user", "start", "--no-block", modName+"-valkey").Run()
				runCmd := exec.Command("systemctl", "--user", "start", "--no-block", modName)
				if err := runCmd.Run(); err != nil {
					fmt.Printf("  ✗ %-12s Errore avvio (systemd unit %s): %v\n", modName, modName, err)
				} else {
					fmt.Printf("  ✓ %-12s Avviato\n", modName)
				}
			}
		} else {
			_ = exec.Command("systemctl", "--user", "start", "--no-block", target+"-postgres").Run()
			_ = exec.Command("systemctl", "--user", "start", "--no-block", target+"-valkey").Run()
			runCmd := exec.Command("systemctl", "--user", "start", "--no-block", target)
			if err := runCmd.Run(); err != nil {
				fmt.Printf("✗ Errore avvio modulo '%s': %v\n", target, err)
				os.Exit(1)
			}
			fmt.Printf("✓ Modulo '%s' avviato con successo\n", target)
		}
	},
}

// stopCmd stops module services
var stopCmd = &cobra.Command{
	Use:   "stop [modulo|all]",
	Short: "Ferma i container o servizi dei moduli",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "all"
		if len(args) > 0 {
			target = args[0]
		}

		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore config: %v\n", err)
			os.Exit(1)
		}

		if target == "all" {
			fmt.Println("Arresto di tutti i moduli...")
			for modName := range cfg.Modules {
				_ = exec.Command("systemctl", "--user", "stop", modName, modName+"-postgres", modName+"-valkey").Run()
				fmt.Printf("  ⏹ %-12s Fermato\n", modName)
			}
		} else {
			_ = exec.Command("systemctl", "--user", "stop", target, target+"-postgres", target+"-valkey").Run()
			fmt.Printf("⏹ Modulo '%s' fermato\n", target)
		}
	},
}

// statusCmd shows live runtime status of modules
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra lo stato di esecuzione in tempo reale dei container e servizi",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("=== Stato Runtime del Nodo: %s ===\n\n", cfg.Node.Name)
		fmt.Printf("%-15s %-12s %-25s %s\n", "MODULO", "LIVELLO", "STATO RUNTIME", "SERVIZIO SYSTEMD")
		fmt.Println(strings.Repeat("-", 65))

		for modName, modCfg := range cfg.Modules {
			unit := modName + ".service"
			out, _ := exec.Command("systemctl", "--user", "is-active", modName).Output()
			status := strings.TrimSpace(string(out))
			if status == "" {
				status = "not-found / stopped"
			}

			statusIcon := "⏹ inactive"
			if status == "active" {
				statusIcon = "🟢 active (running)"
			} else if status == "failed" {
				statusIcon = "🔴 failed"
			} else if modCfg.Level == "off" {
				statusIcon = "⚪ off"
			}

			fmt.Printf("%-15s %-12s %-25s %s\n", modName, modCfg.Level, statusIcon, unit)
		}
	},
}

func cleanModuleUnits(outDir, modName string) {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, modName+".") || strings.HasPrefix(name, modName+"-") {
			os.Remove(filepath.Join(outDir, name))
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// setCmd per M2
var setCmd = &cobra.Command{
	Use:   "set [modulo=livello]",
	Short: "Modifica il livello di un modulo",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		parts := strings.Split(args[0], "=")
		if len(parts) != 2 {
			fmt.Println("Formato errato. Usa: allod set <modulo>=<livello>")
			os.Exit(1)
		}
		modName, reqLevel := parts[0], parts[1]

		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore config: %v\n", err)
			os.Exit(1)
		}

		manifestPath := filepath.Join("modules", modName, "module.yaml")
		m, err := manifest.LoadManifest(manifestPath)
		if err != nil {
			fmt.Printf("Modulo '%s' non trovato.\n", modName)
			os.Exit(1)
		}

		// Validate level existence BEFORE preflight (--accept-risk cannot bypass this)
		if reqLevel != "off" {
			if _, exists := m.Levels[reqLevel]; !exists {
				available := make([]string, 0, len(m.Levels))
				for k := range m.Levels {
					available = append(available, k)
				}
				sort.Strings(available)
				fmt.Printf("✗ Livello '%s' inesistente per il modulo '%s'.\n", reqLevel, modName)
				fmt.Printf("  Livelli disponibili: %s\n", strings.Join(available, ", "))
				os.Exit(1)
			}
		}

		res := preflight.Check(cfg, modName, m, reqLevel)
		if !res.Pass {
			if acceptRisk {
				fmt.Printf("⚠️  Rischio accettato. Il sistema potrebbe diventare instabile.\n")
			} else {
				fmt.Printf("✗ Rifiutato: risorse insufficienti o dipendenze non soddisfatte\n\n")

				totalSysRAM := preflight.GetSystemRAMMB()
				fmt.Printf("  disponibile ............ %.1f GB     (%d MB fisici − riservato core)\n", float64(res.AvailMB)/1024.0, totalSysRAM)
				fmt.Printf("  impegnato .............. %.1f GB     %s\n", float64(res.Committed)/1024.0, preflight.FormatActiveMods(res.ActiveMods))
				fmt.Printf("  richiesto da %s ...... %.1f GB     %s\n\n", reqLevel, float64(res.TotalReqMB)/1024.0, res.Message)

				fmt.Println("  Il livello richiesto non può essere avviato in sicurezza.")
				fmt.Printf("  Per procedere comunque:  allod set %s=%s --accept-risk\n", modName, reqLevel)
				fmt.Printf("  Approfondimento:         allod help %s#requirements\n", modName)
				os.Exit(1)
			}
		}

		if cfg.Modules == nil {
			cfg.Modules = make(map[string]config.ModuleConfig)
		}

		modCfg := cfg.Modules[modName]
		modCfg.Level = reqLevel
		cfg.Modules[modName] = modCfg

		if err := cfg.Save(cfgFile); err != nil {
			fmt.Printf("Errore salvataggio config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Modulo %s impostato al livello %s\n", modName, reqLevel)
	},
}

// doctorCmd per M2
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnostica dello stato del nodo",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Printf("Errore caricamento config: %v\n", err)
			os.Exit(1)
		}

		issues := 0
		fmt.Println("Diagnostica di base:")

		for modName, modCfg := range cfg.Modules {
			if modCfg.Level == "off" {
				continue
			}
			manifestPath := filepath.Join("modules", modName, "module.yaml")
			m, err := manifest.LoadManifest(manifestPath)
			if err != nil {
				fmt.Printf("  [%s-01] Errore manifest: %v\n", modName, err)
				issues++
				continue
			}

			res := preflight.Check(cfg, modName, m, modCfg.Level)
			if !res.Pass {
				fmt.Printf("  [%s-02] ATTENZIONE: Il modulo sta girando fuori specifica (Out of spec).\n", modName)
				fmt.Printf("               Causa: %s\n", res.Message)
				fmt.Printf("               -> allod help %s#requirements\n", modName)
				issues++
			}
		}

		if issues == 0 {
			fmt.Println("\n✓ Tutto in regola")
		} else {
			fmt.Printf("\n✗ Trovati %d avvisi\n", issues)
		}
	},
}

// installCmd per M3
var installCmd = &cobra.Command{
	Use:   "install [hostname]",
	Short: "Genera il file cloud-init per l'installazione zero-touch su Ubuntu Server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hostname := args[0]
		out, err := cloudinit.Generate(cloudinit.Config{Hostname: hostname})
		if err != nil {
			fmt.Printf("Errore generazione cloud-init: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
	},
}

// initCmd initializes a new node configuration file (config.yaml)
var initCmd = &cobra.Command{
	Use:   "init [nome-nodo]",
	Short: "Inizializza una nuova configurazione locale (config.yaml)",
	Run: func(cmd *cobra.Command, args []string) {
		targetFile := "config.yaml"
		if cfgFile != "" && cfgFile != "configs/config.example.yaml" {
			targetFile = cfgFile
		}

		nodeName := "allod-node-01"
		if len(args) > 0 && args[0] != "" {
			nodeName = args[0]
		} else {
			if h, err := os.Hostname(); err == nil && h != "" {
				nodeName = "allod-" + h
			}
		}

		if _, err := os.Stat(targetFile); err == nil {
			fmt.Printf("⚠️  Il file '%s' esiste già.\nPer rigenerarlo, rimuovilo prima o modifica il nome in %s\n", targetFile, targetFile)
			return
		}

		cfg, err := config.LoadConfig("configs/config.example.yaml")
		if err != nil {
			cfg = &config.Config{
				Node: config.NodeConfig{
					Name:    nodeName,
					Channel: "stable",
				},
				Modules: map[string]config.ModuleConfig{
					"storage": {Level: "basic"},
					"shares":  {Level: "basic"},
					"photos":  {Level: "standard"},
					"backup":  {Level: "peers"},
					"watch":   {Level: "federated"},
				},
			}
		} else {
			cfg.Node.Name = nodeName
		}

		if err := cfg.Save(targetFile); err != nil {
			fmt.Printf("Errore creazione '%s': %v\n", targetFile, err)
			os.Exit(1)
		}

		fmt.Printf("✓ Inizializzata configurazione per il nodo '%s' in '%s'\n\n", nodeName, targetFile)
		fmt.Println("Prossimi passi:")
		fmt.Printf("  1. Ispeziona il piano:    ./allod plan -c %s\n", targetFile)
		fmt.Printf("  2. Applica le unità:     ./allod apply -c %s --systemd\n", targetFile)
		fmt.Printf("  3. Avvia la dashboard:   ./allod-panel\n")
	},
}

// ringCmd per M6
var ringCmd = &cobra.Command{
	Use:   "ring",
	Short: "Gestione della topologia federata del gruppo (Ring)",
}

var ringStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra lo stato del gruppo (Ring) e il piazzamento delle repliche",
	Run: func(cmd *cobra.Command, args []string) {
		topo, err := ring.LoadTopology(ringFile)
		if err != nil {
			fmt.Printf("Errore caricamento topologia ring (%s): %v\n", ringFile, err)
			os.Exit(1)
		}

		fmt.Printf("=== Gruppo Allod: %s ===\n", topo.Name)
		fmt.Printf("Target Repliche Remote: %d | Nodi Membri: %d\n\n", topo.TargetReplicas, len(topo.Members))

		fmt.Println("--- Membri del Ring ---")
		var memberIDs []string
		for id := range topo.Members {
			memberIDs = append(memberIDs, id)
		}
		sort.Strings(memberIDs)

		totalQuota := 0
		for _, id := range memberIDs {
			m := topo.Members[id]
			totalQuota += m.QuotaGB
			fmt.Printf("  • %-15s [IP: %-15s] Quota: %4d GB | Datasets locali: %d\n", m.ID, m.Address, m.QuotaGB, len(m.Datasets))
		}
		fmt.Printf("  Capacità totale del gruppo: %d GB\n\n", totalQuota)

		fmt.Println("--- Assegnazione Repliche Federate ---")
		placements := topo.CalculatePlacement()

		var pKeys []string
		for k := range placements {
			pKeys = append(pKeys, k)
		}
		sort.Strings(pKeys)

		allHealthy := true
		for _, k := range pKeys {
			p := placements[k]
			critTag := ""
			if p.Critical {
				critTag = " [CRITICO]"
			}
			targets := strings.Join(p.TargetNodes, ", ")
			if len(p.TargetNodes) == 0 {
				targets = "NESSUNA"
				allHealthy = false
			}
			fmt.Printf("  • %-25s (%2d GB)%s -> Repliche su: %-30s | Stato: %s\n",
				fmt.Sprintf("%s:%s", p.OwnerNode, p.DatasetID), p.SizeGB, critTag, fmt.Sprintf("[%s]", targets), p.Status)
		}

		fmt.Println()
		if allHealthy {
			fmt.Println("✓ Stato Gruppo: OTTIMALE (Ogni dataset critico possiede 2 repliche remote distinte)")
		} else {
			fmt.Println("⚠️ Stato Gruppo: DEGRADATO (Alcuni dataset non raggiungono il target di repliche)")
		}
	},
}

var ringSimulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simula eventi di topologia nel gruppo (es. rimozione di un membro)",
	Run: func(cmd *cobra.Command, args []string) {
		if removeMember == "" {
			fmt.Println("Specifica un'azione di simulazione (es. --remove <membro>)")
			os.Exit(1)
		}

		topo, err := ring.LoadTopology(ringFile)
		if err != nil {
			fmt.Printf("Errore caricamento topologia ring (%s): %v\n", ringFile, err)
			os.Exit(1)
		}

		impact, err := topo.SimulateRemoval(removeMember)
		if err != nil {
			fmt.Printf("Errore simulazione: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("=== Simulazione Uscita/Guasto Membro: %s ===\n\n", removeMember)
		fmt.Printf("Stato Quorum: %s\n\n", impact.QuorumHealth)

		fmt.Println("1. Dataset Primari del nodo rimosso:")
		if len(impact.LostPrimaryDatasets) == 0 {
			fmt.Println("   Nessun dataset primario perso.")
		} else {
			for _, ds := range impact.LostPrimaryDatasets {
				fmt.Printf("   • %s (da recuperare dalle repliche sui superstiti)\n", ds)
			}
		}

		fmt.Println("\n2. Dataset del gruppo che perdono una replica:")
		if len(impact.DegradedDatasets) == 0 {
			fmt.Println("   Nessun dataset degradato.")
		} else {
			for _, ds := range impact.DegradedDatasets {
				fmt.Printf("   ⚠️ %s\n", ds)
			}
		}

		fmt.Println("\n3. Piano di Riallocazione Automatica:")
		for _, act := range impact.RebalanceActions {
			fmt.Printf("   -> %s\n", act)
		}

		fmt.Printf("\n4. Capacità Residua Superstiti: %d GB usati / %d GB totali disponibili\n",
			impact.TotalUsedRemaining, impact.TotalQuotaRemaining)
	},
}

// ringAddCmd adds a peer to the federation ring
var ringAddCmd = &cobra.Command{
	Use:   "add [node-id] [mesh-ip] [quota-gb]",
	Short: "Aggiunge un nuovo nodo peer alla federazione Ring",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nodeID := args[0]
		meshIP := args[1]
		quotaGB := 500
		if len(args) >= 3 {
			if q, err := strconv.Atoi(args[2]); err == nil && q > 0 {
				quotaGB = q
			}
		}

		targetRing := "ring.yaml"
		if ringFile != "" && ringFile != "configs/ring.example.yaml" {
			targetRing = ringFile
		}

		topo, err := ring.LoadTopology(targetRing)
		if err != nil {
			topo = ring.NewRingTopology("allod-ring", 2)
		}

		topo.AddMember(&ring.Member{
			ID:      nodeID,
			Address: meshIP,
			QuotaGB: quotaGB,
			Datasets: []ring.Dataset{
				{ID: "photos", SizeGB: 40, Critical: true},
				{ID: "documents", SizeGB: 10, Critical: true},
			},
		})

		if err := topo.Save(targetRing); err != nil {
			fmt.Printf("Errore salvataggio ring (%s): %v\n", targetRing, err)
			os.Exit(1)
		}

		fmt.Printf("✓ Aggiunto nodo '%s' (IP: %s, Quota: %d GB) al Ring in '%s'\n", nodeID, meshIP, quotaGB, targetRing)
	},
}

// sbomCmd per M7 (Cyber Resilience Act SBOM)
var sbomCmd = &cobra.Command{
	Use:   "sbom",
	Short: "Genera la distinta base del software (Software Bill of Materials in formato CycloneDX JSON)",
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := sbom.GenerateSBOM("modules")
		if err != nil {
			fmt.Printf("Errore generazione SBOM: %v\n", err)
			os.Exit(1)
		}
		jsonStr, err := doc.ToJSON()
		if err != nil {
			fmt.Printf("Errore codifica JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	},
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Gestione dischi fisici, pool Btrfs RAID 1 e montaggi per il NAS",
}

var storageDisksCmd = &cobra.Command{
	Use:   "disks",
	Short: "Mostra tutti i dischi fisici rilevati e la topologia NAS (RAID 1 / Single / Witness)",
	Run: func(cmd *cobra.Command, args []string) {
		topo := preflight.DetectStorageTopology()
		fmt.Println("=== Dischi Fisici & Pool Storage Allod ===")
		fmt.Printf("Topologia Attuale: %s\n\n", topo.ModeSummary)

		if topo.SystemDisk != nil {
			sys := topo.SystemDisk
			ssdTag := "HDD"
			if sys.IsSSD {
				ssdTag = "SSD"
			}
			fmt.Printf("  • /dev/%-8s [%3d GB, %s] (Disco di Sistema OS - root '/') Modello: %s\n", sys.Name, sys.SizeGB, ssdTag, sys.Model)
		}

		if len(topo.DataDisks) > 0 {
			fmt.Println("\nDischi Dati Rilevati per il Pool NAS:")
			for i, d := range topo.DataDisks {
				ssdTag := "HDD"
				if d.IsSSD {
					ssdTag = "SSD"
				}
				fmt.Printf("  • /dev/%-8s [%3d GB, %s] (Candidato Pool NAS #%d) Modello: %s\n", d.Name, d.SizeGB, ssdTag, i+1, d.Model)
			}
		} else {
			fmt.Println("\nNessun disco secondario per NAS dedicato rilevato.")
		}

		if topo.HasWarning {
			fmt.Printf("\n⚠️ %s\n", topo.WarningMsg)
		}
	},
}

var storageInitCmd = &cobra.Command{
	Use:   "init [dischi...]",
	Short: "Inizializza un pool Btrfs (RAID 1 o Single) sui dischi specificati montandolo su /mnt/allod-storage",
	Run: func(cmd *cobra.Command, args []string) {
		topo := preflight.DetectStorageTopology()
		disks := args
		if len(disks) == 0 {
			if len(topo.DataDisks) == 0 {
				fmt.Println("Nessun disco dati secondario rilevato automaticamente. Specifica i dischi manualmente (es. allod storage init sda sdb).")
				os.Exit(1)
			}
			for _, d := range topo.DataDisks {
				disks = append(disks, d.Name)
			}
		}

		mode := storageInitMode
		if mode == "" {
			if len(disks) >= 2 {
				mode = "raid1"
			} else {
				mode = "single"
			}
		}

		mountPoint := storageInitMount
		if mountPoint == "" {
			mountPoint = "/mnt/allod-storage"
		}

		// Double confirmation if already mounted
		if topo.IsMounted && !storageInitForce {
			fmt.Printf("⚠️  ATTENZIONE CRITICA: Il pool storage su '%s' è GIÀ ESISTENTE e ATTIVO!\n", mountPoint)
			fmt.Println("   Questa operazione CANCELLERÀ e FORMATTERÀ tutti i file, database e foto presenti sui dischi.")
			fmt.Print("   Per confermare la formattazione, digita 'FORMATTA': ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input != "FORMATTA" {
				fmt.Println("\n❌ Operazione annullata in sicurezza. Nessun dato è stato modificato.")
				return
			}
			fmt.Println()
		}

		fmt.Printf("=== Inizializzazione Pool Storage NAS Allod ===\n")
		fmt.Printf("Dischi selezionati: %v\n", disks)
		fmt.Printf("Profilo Btrfs:      %s\n", strings.ToUpper(mode))
		fmt.Printf("Punto di mount:     %s\n\n", mountPoint)

		// Call helper daemon over socket
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		res, err := client.Execute("storage.init", map[string]interface{}{
			"disks": disks,
			"mode":  mode,
			"mount": mountPoint,
			"user":  os.Getenv("USER"),
		}, false)

		if err != nil {
			client.SocketPath = "allod-helper.sock"
			res, err = client.Execute("storage.init", map[string]interface{}{
				"disks": disks,
				"mode":  mode,
				"mount": mountPoint,
				"user":  os.Getenv("USER"),
			}, false)
		}

		if err != nil || !res.Ok {
			errMsg := "Errore comunicazione con allod-helperd (assicurati che sia avviato con sudo)"
			if err != nil {
				errMsg = err.Error()
			} else if res.Error != "" {
				errMsg = res.Error
			}
			fmt.Printf("❌ %s\n", errMsg)
			os.Exit(1)
		}

		fmt.Printf("✓ Pool Btrfs %s creato e montato con successo su %s!\n", strings.ToUpper(mode), mountPoint)
		fmt.Printf("✓ Subvolume permanenti creati: %s/{cloud, photos, shares, backup}\n", mountPoint)
		fmt.Println("✓ Ora i container salveranno tutti i file direttamente sui tuoi hard disk fisici.")
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "configs/config.example.yaml", "file di configurazione")
	rootCmd.PersistentFlags().StringVar(&stateDB, "state-db", "state.db", "percorso file state.db")
	rootCmd.PersistentFlags().StringVar(&ringFile, "ring", "configs/ring.example.yaml", "percorso file topologia ring")
	setCmd.Flags().BoolVar(&acceptRisk, "accept-risk", false, "Ignora i requisiti hardware")

	applyCmd.Flags().BoolVar(&useSystemd, "systemd", false, "Genera direttamente nella cartella Quadlet di sistema Ubuntu")
	applyCmd.Flags().StringVar(&outDirOverride, "out-dir", "", "Percorso personalizzato per i file Quadlet generati")

	ringSimulateCmd.Flags().StringVar(&removeMember, "remove", "", "ID del membro da simulare la rimozione")

	storageInitCmd.Flags().StringVar(&storageInitMode, "mode", "", "Profilo Btrfs (raid1 oppure single)")
	storageInitCmd.Flags().StringVar(&storageInitMount, "mount", "/mnt/allod-storage", "Punto di montaggio del pool NAS")
	storageInitCmd.Flags().BoolVarP(&storageInitForce, "force", "f", false, "Forza la formattazione senza prompt interattivo di sicurezza")

	storageCmd.AddCommand(storageDisksCmd)
	storageCmd.AddCommand(storageInitCmd)

	ringCmd.AddCommand(ringStatusCmd)
	ringCmd.AddCommand(ringSimulateCmd)
	ringCmd.AddCommand(ringAddCmd)

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(ringCmd)
	rootCmd.AddCommand(sbomCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
