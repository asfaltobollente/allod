package ring

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{10 * 1024 * 1024, "10.0 MB"},
		{1500 * 1024 * 1024, "1.46 GB"},
		{500 * 1024 * 1024 * 1024, "500 GB"},
	}

	for _, c := range cases {
		res := FormatBytes(c.bytes)
		if res != c.expected {
			t.Errorf("FormatBytes(%d) = %s, expected %s", c.bytes, res, c.expected)
		}
	}
}

func TestScanDatasetTree(t *testing.T) {
	tempDir := t.TempDir()

	// Create fake structure:
	// shares/davide/doc.txt
	// shares/public/Stampa 3D/part.stl
	// photos/upload/pic.jpg
	// media/config/conf.xml
	_ = os.MkdirAll(filepath.Join(tempDir, "shares", "davide"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "shares", "davide", "doc.txt"), []byte("davide-data"), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "shares", "public", "Stampa 3D"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "shares", "public", "Stampa 3D", "part.stl"), []byte("stl-data-3d"), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "photos", "upload"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "photos", "upload", "pic.jpg"), []byte("photo-data"), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "media", "config"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "media", "config", "conf.xml"), []byte("xml-config"), 0644)

	items := ScanDatasetTree(tempDir, nil)
	if len(items) == 0 {
		t.Fatalf("expected scanned dataset items, got empty")
	}

	foundUser := false
	foundPublic := false
	foundPhotos := false
	foundMedia := false

	for _, it := range items {
		if it.ID == "user-davide" {
			foundUser = true
			if it.SizeBytes <= 0 {
				t.Errorf("expected user-davide to have size > 0")
			}
		}
		if it.ID == "public-root" {
			foundPublic = true
			if len(it.Children) == 0 {
				t.Errorf("expected public-root to have children (Stampa 3D)")
			} else if it.Children[0].Name != "Stampa 3D" {
				t.Errorf("expected child Stampa 3D, got %s", it.Children[0].Name)
			}
		}
		if it.ID == "module-photos" {
			foundPhotos = true
		}
		if it.ID == "module-media-config" {
			foundMedia = true
		}
	}

	if !foundUser || !foundPublic || !foundPhotos || !foundMedia {
		t.Errorf("missing expected dataset categories: user=%v, pub=%v, photos=%v, media=%v", foundUser, foundPublic, foundPhotos, foundMedia)
	}
}

func TestCalculateForecast(t *testing.T) {
	tempDir := t.TempDir()

	// Create dummy files with different mod times
	now := time.Now()
	f1 := filepath.Join(tempDir, "file1.bin")
	f2 := filepath.Join(tempDir, "file2.bin")
	_ = os.WriteFile(f1, make([]byte, 10*1024*1024), 0644) // 10 MB
	_ = os.WriteFile(f2, make([]byte, 20*1024*1024), 0644) // 20 MB

	_ = os.Chtimes(f1, now.AddDate(-2, 0, 0), now.AddDate(-2, 0, 0))
	_ = os.Chtimes(f2, now, now)

	fc := CalculateForecast([]string{tempDir}, 500)
	if fc == nil {
		t.Fatalf("CalculateForecast returned nil")
	}

	if fc.CurrentSizeBytes != 30*1024*1024 {
		t.Errorf("expected 30 MB current size, got %d", fc.CurrentSizeBytes)
	}

	if fc.Forecast1YearBytes <= fc.CurrentSizeBytes {
		t.Errorf("expected 1-year forecast to be greater than current size")
	}

	if fc.Forecast5YearBytes <= fc.Forecast1YearBytes {
		t.Errorf("expected 5-year forecast to be greater than 1-year forecast")
	}

	if fc.QuotaStatus != "green" {
		t.Errorf("expected green quota status for 30 MB on 500 GB, got %s", fc.QuotaStatus)
	}

	if fc.RunwayMonths <= 12 {
		t.Errorf("expected generous runway > 12 months, got %d", fc.RunwayMonths)
	}
}

func TestCheckTelegramQuotaAlertIntervals(t *testing.T) {
	now := time.Now().Unix()

	// 1. Safe condition (Quota < 80%) -> No alert
	fcSafe := &ForecastReport{
		QuotaUsedPercent: 50.0,
		RunwayMonths:     36,
	}
	_, _, sent, err := CheckTelegramQuotaAlert(fcSafe, "dummy_token", "dummy_chat", 0, 0)
	if err != nil || sent != "" {
		t.Errorf("expected no alert for safe quota, got sent=%s, err=%v", sent, err)
	}

	// 2. Yellow condition (82% used, Runway 4 months)
	fcYellow := &ForecastReport{
		QuotaUsedPercent:     82.0,
		RunwayMonths:         4,
		CurrentSizeFormatted: "410 GB",
		RemoteQuotaFormatted: "500 GB",
	}
	// If lastYellowTime was 10 days ago (< 30 days) -> should NOT send (anti-spam)
	newY, _, sentY, _ := CheckTelegramQuotaAlert(fcYellow, "dummy_token", "dummy_chat", now-10*86400, 0)
	if sentY != "" {
		t.Errorf("expected yellow alert to be suppressed by monthly interval, got %s", sentY)
	}

	// 3. Red condition (92% used, Runway 1 month)
	fcRed := &ForecastReport{
		QuotaUsedPercent:     92.0,
		RunwayMonths:         1,
		CurrentSizeFormatted: "460 GB",
		RemoteQuotaFormatted: "500 GB",
	}
	// If lastRedTime was 2 days ago (< 7 days) -> should NOT send (anti-spam)
	_, newR, sentR, _ := CheckTelegramQuotaAlert(fcRed, "dummy_token", "dummy_chat", 0, now-2*86400)
	if sentR != "" {
		t.Errorf("expected red alert to be suppressed by weekly interval, got %s", sentR)
	}
	if newR != now-2*86400 {
		t.Errorf("expected newRedTime unchanged when suppressed")
	}
	_ = newY
}
