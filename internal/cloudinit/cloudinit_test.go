package cloudinit

import (
	"strings"
	"testing"
)

func TestGenerateValidHostname(t *testing.T) {
	cfg := Config{Hostname: "allod-luca-01"}
	out, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hostname: allod-luca-01") {
		t.Errorf("expected hostname in output, got:\n%s", out)
	}
	if !strings.Contains(out, "apt-get install -y allod-core") {
		t.Errorf("expected package install in output, got:\n%s", out)
	}
}

func TestGenerateInvalidHostnameInjection(t *testing.T) {
	invalidHostnames := []string{
		"allod\n  runcmd:\n    - malicious",
		"-invalid-start",
		"invalid space",
		"host.with.dots",
		"",
	}

	for _, h := range invalidHostnames {
		cfg := Config{Hostname: h}
		_, err := Generate(cfg)
		if err == nil {
			t.Errorf("expected error for invalid hostname %q, got nil", h)
		}
	}
}
