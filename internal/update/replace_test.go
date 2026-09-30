package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceBinary(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "agrep")

	// Create initial dummy binary
	if err := os.WriteFile(targetPath, []byte("version 0.1.0"), 0755); err != nil {
		t.Fatalf("failed to create dummy binary: %v", err)
	}

	newContent := "#!/bin/sh\necho updated version 0.2.0"
	if err := ReplaceBinary(targetPath, strings.NewReader(newContent)); err != nil {
		t.Fatalf("ReplaceBinary failed: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read replaced binary: %v", err)
	}

	if string(got) != newContent {
		t.Errorf("replaced binary content mismatch: got %q, want %q", string(got), newContent)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat replaced binary: %v", err)
	}
	// Check executable permission
	if info.Mode()&0111 == 0 {
		t.Errorf("expected executable mode, got %v", info.Mode())
	}
}

func TestDetectInstallMethod(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/opt/homebrew/Cellar/agrep/0.1.0/bin/agrep", "homebrew"},
		{"/usr/local/Cellar/agrep/0.1.0/bin/agrep", "homebrew"},
		{"/home/linuxbrew/.linuxbrew/bin/agrep", "homebrew"},
		{"/usr/local/bin/agrep", "binary"},
		{"/usr/bin/agrep", "binary"},
	}

	for _, tc := range tests {
		got := DetectInstallMethod(tc.path)
		if got != tc.want {
			t.Errorf("DetectInstallMethod(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
