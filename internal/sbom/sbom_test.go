package sbom

import (
	"encoding/json"
	"testing"
)

func TestGenerateSBOM(t *testing.T) {
	doc, err := GenerateSBOM("../../modules")
	if err != nil {
		t.Fatalf("unexpected error generating SBOM: %v", err)
	}

	if doc.BOMFormat != "CycloneDX" {
		t.Errorf("expected BOMFormat CycloneDX, got %s", doc.BOMFormat)
	}

	jsonStr, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error formatting JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("generated invalid JSON: %v", err)
	}

	foundCore := false
	for _, comp := range doc.Components {
		if comp.Name == "allod-core" {
			foundCore = true
			break
		}
	}
	if !foundCore {
		t.Errorf("allod-core application component not found in SBOM")
	}
}
