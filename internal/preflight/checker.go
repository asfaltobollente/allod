package preflight

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

type DiskInfo struct {
	Name       string `json:"name"`
	SizeGB     int64  `json:"size_gb"`
	Type       string `json:"type"`
	Mountpoint string `json:"mountpoint"`
	Model      string `json:"model"`
	IsSystem   bool   `json:"is_system"`
	IsSSD      bool   `json:"is_ssd"`
}

type StorageTopology struct {
	TotalDisks  int        `json:"total_disks"`
	SystemDisk  *DiskInfo  `json:"system_disk,omitempty"`
	DataDisks   []DiskInfo `json:"data_disks"`
	Mode        string     `json:"mode"`
	ModeSummary string     `json:"mode_summary"`
	HasWarning  bool       `json:"has_warning"`
	WarningMsg  string     `json:"warning_msg,omitempty"`
	IsMounted   bool       `json:"is_mounted"`
	MountPoint  string     `json:"mount_point"`
}

type lsblkOutput struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Size       interface{}   `json:"size"`
	Type       string        `json:"type"`
	Mountpoint *string       `json:"mountpoint"`
	Model      *string       `json:"model"`
	Rota       interface{}   `json:"rota"`
	Children   []lsblkDevice `json:"children,omitempty"`
}

// DetectStorageTopology detects physical drives on Linux and determines the NAS storage tier.
func DetectStorageTopology() StorageTopology {
	topo := StorageTopology{
		DataDisks: []DiskInfo{},
	}

	cmd := exec.Command("lsblk", "-J", "-b", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,MODEL,ROTA")
	out, err := cmd.Output()
	if err != nil {
		topo.TotalDisks = 1
		topo.Mode = "witness"
		topo.ModeSummary = "Modalità Witness (0 Dischi Dati Dedicati)"
		topo.HasWarning = true
		topo.WarningMsg = "Nessun disco NAS secondario rilevato (Ambiente simulato / Non-Linux)"
		return topo
	}

	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		topo.Mode = "witness"
		topo.ModeSummary = "Modalità Witness"
		return topo
	}

	for _, dev := range parsed.Blockdevices {
		if dev.Type != "disk" {
			continue
		}
		if strings.HasPrefix(dev.Name, "loop") || strings.HasPrefix(dev.Name, "zram") {
			continue
		}

		isSys := hasRootMount(dev)
		sizeGB := parseSizeGB(dev.Size)
		model := ""
		if dev.Model != nil {
			model = strings.TrimSpace(*dev.Model)
		}
		isSSD := !parseRota(dev.Rota)
		mp := ""
		if dev.Mountpoint != nil {
			mp = *dev.Mountpoint
		}

		dInfo := DiskInfo{
			Name:       dev.Name,
			SizeGB:     sizeGB,
			Type:       dev.Type,
			Mountpoint: mp,
			Model:      model,
			IsSystem:   isSys,
			IsSSD:      isSSD,
		}

		topo.TotalDisks++
		if isSys {
			dCopy := dInfo
			topo.SystemDisk = &dCopy
		} else {
			topo.DataDisks = append(topo.DataDisks, dInfo)
		}
	}

	if len(topo.DataDisks) >= 2 {
		topo.Mode = "raid1"
		topo.ModeSummary = fmt.Sprintf("Btrfs RAID 1 Mirroring (%d Dischi Dati Dedicati - Ridondanza Completa)", len(topo.DataDisks))
		topo.HasWarning = false
	} else if len(topo.DataDisks) == 1 {
		topo.Mode = "single"
		topo.ModeSummary = "Btrfs Single (1 Disco Dati)"
		topo.HasWarning = true
		topo.WarningMsg = "Attenzione: 1 solo disco dati. Nessuna ridondanza RAID 1 hardware locale. Il ripristino in caso di guasto meccanico dipenderà dalle repliche remote del Ring."
	} else {
		topo.Mode = "witness"
		topo.ModeSummary = "Modalità Witness / Cassaforte Remota (0 Dischi Dati NAS)"
		topo.HasWarning = true
		topo.WarningMsg = "Nessun disco dati secondario rilevato. Il server opera in modalità Witness preservando il disco dell'OS e abilitando solo la cassaforte di backup cifrata per gli amici."
	}

	if _, err := os.Stat("/mnt/allod-storage"); err == nil {
		topo.IsMounted = true
		topo.MountPoint = "/mnt/allod-storage"
	}

	return topo
}

func hasRootMount(dev lsblkDevice) bool {
	if dev.Mountpoint != nil && *dev.Mountpoint == "/" {
		return true
	}
	for _, child := range dev.Children {
		if hasRootMount(child) {
			return true
		}
	}
	return false
}

func parseSizeGB(raw interface{}) int64 {
	switch v := raw.(type) {
	case float64:
		return int64(v / (1024 * 1024 * 1024))
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n / (1024 * 1024 * 1024)
	default:
		return 0
	}
}

func parseRota(raw interface{}) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return v == "1" || v == "true"
	case float64:
		return v == 1
	default:
		return true
	}
}

// RealRAMStats holds live physical memory metrics read from /proc/meminfo.
type RealRAMStats struct {
	TotalMB     int `json:"total_mb"`
	UsedMB      int `json:"used_mb"`
	AvailableMB int `json:"available_mb"`
	FreeMB      int `json:"free_mb"`
}

// GetRealRAMStats reads actual real-time memory stats from /proc/meminfo (matching free -h).
func GetRealRAMStats() RealRAMStats {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return RealRAMStats{
			TotalMB:     DefaultSystemRAMMB,
			UsedMB:      1300,
			AvailableMB: DefaultSystemRAMMB - 1300,
			FreeMB:      DefaultSystemRAMMB - 1300,
		}
	}
	defer file.Close()

	var memTotalKB, memFreeKB, memAvailableKB, buffersKB, cachedKB int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.Atoi(fields[1])
		if strings.HasPrefix(line, "MemTotal:") {
			memTotalKB = val
		} else if strings.HasPrefix(line, "MemFree:") {
			memFreeKB = val
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailableKB = val
		} else if strings.HasPrefix(line, "Buffers:") {
			buffersKB = val
		} else if strings.HasPrefix(line, "Cached:") {
			cachedKB = val
		}
	}

	totalMB := memTotalKB / 1024
	if totalMB <= 0 {
		totalMB = DefaultSystemRAMMB
	}

	var usedMB, availMB int
	if memAvailableKB > 0 {
		availMB = memAvailableKB / 1024
		usedMB = (memTotalKB - memAvailableKB) / 1024
	} else {
		freeKB := memFreeKB + buffersKB + cachedKB
		usedMB = (memTotalKB - freeKB) / 1024
		availMB = totalMB - usedMB
	}
	if usedMB < 0 {
		usedMB = 0
	}
	if availMB < 0 {
		availMB = 0
	}

	return RealRAMStats{
		TotalMB:     totalMB,
		UsedMB:      usedMB,
		AvailableMB: availMB,
		FreeMB:      memFreeKB / 1024,
	}
}

// GetSystemRAMMB reads total memory from /proc/meminfo on Linux, or falls back to default.
func GetSystemRAMMB() int {
	return GetRealRAMStats().TotalMB
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
