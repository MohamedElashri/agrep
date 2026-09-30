package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdaterEndToEnd(t *testing.T) {
	// Prepare new binary and archive
	newBinaryContent := "#!/bin/sh\necho updated-agrep-binary-0.2.0"
	archiveData := createTestTarGz(t, map[string]string{
		"agrep":     newBinaryContent,
		"README.md": "documentation",
	})

	archiveHashBytes := sha256.Sum256(archiveData)
	archiveHash := hex.EncodeToString(archiveHashBytes[:])

	checksumsContent := fmt.Sprintf("%s  agrep_0.2.0_linux_amd64.tar.gz\n", archiveHash)

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/MohamedElashri/agrep/releases/latest":
			rel := Release{
				TagName: "v0.2.0",
				Name:    "v0.2.0",
				HTMLURL: serverURL + "/releases/tag/v0.2.0",
				Assets: []Asset{
					{
						Name:        "agrep_0.2.0_linux_amd64.tar.gz",
						DownloadURL: serverURL + "/download/agrep_0.2.0_linux_amd64.tar.gz",
					},
					{
						Name:        "checksums.txt",
						DownloadURL: serverURL + "/download/checksums.txt",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(checksumsContent))
		case "/download/agrep_0.2.0_linux_amd64.tar.gz":
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	// Create initial target executable
	tmpDir := t.TempDir()
	targetExec := filepath.Join(tmpDir, "agrep")
	if err := os.WriteFile(targetExec, []byte("initial-binary-v0.1.0"), 0755); err != nil {
		t.Fatalf("failed to create initial target: %v", err)
	}

	updater := New("v0.1.0")
	updater.TargetExecutable = targetExec
	updater.GOOS = "linux"
	updater.GOARCH = "amd64"
	updater.Client.BaseAPIURL = server.URL
	updater.Client.BaseHTMLURL = server.URL

	// 1. Test Check
	checkRes, err := updater.Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !checkRes.UpdateAvailable {
		t.Errorf("expected UpdateAvailable=true, got false")
	}
	if checkRes.LatestVersion != "v0.2.0" {
		t.Errorf("expected LatestVersion=v0.2.0, got %s", checkRes.LatestVersion)
	}

	// 2. Test Update
	var statusBuf bytes.Buffer
	updateRes, err := updater.Update(context.Background(), UpdateOptions{
		StatusWriter: &statusBuf,
	})
	if err != nil {
		t.Fatalf("Update failed: %v\nStatus output:\n%s", err, statusBuf.String())
	}

	if updateRes.AlreadyUpToDate {
		t.Errorf("expected AlreadyUpToDate=false")
	}
	if updateRes.NewVersion != "v0.2.0" {
		t.Errorf("expected NewVersion=v0.2.0, got %s", updateRes.NewVersion)
	}

	// Verify target executable has new content
	replacedContent, err := os.ReadFile(targetExec)
	if err != nil {
		t.Fatalf("failed to read replaced executable: %v", err)
	}
	if string(replacedContent) != newBinaryContent {
		t.Errorf("target executable content mismatch: got %q, want %q", string(replacedContent), newBinaryContent)
	}

	// Verify status messages
	statusStr := statusBuf.String()
	if !strings.Contains(statusStr, "Verifying SHA-256 checksum... OK") {
		t.Errorf("expected checksum OK in status output: %s", statusStr)
	}
	if !strings.Contains(statusStr, "Successfully updated agrep to v0.2.0!") {
		t.Errorf("expected success message in status output: %s", statusStr)
	}

	// 3. Test Update when already up to date
	updater.CurrentVersion = "v0.2.0"
	var upToDateBuf bytes.Buffer
	upToDateRes, err := updater.Update(context.Background(), UpdateOptions{
		StatusWriter: &upToDateBuf,
	})
	if err != nil {
		t.Fatalf("Update when up-to-date failed: %v", err)
	}
	if !upToDateRes.AlreadyUpToDate {
		t.Errorf("expected AlreadyUpToDate=true")
	}
	if !strings.Contains(upToDateBuf.String(), "already up to date") {
		t.Errorf("expected 'already up to date' in status output: %s", upToDateBuf.String())
	}
}

func TestUpdaterChecksumMismatch(t *testing.T) {
	newBinaryContent := "#!/bin/sh\necho updated-agrep-binary-0.2.0"
	archiveData := createTestTarGz(t, map[string]string{
		"agrep": newBinaryContent,
	})

	wrongChecksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	checksumsContent := fmt.Sprintf("%s  agrep_0.2.0_linux_amd64.tar.gz\n", wrongChecksum)

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/MohamedElashri/agrep/releases/latest":
			rel := Release{
				TagName: "v0.2.0",
				Assets: []Asset{
					{
						Name:        "agrep_0.2.0_linux_amd64.tar.gz",
						DownloadURL: serverURL + "/download/agrep_0.2.0_linux_amd64.tar.gz",
					},
					{
						Name:        "checksums.txt",
						DownloadURL: serverURL + "/download/checksums.txt",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(checksumsContent))
		case "/download/agrep_0.2.0_linux_amd64.tar.gz":
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	tmpDir := t.TempDir()
	targetExec := filepath.Join(tmpDir, "agrep")
	originalContent := "original-binary-v0.1.0"
	if err := os.WriteFile(targetExec, []byte(originalContent), 0755); err != nil {
		t.Fatalf("failed to create initial target: %v", err)
	}

	updater := New("v0.1.0")
	updater.TargetExecutable = targetExec
	updater.GOOS = "linux"
	updater.GOARCH = "amd64"
	updater.Client.BaseAPIURL = server.URL
	updater.Client.BaseHTMLURL = server.URL

	var statusBuf bytes.Buffer
	_, err := updater.Update(context.Background(), UpdateOptions{
		StatusWriter: &statusBuf,
	})
	if err == nil {
		t.Fatal("expected update to fail on checksum mismatch, but got nil error")
	}

	if !strings.Contains(err.Error(), "integrity check failed") {
		t.Errorf("expected integrity check failed error, got: %v", err)
	}

	// Ensure target binary was NOT modified
	gotContent, err := os.ReadFile(targetExec)
	if err != nil {
		t.Fatalf("failed to read target: %v", err)
	}
	if string(gotContent) != originalContent {
		t.Errorf("target was modified despite checksum failure! got %q, want %q", string(gotContent), originalContent)
	}
}
