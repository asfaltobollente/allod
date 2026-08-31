package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allod-project/allod/internal/cloudinit"
	"github.com/allod-project/allod/internal/config"
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
	useSystemd     bool
	outDirOverride string
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

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "configs/config.example.yaml", "file di configurazione")
	rootCmd.PersistentFlags().StringVar(&stateDB, "state-db", "state.db", "percorso file state.db")
	rootCmd.PersistentFlags().StringVar(&ringFile, "ring", "configs/ring.example.yaml", "percorso file topologia ring")
	setCmd.Flags().BoolVar(&acceptRisk, "accept-risk", false, "Ignora i requisiti hardware")

	applyCmd.Flags().BoolVar(&useSystemd, "systemd", false, "Genera direttamente nella cartella Quadlet di sistema Ubuntu")
	applyCmd.Flags().StringVar(&outDirOverride, "out-dir", "", "Percorso personalizzato per i file Quadlet generati")

	ringSimulateCmd.Flags().StringVar(&removeMember, "remove", "", "ID del membro da simulare la rimozione")

	ringCmd.AddCommand(ringStatusCmd)
	ringCmd.AddCommand(ringSimulateCmd)

	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(doctorCmd)
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
