package helper

import (
	"testing"
)

func TestParseSmartctlOutput_SATA_Healthy(t *testing.T) {
	mockJSON := []byte(`{
		"json_format_version": [1, 0],
		"device": {
			"name": "/dev/sda",
			"type": "ata",
			"protocol": "ATA"
		},
		"model_family": "Western Digital Red Plus",
		"model_name": "WDC WD40EFZX-68AWUN0",
		"serial_number": "WD-WCC7K1234567",
		"user_capacity": {
			"bytes": 4000787030016
		},
		"rotation_rate": 5400,
		"smart_status": {
			"passed": true
		},
		"temperature": {
			"current": 33
		},
		"power_on_time": {
			"hours": 12450
		},
		"power_cycle_count": 42,
		"ata_smart_attributes": {
			"table": [
				{"id": 5, "name": "Reallocated_Sector_Ct", "raw": {"value": 0, "string": "0"}},
				{"id": 197, "name": "Current_Pending_Sector", "raw": {"value": 0, "string": "0"}},
				{"id": 198, "name": "Offline_Uncorrectable", "raw": {"value": 0, "string": "0"}}
			]
		}
	}`)

	report, err := ParseSmartctlOutput("sda", "/dev/sda", mockJSON)
	if err != nil {
		t.Fatalf("unexpected error parsing healthy SATA JSON: %v", err)
	}

	if !report.SmartPassed {
		t.Errorf("expected SmartPassed=true, got %v", report.SmartPassed)
	}
	if report.SmartStatus != "PASSED" {
		t.Errorf("expected SmartStatus=PASSED, got %s", report.SmartStatus)
	}
	if report.ModelName != "WDC WD40EFZX-68AWUN0" {
		t.Errorf("unexpected model name: %s", report.ModelName)
	}
	if report.TemperatureC != 33 {
		t.Errorf("expected temperature 33, got %d", report.TemperatureC)
	}
	if report.PowerOnHours != 12450 {
		t.Errorf("expected 12450 power on hours, got %d", report.PowerOnHours)
	}
	if report.ReallocatedSectors != 0 {
		t.Errorf("expected 0 reallocated sectors, got %d", report.ReallocatedSectors)
	}
	if report.IsSSD {
		t.Errorf("expected IsSSD=false for 5400 RPM drive, got true")
	}
	if report.UserCapacity != "4.0 TB" {
		t.Errorf("expected UserCapacity='4.0 TB', got '%s'", report.UserCapacity)
	}
}

func TestParseSmartctlOutput_NVMe_Healthy(t *testing.T) {
	mockJSON := []byte(`{
		"json_format_version": [1, 0],
		"device": {
			"name": "/dev/nvme0n1",
			"type": "nvme",
			"protocol": "NVMe"
		},
		"model_name": "Samsung SSD 980 PRO 1TB",
		"serial_number": "S5GXNF0R123456",
		"user_capacity": {
			"bytes": 1000204886016
		},
		"rotation_rate": 0,
		"nvme_smart_health_information_log": {
			"critical_warning": 0,
			"temperature": 41,
			"available_spare": 100,
			"percentage_used": 4,
			"power_on_hours": 3200,
			"media_errors": 0
		}
	}`)

	report, err := ParseSmartctlOutput("nvme0n1", "/dev/nvme0n1", mockJSON)
	if err != nil {
		t.Fatalf("unexpected error parsing NVMe JSON: %v", err)
	}

	if !report.SmartPassed {
		t.Errorf("expected SmartPassed=true, got %v", report.SmartPassed)
	}
	if report.SmartStatus != "PASSED" {
		t.Errorf("expected SmartStatus=PASSED, got %s", report.SmartStatus)
	}
	if !report.IsSSD {
		t.Errorf("expected IsSSD=true for NVMe, got false")
	}
	if report.TemperatureC != 41 {
		t.Errorf("expected temperature 41, got %d", report.TemperatureC)
	}
	if report.PowerOnHours != 3200 {
		t.Errorf("expected power on hours 3200, got %d", report.PowerOnHours)
	}
	if report.UncorrectableSectors != 0 {
		t.Errorf("expected media errors 0, got %d", report.UncorrectableSectors)
	}
}

func TestParseSmartctlOutput_FailingDrive(t *testing.T) {
	mockJSON := []byte(`{
		"json_format_version": [1, 0],
		"device": {
			"name": "/dev/sdb",
			"type": "ata",
			"protocol": "ATA"
		},
		"model_name": "Seagate IronWolf 4TB",
		"smart_status": {
			"passed": false
		},
		"temperature": {
			"current": 49
		},
		"ata_smart_attributes": {
			"table": [
				{"id": 5, "name": "Reallocated_Sector_Ct", "raw": {"value": 142, "string": "142"}},
				{"id": 197, "name": "Current_Pending_Sector", "raw": {"value": 18, "string": "18"}},
				{"id": 198, "name": "Offline_Uncorrectable", "raw": {"value": 5, "string": "5"}}
			]
		}
	}`)

	report, err := ParseSmartctlOutput("sdb", "/dev/sdb", mockJSON)
	if err != nil {
		t.Fatalf("unexpected error parsing failing SATA JSON: %v", err)
	}

	if report.SmartPassed {
		t.Errorf("expected SmartPassed=false for failing disk, got true")
	}
	if report.SmartStatus != "FAILING" {
		t.Errorf("expected SmartStatus=FAILING, got %s", report.SmartStatus)
	}
	if report.ReallocatedSectors != 142 {
		t.Errorf("expected 142 reallocated sectors, got %d", report.ReallocatedSectors)
	}
	if report.PendingSectors != 18 {
		t.Errorf("expected 18 pending sectors, got %d", report.PendingSectors)
	}
	if report.UncorrectableSectors != 5 {
		t.Errorf("expected 5 uncorrectable sectors, got %d", report.UncorrectableSectors)
	}
}

func TestParseSmartctlOutput_Empty(t *testing.T) {
	report, err := ParseSmartctlOutput("sdc", "/dev/sdc", []byte{})
	if err == nil {
		t.Error("expected error on empty bytes, got nil")
	}
	if report.SmartStatus != "UNKNOWN" {
		t.Errorf("expected UNKNOWN status on empty input, got %s", report.SmartStatus)
	}
}

func TestCollectStorageSmartReports_Empty(t *testing.T) {
	reports := CollectStorageSmartReports("no device patterns here")
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for empty stats, got %d", len(reports))
	}
}
