package manifest

import "testing"

func TestIsValidModule(t *testing.T) {
	validModules := []string{"backup", "cloud", "media", "network", "odysseus", "photos", "shares", "storage", "watch"}
	for _, mod := range validModules {
		if !IsValidModule(mod) {
			t.Errorf("expected valid module %q to be recognized", mod)
		}
	}

	invalidModules := []string{"../x", "../../etc", "unknown", "", "root", "bin", "../", "/etc/passwd"}
	for _, mod := range invalidModules {
		if IsValidModule(mod) {
			t.Errorf("SECURITY: expected invalid module %q to be rejected", mod)
		}
	}
}
