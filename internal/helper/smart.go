package helper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// SmartReport holds normalized SMART telemetry for a drive.
type SmartReport struct {
	Device               string `json:"device"`
	DevicePath           string `json:"device_path"`
	ModelFamily          string `json:"model_family,omitempty"`
	ModelName            string `json:"model_name,omitempty"`
	SerialNumber         string `json:"serial_number,omitempty"`
	UserCapacity         string `json:"user_capacity,omitempty"`
	RotationRate         int    `json:"rotation_rate,omitempty"`
	IsSSD                bool   `json:"is_ssd"`
	SmartStatus          string `json:"smart_status"` // "PASSED", "FAILING", "UNAVAILABLE", "UNKNOWN"
	SmartPassed          bool   `json:"smart_passed"`
	TemperatureC         int    `json:"temperature_c,omitempty"`
	PowerOnHours         int64  `json:"power_on_hours,omitempty"`
	PowerCycleCount      int64  `json:"power_cycle_count,omitempty"`
	ReallocatedSectors   int64  `json:"reallocated_sectors"`
	PendingSectors       int64  `json:"pending_sectors"`
	UncorrectableSectors int64  `json:"uncorrectable_sectors"`
	RawJSON              string `json:"raw_json,omitempty"`
	Error                string `json:"error,omitempty"`
}

// rawSmartctlJSON maps the subset of smartmontools 7.0+ JSON schema used by Allod.
type rawSmartctlJSON struct {
	JSONFormatVersion []int `json:"json_format_version"`
	Device            struct {
		Name     string `json:"name"`
		InfoName string `json:"info_name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelFamily  string `json:"model_family"`
	ModelName    string `json:"model_name"`
	SerialNumber string `json:"serial_number"`
	UserCapacity struct {
		Bytes int64 `json:"bytes"`
	} `json:"user_capacity"`
	RotationRate int `json:"rotation_rate"`
	SmartStatus  struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount int64 `json:"power_cycle_count"`
	AtaSmartAttributes struct {
		Table []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value int64  `json:"value"`
				Str   string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NvmeSmartHealth struct {
		CriticalWarning int   `json:"critical_warning"`
		Temperature     int   `json:"temperature"`
		AvailableSpare  int   `json:"available_spare"`
		PercentageUsed  int   `json:"percentage_used"`
		PowerOnHours    int64 `json:"power_on_hours"`
		MediaErrors     int64 `json:"media_errors"`
	} `json:"nvme_smart_health_information_log"`
}

// ParseSmartctlOutput parses JSON bytes from smartctl -a -j / smartctl -H -j.
func ParseSmartctlOutput(device, devPath string, raw []byte) (*SmartReport, error) {
	report := &SmartReport{
		Device:      device,
		DevicePath:  devPath,
		SmartStatus: "UNKNOWN",
	}

	if len(raw) == 0 {
		return report, fmt.Errorf("empty output from smartctl")
	}

	var parsed rawSmartctlJSON
	if err := json.Unmarshal(raw, &parsed); err != nil {
		report.RawJSON = string(raw)
		return report, fmt.Errorf("failed to parse smartctl JSON: %w", err)
	}

	report.ModelFamily = parsed.ModelFamily
	report.ModelName = parsed.ModelName
	report.SerialNumber = parsed.SerialNumber
	report.RotationRate = parsed.RotationRate
	report.PowerCycleCount = parsed.PowerCycleCount

	if parsed.RotationRate == 0 || parsed.Device.Protocol == "NVMe" {
		report.IsSSD = true
	}

	if parsed.UserCapacity.Bytes > 0 {
		gb := float64(parsed.UserCapacity.Bytes) / (1000 * 1000 * 1000)
		if gb >= 1000 {
			report.UserCapacity = fmt.Sprintf("%.1f TB", gb/1000)
		} else {
			report.UserCapacity = fmt.Sprintf("%.0f GB", gb)
		}
	}

	// SATA / ATA SMART evaluation
	if parsed.SmartStatus.Passed {
		report.SmartPassed = true
		report.SmartStatus = "PASSED"
	} else if parsed.Device.Protocol != "NVMe" && len(parsed.AtaSmartAttributes.Table) > 0 {
		report.SmartPassed = false
		report.SmartStatus = "FAILING"
	}

	// Temperature & Power on hours
	if parsed.Temperature.Current > 0 {
		report.TemperatureC = parsed.Temperature.Current
	}
	if parsed.PowerOnTime.Hours > 0 {
		report.PowerOnHours = parsed.PowerOnTime.Hours
	}

	// Extract critical ATA attributes
	for _, attr := range parsed.AtaSmartAttributes.Table {
		switch attr.ID {
		case 5: // Reallocated_Sector_Ct
			report.ReallocatedSectors = attr.Raw.Value
		case 197: // Current_Pending_Sector
			report.PendingSectors = attr.Raw.Value
		case 198: // Offline_Uncorrectable
			report.UncorrectableSectors = attr.Raw.Value
		case 194: // Temperature_Celsius
			if report.TemperatureC == 0 && attr.Raw.Value > 0 {
				report.TemperatureC = int(attr.Raw.Value & 0xFF)
			}
		case 9: // Power_On_Hours
			if report.PowerOnHours == 0 && attr.Raw.Value > 0 {
				report.PowerOnHours = attr.Raw.Value
			}
		}
	}

	// NVMe Telemetry fallback/override
	if parsed.Device.Protocol == "NVMe" || parsed.NvmeSmartHealth.Temperature > 0 {
		report.IsSSD = true
		if parsed.NvmeSmartHealth.CriticalWarning == 0 {
			report.SmartPassed = true
			report.SmartStatus = "PASSED"
		} else {
			report.SmartPassed = false
			report.SmartStatus = "FAILING"
		}
		if parsed.NvmeSmartHealth.Temperature > 0 {
			report.TemperatureC = parsed.NvmeSmartHealth.Temperature
		}
		if parsed.NvmeSmartHealth.PowerOnHours > 0 {
			report.PowerOnHours = parsed.NvmeSmartHealth.PowerOnHours
		}
		report.UncorrectableSectors = parsed.NvmeSmartHealth.MediaErrors
	}

	return report, nil
}

// resolveDiskDevicePath safely finds the dev path (/dev/disk/by-id/<disk> or /dev/<disk>).
func resolveDiskDevicePath(disk string) string {
	clean := strings.TrimSpace(disk)
	clean = strings.TrimPrefix(clean, "/dev/")
	clean = strings.TrimPrefix(clean, "disk/by-id/")

	// If disk already looks like by-id or ata/nvme identifier, try disk/by-id first
	byId := "/dev/disk/by-id/" + clean
	if _, err := os.Stat(byId); err == nil {
		return byId
	}
	return "/dev/" + clean
}

// ExecuteSmartRead queries SMART health via smartctl -a -j.
func ExecuteSmartRead(disk string) (*SmartReport, error) {
	cleanDisk := strings.TrimSpace(disk)
	cleanDisk = strings.TrimPrefix(cleanDisk, "/dev/")
	cleanDisk = strings.TrimPrefix(cleanDisk, "disk/by-id/")

	devPath := resolveDiskDevicePath(cleanDisk)

	smartctlBin := resolveExecutable("smartctl", "/usr/sbin/smartctl", "/sbin/smartctl", "/usr/bin/smartctl")

	// smartctl returns a bitmask exit code; non-zero doesn't strictly mean failure to output JSON.
	cmd := exec.Command(smartctlBin, "-a", "-j", devPath)
	out, err := cmd.CombinedOutput()

	if len(out) == 0 {
		// smartctl binary missing or device cannot be opened
		report := &SmartReport{
			Device:      cleanDisk,
			DevicePath:  devPath,
			SmartStatus: "UNAVAILABLE",
			SmartPassed: true, // Do not flag false positive if tool is missing
			Error:       fmt.Sprintf("smartctl execution failed: %v", err),
		}
		return report, nil
	}

	report, parseErr := ParseSmartctlOutput(cleanDisk, devPath, out)
	if parseErr != nil {
		report.Error = parseErr.Error()
	}
	return report, nil
}

var btrfsDeviceRegex = regexp.MustCompile(`\[(/dev/[a-zA-Z0-9_\-\/]+)\]`)

// CollectStorageSmartReports parses device paths from btrfs device stats output and queries SMART for each.
func CollectStorageSmartReports(statsOut string) []*SmartReport {
	matches := btrfsDeviceRegex.FindAllStringSubmatch(statsOut, -1)
	seen := make(map[string]bool)
	var reports []*SmartReport

	for _, m := range matches {
		if len(m) >= 2 {
			dev := m[1]
			if !seen[dev] {
				seen[dev] = true
				rep, _ := ExecuteSmartRead(dev)
				if rep != nil {
					reports = append(reports, rep)
				}
			}
		}
	}
	return reports
}
