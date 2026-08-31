package preflight

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/allod-project/allod/internal/config"
	"github.com/allod-project/allod/internal/manifest"
)

const (
	DefaultSystemRAMMB = 8192
	CoreReservedMB     = 600
)

// GetSystemRAMMB reads total memory from /proc/meminfo on Linux, or falls back to default.
func GetSystemRAMMB() int {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return DefaultSystemRAMMB
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.Atoi(fields[1])
				if err == nil && kb > 0 {
					return kb / 1024
				}
			}
		}
	}
	return DefaultSystemRAMMB
}

type CheckResult struct {
	Pass       bool
	Message    string
	TotalReqMB int
	AvailMB    int
	Committed  int
	ActiveMods []string
}

// CommittedRAMInfo returns the sum of ram_mb of active modules and their names.
func CommittedRAMInfo(cfg *config.Config, excludeModName string) (int, []string) {
	total := 0
	var active []string
	for name, modCfg := range cfg.Modules {
		if name == excludeModName || modCfg.Level == "off" {
			continue
		}
		mPath := filepath.Join("modules", name, "module.yaml")
		m, err := manifest.LoadManifest(mPath)
		if err != nil {
			continue
		}
		level, ok := m.Levels[modCfg.Level]
		if !ok {
			continue
		}
		total += level.RAMMB
		active = append(active, name)
	}
	return total, active
}

// Check evaluates if a requested module level can run on the system.
func Check(cfg *config.Config, modName string, m *manifest.Manifest, requestedLevel string) CheckResult {
	if requestedLevel == "off" {
		return CheckResult{Pass: true}
	}

	level, ok := m.Levels[requestedLevel]
	if !ok {
		return CheckResult{Pass: false, Message: fmt.Sprintf("Livello '%s' inesistente", requestedLevel)}
	}

	totalSysRAM := GetSystemRAMMB()
	committed, activeMods := CommittedRAMInfo(cfg, modName)
	availMB := totalSysRAM - CoreReservedMB - committed

	res := CheckResult{
		TotalReqMB: level.RAMMB,
		AvailMB:    availMB,
		Committed:  committed,
		ActiveMods: activeMods,
	}

	// 1. Dependency check (requires.modules)
	for _, reqMod := range level.Requires.Modules {
		reqCfg, exists := cfg.Modules[reqMod]
		if !exists || reqCfg.Level == "off" || reqCfg.Level == "" {
			res.Pass = false
			res.Message = fmt.Sprintf("Richiede il modulo '%s' attivo", reqMod)
			return res
		}
	}

	// 2. Total system RAM check
	if level.Requires.TotalRAMMB > 0 && totalSysRAM < level.Requires.TotalRAMMB {
		res.Pass = false
		res.Message = fmt.Sprintf("Richiede %d GB totali di sistema, ne hai %d GB", level.Requires.TotalRAMMB/1024, totalSysRAM/1024)
		return res
	}

	// 3. Available RAM check
	if level.RAMMB > availMB {
		res.Pass = false
		res.Message = fmt.Sprintf("Memoria disponibile insufficiente (disponibile: %d MB, richiesto: %d MB)", availMB, level.RAMMB)
		return res
	}

	res.Pass = true
	return res
}

func FormatActiveMods(mods []string) string {
	if len(mods) == 0 {
		return "nessuno"
	}
	return strings.Join(mods, ", ")
}
