package preflight

import (
	"testing"
)

func TestGetTopProcesses(t *testing.T) {
	procs, err := GetTopProcesses(5, "cpu")
	if err != nil {
		t.Fatalf("unexpected error getting top processes: %v", err)
	}
	if len(procs) == 0 {
		t.Fatalf("expected at least 1 process returned")
	}
	if len(procs) > 5 {
		t.Fatalf("expected at most 5 processes, got %d", len(procs))
	}

	// Verify CPU sorting
	for i := 1; i < len(procs); i++ {
		if procs[i].CPUPercent > procs[i-1].CPUPercent {
			t.Errorf("processes not properly sorted by CPU: index %d (%f) > index %d (%f)",
				i, procs[i].CPUPercent, i-1, procs[i-1].CPUPercent)
		}
	}

	// Test Memory sorting
	procsMem, err := GetTopProcesses(5, "mem")
	if err != nil {
		t.Fatalf("unexpected error getting top processes by mem: %v", err)
	}
	for i := 1; i < len(procsMem); i++ {
		if procsMem[i].MemoryMB > procsMem[i-1].MemoryMB {
			t.Errorf("processes not properly sorted by Memory: index %d (%d) > index %d (%d)",
				i, procsMem[i].MemoryMB, i-1, procsMem[i-1].MemoryMB)
		}
	}
}

func TestGetTopProcessSummary(t *testing.T) {
	summary := GetTopProcessSummary()
	// In mock or on linux, if CPU > 1.0, should contain "%"
	if summary != "" && !testing.Short() {
		if len(summary) < 3 {
			t.Errorf("unexpected short summary: %q", summary)
		}
	}
}

func TestSortAndTrimProcesses(t *testing.T) {
	list := []ProcessInfo{
		{PID: 1, Name: "procA", CPUPercent: 5.0, MemoryMB: 100},
		{PID: 2, Name: "procB", CPUPercent: 25.0, MemoryMB: 50},
		{PID: 3, Name: "procC", CPUPercent: 12.0, MemoryMB: 200},
	}

	sortedCPU := sortAndTrimProcesses(list, 2, "cpu")
	if len(sortedCPU) != 2 {
		t.Fatalf("expected 2 items, got %d", len(sortedCPU))
	}
	if sortedCPU[0].PID != 2 || sortedCPU[1].PID != 3 {
		t.Errorf("expected [procB, procC], got [%s, %s]", sortedCPU[0].Name, sortedCPU[1].Name)
	}

	sortedMem := sortAndTrimProcesses(list, 2, "mem")
	if len(sortedMem) != 2 {
		t.Fatalf("expected 2 items, got %d", len(sortedMem))
	}
	if sortedMem[0].PID != 3 || sortedMem[1].PID != 1 {
		t.Errorf("expected [procC, procA], got [%s, %s]", sortedMem[0].Name, sortedMem[1].Name)
	}
}
