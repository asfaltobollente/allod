package version

import (
	"testing"
)

func TestVersionGet(t *testing.T) {
	v := Get()
	if v == "" {
		t.Fatalf("expected non-empty version")
	}

	// Test overridden version
	original := Version
	defer func() { Version = original }()

	Version = "v0.1.0"
	if Get() != "v0.1.0" {
		t.Errorf("expected v0.1.0, got %s", Get())
	}
}
