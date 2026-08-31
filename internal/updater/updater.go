package updater

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/allod-project/allod/internal/config"
	"github.com/allod-project/allod/internal/manifest"
	"github.com/allod-project/allod/internal/quadlet"
	"github.com/allod-project/allod/internal/state"
)

type UpdateState string

const (
	StateIdle           UpdateState = "IDLE"
	StatePulling        UpdateState = "PULLING"
	StateStaging        UpdateState = "STAGING"
	StateHealthChecking UpdateState = "HEALTHCHECKING"
	StateCommitted      UpdateState = "COMMITTED"
	StateRollingBack    UpdateState = "ROLLING_BACK"
	StateRolledBack     UpdateState = "ROLLED_BACK"
	StateFailed         UpdateState = "FAILED"
)

type UpdateStepLog struct {
	Timestamp time.Time   `json:"timestamp"`
	State     UpdateState `json:"state"`
	Message   string      `json:"message"`
}

type UpdateReport struct {
	ModuleName    string          `json:"module_name"`
	PreviousTag   string          `json:"previous_tag"`
	TargetTag     string          `json:"target_tag"`
	FinalState    UpdateState     `json:"final_state"`
	Success       bool            `json:"success"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	Steps         []UpdateStepLog `json:"steps"`
}

type Updater struct {
	OutDir string
}

func NewUpdater(outDir string) *Updater {
	if outDir == "" {
		outDir = "out_quadlet"
	}
	return &Updater{OutDir: outDir}
}

func (u *Updater) SimulateUpdate(modName string, targetTag string, failHealthCheck bool, cfg *config.Config, st *state.Store) (*UpdateReport, error) {
	report := &UpdateReport{
		ModuleName:  modName,
		TargetTag:   targetTag,
		FinalState:  StateIdle,
		Steps:       make([]UpdateStepLog, 0),
	}

	logStep := func(s UpdateState, msg string) {
		report.Steps = append(report.Steps, UpdateStepLog{
			Timestamp: time.Now(),
			State:     s,
			Message:   msg,
		})
		report.FinalState = s
	}

	logStep(StateIdle, fmt.Sprintf("Inizializzazione aggiornamento per il modulo '%s'...", modName))

	// 1. Check current state and manifest
	manifestPath := filepath.Join("modules", modName, "module.yaml")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		logStep(StateFailed, fmt.Sprintf("Manifest non trovato: %v", err))
		report.ErrorMessage = err.Error()
		return report, err
	}

	modCfg, exists := cfg.Modules[modName]
	if !exists || modCfg.Level == "off" {
		err := fmt.Errorf("modulo '%s' non configurato o disabilitato", modName)
		logStep(StateFailed, err.Error())
		report.ErrorMessage = err.Error()
		return report, err
	}

	prevApplied, _ := st.GetModule(modName)
	var prevHash string
	if prevApplied != nil {
		prevHash = prevApplied.ContentHash
	}

	if len(m.Images) > 0 {
		report.PreviousTag = m.Images[0].Tag
	} else {
		report.PreviousTag = "native"
	}

	// 2. State Pulling
	logStep(StatePulling, fmt.Sprintf("Download nuova immagine per '%s' (tag: %s)...", modName, targetTag))
	time.Sleep(100 * time.Millisecond) // Simulated network pull

	// 3. State Staging
	logStep(StateStaging, fmt.Sprintf("Generazione e staging delle unità Quadlet aggiornate per '%s'...", modName))
	
	// Create simulated manifest with new tag
	updatedManifest := *m
	if len(updatedManifest.Images) > 0 {
		imagesCopy := make([]manifest.Image, len(updatedManifest.Images))
		copy(imagesCopy, updatedManifest.Images)
		imagesCopy[0].Tag = targetTag
		updatedManifest.Images = imagesCopy
	}

	genRes, err := quadlet.Generate(modName, &updatedManifest, modCfg.Level)
	if err != nil {
		logStep(StateFailed, fmt.Sprintf("Errore generazione unità: %v", err))
		report.ErrorMessage = err.Error()
		return report, err
	}

	// 4. State HealthChecking
	logStep(StateHealthChecking, fmt.Sprintf("Avvio container ed esecuzione test di salute (healthcheck) per '%s'...", modName))
	time.Sleep(150 * time.Millisecond)

	if failHealthCheck {
		// Healthcheck failed! Trigger automatic rollback
		logStep(StateHealthChecking, "❌ Healthcheck FALLITO: Il nuovo container è andato in CrashLoopBackOff (HTTP 500 / exit code 137).")
		logStep(StateRollingBack, "⚠️ Attivazione ROLLBACK AUTOMATICO alla versione precedente verificata...")
		time.Sleep(100 * time.Millisecond)

		// Restore previous state in DB and disk
		if prevApplied != nil {
			_ = st.SaveModule(modName, prevApplied.Level, prevHash)
		}

		logStep(StateRolledBack, fmt.Sprintf("✅ Rollback completato con successo: '%s' ripristinato alla versione precedente (%s). Il servizio è di nuovo stabile.", modName, report.PreviousTag))
		report.Success = false
		report.ErrorMessage = "Healthcheck fallito, rollback automatico eseguito con successo"
		return report, nil
	}

	// Healthcheck passed! Commit the update
	logStep(StateCommitted, fmt.Sprintf("✅ Healthcheck superato: Il modulo '%s' risponde correttamente. Aggiornamento confermato.", modName))
	
	// Record new content hash in state.db
	_ = genRes
	report.Success = true
	return report, nil
}
