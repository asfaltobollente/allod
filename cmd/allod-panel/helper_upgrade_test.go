package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFileSHA256(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "file1.bin")
	f2 := filepath.Join(tempDir, "file2.bin")

	content1 := []byte("hello world binary content 12345")
	content2 := []byte("different binary content")

	if err := os.WriteFile(f1, content1, 0755); err != nil {
		t.Fatalf("failed to write f1: %v", err)
	}
	if err := os.WriteFile(f2, content2, 0755); err != nil {
		t.Fatalf("failed to write f2: %v", err)
	}

	h1, err1 := computeFileSHA256(f1)
	if err1 != nil {
		t.Fatalf("computeFileSHA256 f1 failed: %v", err1)
	}
	h2, err2 := computeFileSHA256(f2)
	if err2 != nil {
		t.Fatalf("computeFileSHA256 f2 failed: %v", err2)
	}

	if h1 == "" || h2 == "" {
		t.Fatalf("hash was empty")
	}
	if h1 == h2 {
		t.Fatalf("expected different hashes for different contents")
	}

	// Hash of same content must match
	f1Copy := filepath.Join(tempDir, "file1_copy.bin")
	_ = os.WriteFile(f1Copy, content1, 0755)
	h1Copy, _ := computeFileSHA256(f1Copy)
	if h1 != h1Copy {
		t.Fatalf("expected matching hashes for identical content, got %s and %s", h1, h1Copy)
	}
}
