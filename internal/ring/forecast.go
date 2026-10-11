package ring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DatasetItem represents an entry in the hierarchical backup dataset selector.
type DatasetItem struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Path          string         `json:"path"`
	Category      string         `json:"category"` // "user", "public", "module"
	Level         int            `json:"level"`    // 0 = root/main, 1 = subfolder level 1, 2 = subfolder level 2
	SizeBytes     int64          `json:"size_bytes"`
	SizeFormatted string         `json:"size_formatted"`
	Selected      bool           `json:"selected"`
	Children      []*DatasetItem `json:"children,omitempty"`
	Description   string         `json:"description,omitempty"`
}

// ForecastPoint represents cumulative storage at a historical year.
type ForecastPoint struct {
	Year          int    `json:"year"`
	SizeBytes     int64  `json:"size_bytes"`
	SizeFormatted string `json:"size_formatted"`
}

// ForecastReport provides historical growth curve, future 1Y & 5Y projections,
// and quota runway for remote peer backups.
type ForecastReport struct {
	CurrentSizeBytes        int64           `json:"current_size_bytes"`
	CurrentSizeFormatted    string          `json:"current_size_formatted"`
	History                 []ForecastPoint `json:"history"`
	AnnualGrowthBytes       int64           `json:"annual_growth_bytes"`
	AnnualGrowthFormatted   string          `json:"annual_growth_formatted"`
	Forecast1YearBytes      int64           `json:"forecast_1year_bytes"`
	Forecast1YearFormatted  string          `json:"forecast_1year_formatted"`
	Forecast5YearBytes      int64           `json:"forecast_5year_bytes"`
	Forecast5YearFormatted  string          `json:"forecast_5year_formatted"`
	RemoteQuotaGB           int             `json:"remote_quota_gb"`
	RemoteQuotaFormatted    string          `json:"remote_quota_formatted"`
	QuotaUsedPercent        float64         `json:"quota_used_percent"`
	RunwayMonths            int             `json:"runway_months"`
	RunwayFormatted         string          `json:"runway_formatted"`
	QuotaStatus             string          `json:"quota_status"` // "green", "yellow", "red"
	TelegramYellowThreshold int             `json:"telegram_yellow_threshold"`
	TelegramRedThreshold    int             `json:"telegram_red_threshold"`
	SelectedPathsCount      int             `json:"selected_paths_count"`
}

// FormatBytes converts raw bytes into a human-readable string (B, KB, MB, GB, TB).
func FormatBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	val := float64(b) / float64(div)
	if val >= 100 {
		return fmt.Sprintf("%.0f %s", val, units[exp])
	} else if val >= 10 {
		return fmt.Sprintf("%.1f %s", val, units[exp])
	}
	return fmt.Sprintf("%.2f %s", val, units[exp])
}

// FastDirSize computes the total size in bytes of a directory recursively.
func FastDirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, errI := d.Info(); errI == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// ScanDatasetTree constructs the hierarchical selector:
// 1. User shares in <storageRoot>/shares/<user>
// 2. Public folder in <storageRoot>/shares/public (with 2-level subfolder inspection)
// 3. Application modules (photos, cloud, odysseus, media/config)
func ScanDatasetTree(storageRoot string, selectedPaths map[string]bool) []*DatasetItem {
	var items []*DatasetItem

	sharesDir := filepath.Join(storageRoot, "shares")

	// 1. User Personal Shares (e.g. shares/davide, shares/mamma)
	if entries, err := os.ReadDir(sharesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "public" || name == "photos" || name == "media" || name == "lost+found" || strings.HasPrefix(name, ".") {
				continue
			}
			uPath := filepath.Join(sharesDir, name)
			uSize := FastDirSize(uPath)
			selected := selectedPaths[uPath]
			if len(selectedPaths) == 0 {
				selected = true // Default select personal user folders
			}

			items = append(items, &DatasetItem{
				ID:            "user-" + name,
				Name:          "Cartella Personale " + name,
				Path:          uPath,
				Category:      "user",
				Level:         0,
				SizeBytes:     uSize,
				SizeFormatted: FormatBytes(uSize),
				Selected:      selected,
				Description:   fmt.Sprintf("Dati personali e documenti SMB per l'utente %s", name),
			})
		}
	}

	// 2. Public Folder (shares/public) with 2-level deep subfolders
	pubDir := filepath.Join(sharesDir, "public")
	if _, err := os.Stat(pubDir); err == nil {
		pubSize := FastDirSize(pubDir)
		pubItem := &DatasetItem{
			ID:            "public-root",
			Name:          "Cartella Pubblica (Condivisione SMB)",
			Path:          pubDir,
			Category:      "public",
			Level:         0,
			SizeBytes:     pubSize,
			SizeFormatted: FormatBytes(pubSize),
			Selected:      selectedPaths[pubDir],
			Description:   "Archivio pubblico condiviso di rete (es. progetti 3D, documenti comuni)",
		}

		// Level 1 Subfolders inside public/
		if l1Entries, err1 := os.ReadDir(pubDir); err1 == nil {
			for _, e1 := range l1Entries {
				if !e1.IsDir() || strings.HasPrefix(e1.Name(), ".") {
					continue
				}
				l1Path := filepath.Join(pubDir, e1.Name())
				l1Size := FastDirSize(l1Path)
				l1Selected := selectedPaths[l1Path] || pubItem.Selected

				l1Item := &DatasetItem{
					ID:            "public-" + e1.Name(),
					Name:          e1.Name(),
					Path:          l1Path,
					Category:      "public",
					Level:         1,
					SizeBytes:     l1Size,
					SizeFormatted: FormatBytes(l1Size),
					Selected:      l1Selected,
				}

				// Level 2 Subfolders
				if l2Entries, err2 := os.ReadDir(l1Path); err2 == nil {
					for _, e2 := range l2Entries {
						if !e2.IsDir() || strings.HasPrefix(e2.Name(), ".") {
							continue
						}
						l2Path := filepath.Join(l1Path, e2.Name())
						l2Size := FastDirSize(l2Path)
						l2Selected := selectedPaths[l2Path] || l1Selected

						l2Item := &DatasetItem{
							ID:            fmt.Sprintf("public-%s-%s", e1.Name(), e2.Name()),
							Name:          e2.Name(),
							Path:          l2Path,
							Category:      "public",
							Level:         2,
							SizeBytes:     l2Size,
							SizeFormatted: FormatBytes(l2Size),
							Selected:      l2Selected,
						}
						l1Item.Children = append(l1Item.Children, l2Item)
					}
				}

				pubItem.Children = append(pubItem.Children, l1Item)
			}
		}

		items = append(items, pubItem)
	}

	// 3. Application Modules
	// 3a. Photos (Immich) - Originals and DB
	photosPath := filepath.Join(storageRoot, "photos", "upload")
	if _, err := os.Stat(photosPath); err == nil {
		pSize := FastDirSize(photosPath)
		pSel := selectedPaths[photosPath]
		if len(selectedPaths) == 0 {
			pSel = true // Default select photos
		}
		items = append(items, &DatasetItem{
			ID:            "module-photos",
			Name:          "Immich Photos (Foto Originali & Database)",
			Path:          photosPath,
			Category:      "module",
			Level:         0,
			SizeBytes:     pSize,
			SizeFormatted: FormatBytes(pSize),
			Selected:      pSel,
			Description:   "Foto e video originali + database PostgreSQL (esclude miniature e cache)",
		})
	}

	// 3b. Cloud (Nextcloud) - Data and DB
	cloudPath := filepath.Join(storageRoot, "cloud", "data")
	if _, err := os.Stat(cloudPath); err == nil {
		cSize := FastDirSize(cloudPath)
		cSel := selectedPaths[cloudPath]
		if len(selectedPaths) == 0 {
			cSel = true
		}
		items = append(items, &DatasetItem{
			ID:            "module-cloud",
			Name:          "Nextcloud Hub (File Utenti & Database)",
			Path:          cloudPath,
			Category:      "module",
			Level:         0,
			SizeBytes:     cSize,
			SizeFormatted: FormatBytes(cSize),
			Selected:      cSel,
			Description:   "Documenti personali del cloud e database relazionale",
		})
	}

	// 3c. Odysseus (AI Workspace) - ChromaDB, secrets, logs, and app.db
	odysseusPath := filepath.Join(storageRoot, "odysseus")
	if _, err := os.Stat(odysseusPath); err == nil {
		oSize := FastDirSize(odysseusPath)
		oSel := selectedPaths[odysseusPath] || selectedPaths[filepath.Join(odysseusPath, "data")]
		items = append(items, &DatasetItem{
			ID:            "module-odysseus",
			Name:          "Odysseus AI Workspace (Memoria & Chat)",
			Path:          odysseusPath,
			Category:      "module",
			Level:         0,
			SizeBytes:     oSize,
			SizeFormatted: FormatBytes(oSize),
			Selected:      oSel,
			Description:   "Intero workspace: memoria vettoriale ChromaDB, storico conversazioni, database e configurazione",
		})
	}

	// 3d. Media (Jellyfin) - Config and library metadata ONLY
	mediaConfigPath := filepath.Join(storageRoot, "media", "config")
	if _, err := os.Stat(mediaConfigPath); err == nil {
		mSize := FastDirSize(mediaConfigPath)
		mSel := selectedPaths[mediaConfigPath]
		if len(selectedPaths) == 0 {
			mSel = true // Default select light config
		}
		items = append(items, &DatasetItem{
			ID:            "module-media-config",
			Name:          "Jellyfin (Solo Configurazione Libreria)",
			Path:          mediaConfigPath,
			Category:      "module",
			Level:         0,
			SizeBytes:     mSize,
			SizeFormatted: FormatBytes(mSize),
			Selected:      mSel,
			Description:   "Configurazione e metadati della libreria (i video non vengono duplicati)",
		})
	}

	return items
}

// CalculateForecast performs historical timestamp analysis on files across selectedPaths
// and generates linear/weighted forecast projections for +1 Year, +5 Years, and Runway.
func CalculateForecast(selectedPaths []string, remoteQuotaGB int) *ForecastReport {
	if remoteQuotaGB <= 0 {
		remoteQuotaGB = 500 // Default sensible peer quota
	}

	currentYear := time.Now().Year()
	minYear := currentYear - 5
	yearBuckets := make(map[int]int64)

	var totalBytes int64

	// Traverse selected directories to extract file sizes and modification timestamps (mtime)
	for _, p := range selectedPaths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, errI := d.Info()
			if errI != nil {
				return nil
			}
			size := info.Size()
			totalBytes += size

			y := info.ModTime().Year()
			if y < minYear {
				y = minYear
			} else if y > currentYear {
				y = currentYear
			}
			yearBuckets[y] += size
			return nil
		})
	}

	// Build chronological history points (cumulative sum over years)
	var history []ForecastPoint
	var cumSum int64
	for y := minYear; y <= currentYear; y++ {
		cumSum += yearBuckets[y]
		history = append(history, ForecastPoint{
			Year:          y,
			SizeBytes:     cumSum,
			SizeFormatted: FormatBytes(cumSum),
		})
	}

	// Calculate historical growth rate
	// Look at growth over the last 3 years or total span
	spanYears := currentYear - minYear
	if spanYears <= 0 {
		spanYears = 1
	}

	var annualGrowth int64
	recentGrowth := totalBytes - history[0].SizeBytes
	if recentGrowth > 0 && spanYears > 0 {
		annualGrowth = recentGrowth / int64(spanYears)
	}

	// If freshly initialized or small sample, estimate sensible minimum (e.g. 20% of current or 5 GB)
	if annualGrowth <= 0 && totalBytes > 0 {
		annualGrowth = totalBytes / 4
		if annualGrowth < 5*1024*1024*1024 {
			annualGrowth = 5 * 1024 * 1024 * 1024
		}
	} else if annualGrowth < 2*1024*1024*1024 && totalBytes > 5*1024*1024*1024 {
		annualGrowth = 2 * 1024 * 1024 * 1024
	}

	// Projections
	forecast1Y := totalBytes + annualGrowth
	// 5-year forecast includes a slight acceleration (+10%) for growing camera/model resolutions
	forecast5Y := totalBytes + int64(float64(annualGrowth*5)*1.10)

	// Quota calculations
	quotaBytes := int64(remoteQuotaGB) * 1024 * 1024 * 1024
	var quotaUsedPct float64
	if quotaBytes > 0 {
		quotaUsedPct = (float64(totalBytes) / float64(quotaBytes)) * 100.0
	}

	// Runway calculation (months until quota exhaustion)
	runwayMonths := 120 // Default >10 years
	if quotaBytes > totalBytes && annualGrowth > 0 {
		remBytes := quotaBytes - totalBytes
		runwayMonths = int((float64(remBytes) / float64(annualGrowth)) * 12.0)
	} else if totalBytes >= quotaBytes {
		runwayMonths = 0
	}

	var runwayStr string
	if runwayMonths <= 0 {
		runwayStr = "Quota esaurita (0 mesi)"
	} else if runwayMonths < 12 {
		runwayStr = fmt.Sprintf("~%d Mesi", runwayMonths)
	} else {
		years := runwayMonths / 12
		remM := runwayMonths % 12
		if remM == 0 {
			runwayStr = fmt.Sprintf("~%d Anni", years)
		} else {
			runwayStr = fmt.Sprintf("~%d Anni e %d Mesi", years, remM)
		}
	}

	// Status determination
	status := "green"
	if quotaUsedPct >= 90.0 || (runwayMonths >= 0 && runwayMonths <= 1) {
		status = "red"
	} else if quotaUsedPct >= 80.0 || (runwayMonths >= 0 && runwayMonths <= 6) {
		status = "yellow"
	}

	return &ForecastReport{
		CurrentSizeBytes:        totalBytes,
		CurrentSizeFormatted:    FormatBytes(totalBytes),
		History:                 history,
		AnnualGrowthBytes:       annualGrowth,
		AnnualGrowthFormatted:   FormatBytes(annualGrowth) + " / anno",
		Forecast1YearBytes:      forecast1Y,
		Forecast1YearFormatted:  FormatBytes(forecast1Y),
		Forecast5YearBytes:      forecast5Y,
		Forecast5YearFormatted:  FormatBytes(forecast5Y),
		RemoteQuotaGB:           remoteQuotaGB,
		RemoteQuotaFormatted:    fmt.Sprintf("%d GB", remoteQuotaGB),
		QuotaUsedPercent:        quotaUsedPct,
		RunwayMonths:            runwayMonths,
		RunwayFormatted:         runwayStr,
		QuotaStatus:             status,
		TelegramYellowThreshold: 80,
		TelegramRedThreshold:    90,
		SelectedPathsCount:      len(selectedPaths),
	}
}

// CheckTelegramQuotaAlert verifies if usage or runway triggers a Yellow (monthly) or Red (weekly) alert,
// sends the message if due, and returns updated alert timestamps.
func CheckTelegramQuotaAlert(fc *ForecastReport, botToken, chatID string, lastYellowTime, lastRedTime int64) (newYellowTime int64, newRedTime int64, alertSent string, err error) {
	newYellowTime = lastYellowTime
	newRedTime = lastRedTime

	if botToken == "" || chatID == "" || fc == nil {
		return newYellowTime, newRedTime, "", nil
	}

	now := time.Now().Unix()

	// 1. Red Alert: Quota >= 90% OR runway <= 1 month (Weekly alert)
	isRed := fc.QuotaUsedPercent >= 90.0 || (fc.RunwayMonths >= 0 && fc.RunwayMonths <= 1)
	if isRed {
		const weeklyInterval = 7 * 24 * 3600
		if (now - lastRedTime) >= weeklyInterval {
			msg := fmt.Sprintf(`🔴 <b>[Allod Ring] EMERGENZA: Quota Backup Remoto Quasi Esaurita!</b>

⚠️ La cassaforte remota è quasi satura. Il prossimo backup rischia di fallire:

🚨 <b>Stato Attuale:</b> %s / %s (%.1f%%)
⏳ <b>Autonomia residua:</b> %s
📈 <b>Crescita stimata:</b> %s

🛠️ <b>Azioni consigliate:</b>
1. Apri la scheda <i>Ring Federation</i> sul pannello di Allod.
2. Aumenta la quota concordata con l'amico oppure deseleziona cartelle secondarie.`,
				fc.CurrentSizeFormatted,
				fc.RemoteQuotaFormatted,
				fc.QuotaUsedPercent,
				fc.RunwayFormatted,
				fc.AnnualGrowthFormatted,
			)

			if sErr := sendTelegramRaw(botToken, chatID, msg); sErr != nil {
				return newYellowTime, newRedTime, "", sErr
			}
			return newYellowTime, now, "red", nil
		}
		return newYellowTime, newRedTime, "", nil
	}

	// 2. Yellow Alert: Quota >= 80% OR runway <= 6 months (Monthly alert)
	isYellow := fc.QuotaUsedPercent >= 80.0 || (fc.RunwayMonths >= 0 && fc.RunwayMonths <= 6)
	if isYellow {
		const monthlyInterval = 30 * 24 * 3600
		if (now - lastYellowTime) >= monthlyInterval {
			msg := fmt.Sprintf(`🟡 <b>[Allod Ring] Avviso: Quota Backup Remoto in Avvicinamento</b>

Lo spazio occupato sul server del tuo amico si sta avvicinando al limite:

📊 <b>Stato Attuale:</b> %s / %s (%.1f%%)
⏳ <b>Autonomia residua:</b> %s
📈 <b>Crescita media:</b> %s

💡 <i>Consiglio:</i> Puoi concordare un aumento di quota con il tuo amico o alleggerire i dataset selezionati nel pannello Allod.`,
				fc.CurrentSizeFormatted,
				fc.RemoteQuotaFormatted,
				fc.QuotaUsedPercent,
				fc.RunwayFormatted,
				fc.AnnualGrowthFormatted,
			)

			if sErr := sendTelegramRaw(botToken, chatID, msg); sErr != nil {
				return newYellowTime, newRedTime, "", sErr
			}
			return now, newRedTime, "yellow", nil
		}
	}

	return newYellowTime, newRedTime, "", nil
}

// SendTelegramMessage sends an HTML-formatted message to a Telegram chat.
func SendTelegramMessage(botToken, chatID, htmlMsg string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       htmlMsg,
		"parse_mode": "HTML",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func sendTelegramRaw(botToken, chatID, htmlMsg string) error {
	return SendTelegramMessage(botToken, chatID, htmlMsg)
}
