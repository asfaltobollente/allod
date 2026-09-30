package preflight

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProcessInfo represents runtime process statistics.
type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	Cmdline    string  `json:"cmdline,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   int64   `json:"memory_mb"`
	MemoryPct  float64 `json:"memory_pct"`
	Container  string  `json:"container,omitempty"`
	User       string  `json:"user,omitempty"`
	State      string  `json:"state,omitempty"`
}

type procSample struct {
	ticks uint64
}

// ProcessTracker caches process CPU ticks across measurements for accurate differential CPU%.
type ProcessTracker struct {
	mu         sync.Mutex
	lastTotal  uint64
	lastProcs  map[int]procSample
	lastSample time.Time
}

var (
	defaultTracker = &ProcessTracker{
		lastProcs: make(map[int]procSample),
	}
)

// GetTopProcesses returns the top resource-consuming processes, sorted by sortBy ("cpu" or "mem").
func GetTopProcesses(limit int, sortBy string) ([]ProcessInfo, error) {
	if limit <= 0 {
		limit = 10
	}
	if sortBy == "" {
		sortBy = "cpu"
	}

	if runtime.GOOS != "linux" {
		return getMockProcesses(limit, sortBy), nil
	}

	return defaultTracker.Sample(limit, sortBy)
}

// GetTopProcessSummary returns a concise string identifying the top CPU process if usage is notable.
// e.g. "immich-server (21.4%)" or "[photos] python3 (15.2%)". Returns empty if system is idle.
func GetTopProcessSummary() string {
	procs, err := GetTopProcesses(3, "cpu")
	if err != nil || len(procs) == 0 {
		return ""
	}

	top := procs[0]
	if top.CPUPercent < 1.0 {
		return ""
	}

	displayName := top.Name
	if top.Container != "" {
		displayName = fmt.Sprintf("[%s] %s", top.Container, top.Name)
	}

	return fmt.Sprintf("%s (%.1f%%)", displayName, top.CPUPercent)
}

// Sample collects processes and calculates CPU% based on differential ticks since last call.
func (pt *ProcessTracker) Sample(limit int, sortBy string) ([]ProcessInfo, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	totalTicks, err := readSystemTotalCPUTicks()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	deltaTotal := uint64(0)
	isFirstSample := (pt.lastTotal == 0 || len(pt.lastProcs) == 0)

	if !isFirstSample && totalTicks > pt.lastTotal {
		deltaTotal = totalTicks - pt.lastTotal
	}

	// Read /proc meminfo for total RAM to calculate MemoryPct
	ramStats := GetRealRAMStats()
	totalRAMMB := int64(ramStats.TotalMB)
	if totalRAMMB <= 0 {
		totalRAMMB = 8192
	}

	// Read process directories
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	currentProcs := make(map[int]procSample)
	var procList []ProcessInfo

	pageSize := int64(os.Getpagesize())
	if pageSize <= 0 {
		pageSize = 4096
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		pInfo, ticks, ok := readProcessStat(pid, pageSize, totalRAMMB)
		if !ok {
			continue
		}

		currentProcs[pid] = procSample{ticks: ticks}

		// Calculate CPU%
		var cpuPct float64
		if deltaTotal > 0 {
			if prev, exists := pt.lastProcs[pid]; exists && ticks >= prev.ticks {
				diffProc := ticks - prev.ticks
				cpuPct = float64(diffProc) / float64(deltaTotal) * 100.0
			}
		}

		// Format CPU% to 1 decimal place
		pInfo.CPUPercent = float64(int(cpuPct*10)) / 10.0
		procList = append(procList, pInfo)
	}

	// Update cached state
	pt.lastTotal = totalTicks
	pt.lastProcs = currentProcs
	pt.lastSample = now

	// If this was the first sample or differential had 0 elapsed ticks, do a quick micro-sample
	if isFirstSample && len(procList) > 0 {
		time.Sleep(120 * time.Millisecond)
		// Recurse once with established baseline
		return pt.sampleInternal(limit, sortBy, totalRAMMB, pageSize)
	}

	return sortAndTrimProcesses(procList, limit, sortBy), nil
}

func (pt *ProcessTracker) sampleInternal(limit int, sortBy string, totalRAMMB, pageSize int64) ([]ProcessInfo, error) {
	totalTicks, err := readSystemTotalCPUTicks()
	if err != nil {
		return nil, err
	}

	deltaTotal := uint64(0)
	if totalTicks > pt.lastTotal {
		deltaTotal = totalTicks - pt.lastTotal
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	currentProcs := make(map[int]procSample)
	var procList []ProcessInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		pInfo, ticks, ok := readProcessStat(pid, pageSize, totalRAMMB)
		if !ok {
			continue
		}

		currentProcs[pid] = procSample{ticks: ticks}

		var cpuPct float64
		if deltaTotal > 0 {
			if prev, exists := pt.lastProcs[pid]; exists && ticks >= prev.ticks {
				diffProc := ticks - prev.ticks
				cpuPct = float64(diffProc) / float64(deltaTotal) * 100.0
			}
		}

		pInfo.CPUPercent = float64(int(cpuPct*10)) / 10.0
		procList = append(procList, pInfo)
	}

	pt.lastTotal = totalTicks
	pt.lastProcs = currentProcs
	pt.lastSample = time.Now()

	return sortAndTrimProcesses(procList, limit, sortBy), nil
}

func readProcessStat(pid int, pageSize, totalRAMMB int64) (ProcessInfo, uint64, bool) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return ProcessInfo{}, 0, false
	}

	content := string(data)
	openIdx := strings.IndexByte(content, '(')
	closeIdx := strings.LastIndexByte(content, ')')
	if openIdx < 0 || closeIdx <= openIdx {
		return ProcessInfo{}, 0, false
	}

	name := content[openIdx+1 : closeIdx]
	after := strings.Fields(content[closeIdx+1:])
	if len(after) < 14 {
		return ProcessInfo{}, 0, false
	}

	state := after[0]
	utime, _ := strconv.ParseUint(after[11], 10, 64)
	stime, _ := strconv.ParseUint(after[12], 10, 64)
	totalProcTicks := utime + stime

	// Read Memory from /proc/[pid]/statm
	var memMB int64
	var memPct float64
	if statmData, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid)); err == nil {
		fields := strings.Fields(string(statmData))
		if len(fields) >= 2 {
			if residentPages, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				memMB = (residentPages * pageSize) / (1024 * 1024)
				if totalRAMMB > 0 {
					memPct = float64(memMB) / float64(totalRAMMB) * 100.0
					memPct = float64(int(memPct*10)) / 10.0
				}
			}
		}
	}

	// Read Cmdline (first 100 chars max)
	cmdline := ""
	if cmdData, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil && len(cmdData) > 0 {
		cleaned := strings.ReplaceAll(string(cmdData), "\x00", " ")
		cleaned = strings.TrimSpace(cleaned)
		if len(cleaned) > 100 {
			cleaned = cleaned[:97] + "..."
		}
		cmdline = cleaned
	}

	// Detect Container/Unit from /proc/[pid]/cgroup
	container := detectContainerOrUnit(pid)

	return ProcessInfo{
		PID:        pid,
		Name:       name,
		Cmdline:    cmdline,
		MemoryMB:   memMB,
		MemoryPct:  memPct,
		Container:  container,
		State:      state,
	}, totalProcTicks, true
}

func detectContainerOrUnit(pid int) string {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Pattern 1: Quadlet / Systemd Service (.../photoprism.service or .../systemd-photos.service)
		if strings.Contains(line, ".service") {
			parts := strings.Split(line, "/")
			for _, part := range parts {
				if strings.HasSuffix(part, ".service") {
					unit := strings.TrimSuffix(part, ".service")
					// Clean up prefixes like systemd- or app-
					unit = strings.TrimPrefix(unit, "systemd-")
					unit = strings.TrimPrefix(unit, "app-")
					if unit != "systemd" && unit != "init" && unit != "allod" {
						return unit
					}
				}
			}
		}

		// Pattern 2: Podman libpod container (.../libpod-<id>.scope or /podman-<name>)
		if strings.Contains(line, "libpod") || strings.Contains(line, "podman") {
			parts := strings.Split(line, "/")
			for _, p := range parts {
				if strings.HasPrefix(p, "libpod-") {
					id := strings.TrimPrefix(p, "libpod-")
					id = strings.TrimSuffix(id, ".scope")
					if len(id) > 12 {
						id = id[:12]
					}
					return "podman-" + id
				}
			}
			return "container"
		}
	}
	return ""
}

func readSystemTotalCPUTicks() (uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			var total uint64
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
			}
			return total, nil
		}
	}
	return 0, fmt.Errorf("cpu line not found in /proc/stat")
}

func sortAndTrimProcesses(list []ProcessInfo, limit int, sortBy string) []ProcessInfo {
	if sortBy == "mem" {
		sort.Slice(list, func(i, j int) bool {
			if list[i].MemoryMB == list[j].MemoryMB {
				return list[i].CPUPercent > list[j].CPUPercent
			}
			return list[i].MemoryMB > list[j].MemoryMB
		})
	} else {
		sort.Slice(list, func(i, j int) bool {
			if list[i].CPUPercent == list[j].CPUPercent {
				return list[i].MemoryMB > list[j].MemoryMB
			}
			return list[i].CPUPercent > list[j].CPUPercent
		})
	}

	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

func getMockProcesses(limit int, sortBy string) []ProcessInfo {
	mock := []ProcessInfo{
		{PID: 1042, Name: "immich-server", Cmdline: "node ./dist/main.js", CPUPercent: 18.5, MemoryMB: 680, MemoryPct: 8.5, Container: "photos", State: "S"},
		{PID: 1108, Name: "allod-panel", Cmdline: "/usr/local/bin/allod-panel", CPUPercent: 3.2, MemoryMB: 48, MemoryPct: 0.6, Container: "", State: "S"},
		{PID: 1215, Name: "valkey-server", Cmdline: "valkey-server *:6379", CPUPercent: 1.8, MemoryMB: 32, MemoryPct: 0.4, Container: "valkey", State: "S"},
		{PID: 954, Name: "postgres", Cmdline: "postgres -D /var/lib/postgresql/data", CPUPercent: 1.2, MemoryMB: 210, MemoryPct: 2.6, Container: "db", State: "S"},
		{PID: 884, Name: "btrfs-cleaner", Cmdline: "[btrfs-cleaner]", CPUPercent: 0.8, MemoryMB: 0, MemoryPct: 0.0, Container: "", State: "D"},
		{PID: 1429, Name: "podman", Cmdline: "podman stats --no-stream", CPUPercent: 0.4, MemoryMB: 28, MemoryPct: 0.3, Container: "", State: "R"},
		{PID: 742, Name: "caddy", Cmdline: "caddy run --config /etc/caddy/Caddyfile", CPUPercent: 0.2, MemoryMB: 54, MemoryPct: 0.7, Container: "proxy", State: "S"},
		{PID: 1, Name: "systemd", Cmdline: "/sbin/init", CPUPercent: 0.1, MemoryMB: 16, MemoryPct: 0.2, Container: "", State: "S"},
	}

	return sortAndTrimProcesses(mock, limit, sortBy)
}
