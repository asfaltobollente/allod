package sbom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/allod-project/allod/internal/manifest"
)

type Component struct {
	Type        string `json:"type"` // "application", "library", "container"
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	PURL        string `json:"purl,omitempty"`
	License     string `json:"license,omitempty"`
}

type SBOMDocument struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	SerialNumber string     `json:"serialNumber"`
	Version     int         `json:"version"`
	Metadata    Metadata    `json:"metadata"`
	Components  []Component `json:"components"`
}

type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`
	Authors   []string  `json:"authors"`
	License   string    `json:"license"`
}

func GenerateSBOM(modulesDir string) (*SBOMDocument, error) {
	doc := &SBOMDocument{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: "urn:uuid:allod-core-sbom-v2.1",
		Version:      1,
		Metadata: Metadata{
			Timestamp: time.Now().UTC(),
			Tool:      fmtTool(),
			Authors:   []string{"Allod Project Team"},
			License:   "AGPL-3.0",
		},
		Components: make([]Component, 0),
	}

	// 1. Core Allod Application
	doc.Components = append(doc.Components, Component{
		Type:        "application",
		Name:        "allod-core",
		Version:     "2.1.0",
		Description: "Allod Personal Cloud & Federated Backup Orchestrator",
		PURL:        "pkg:golang/github.com/allod-project/allod@2.1.0",
		License:     "AGPL-3.0",
	})

	// 2. Go build dependencies
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range bi.Deps {
			doc.Components = append(doc.Components, Component{
				Type:    "library",
				Name:    dep.Path,
				Version: dep.Version,
				PURL:    "pkg:golang/" + dep.Path + "@" + dep.Version,
			})
		}
	}

	// 3. Container images from manifests
	if modulesDir == "" {
		modulesDir = "modules"
	}
	entries, err := os.ReadDir(modulesDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			mPath := filepath.Join(modulesDir, e.Name(), "module.yaml")
			m, err := manifest.LoadManifest(mPath)
			if err != nil {
				continue
			}
			for _, img := range m.Images {
				doc.Components = append(doc.Components, Component{
					Type:        "container",
					Name:        img.Ref,
					Version:     img.Tag,
					Description: "Container image for Allod module: " + m.ID,
					PURL:        "pkg:docker/" + img.Ref + "@" + img.Tag,
				})
			}
		}
	}

	return doc, nil
}

func (s *SBOMDocument) ToJSON() (string, error) {
	bytes, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func fmtTool() string {
	return "allod-sbom/" + runtime.Version()
}
