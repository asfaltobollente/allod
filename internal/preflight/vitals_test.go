package preflight

import (
	"testing"
)

func TestClassifyTempStatus(t *testing.T) {
	tests := []struct {
		temp     float64
		expected string
	}{
		{0.0, "normal"},
		{45.2, "normal"},
		{59.9, "normal"},
		{60.0, "warm"},
		{74.9, "warm"},
		{75.0, "hot"},
		{84.9, "hot"},
		{85.0, "critical"},
		{95.0, "critical"},
	}

	for _, tt := range tests {
		got := ClassifyTempStatus(tt.temp)
		if got != tt.expected {
			t.Errorf("ClassifyTempStatus(%.1f) = %s; want %s", tt.temp, got, tt.expected)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		seconds  int64
		expected string
	}{
		{0, "--"},
		{-10, "--"},
		{120, "2m"},
		{3600, "1h 0m"},
		{3720, "1h 2m"},
		{86400, "1d 0h 0m"},
		{90060, "1d 1h 1m"},
		{345678, "4d 0h 1m"},
	}

	for _, tt := range tests {
		got := FormatUptime(tt.seconds)
		if got != tt.expected {
			t.Errorf("FormatUptime(%d) = %s; want %s", tt.seconds, got, tt.expected)
		}
	}
}

func TestGetServerVitals(t *testing.T) {
	v := GetServerVitals()
	if v.CPUCores <= 0 {
		t.Errorf("expected CPUCores > 0, got %d", v.CPUCores)
	}
	if v.TempStatus == "" {
		t.Errorf("expected non-empty TempStatus")
	}
}
