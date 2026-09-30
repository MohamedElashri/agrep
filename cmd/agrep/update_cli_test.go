package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

	"github.com/MohamedElashri/agrep/internal/update"
)

func createCLITestTarGz(t *testing.T, binaryContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name:     "agrep",
		Mode:     0755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write([]byte(binaryContent)); err != nil {
		t.Fatalf("failed to write tar body: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func TestRunHelpContainsUpdateFlags(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--help) exit code = %d; want 0", code)
	}

	help := stdout.String()
	if !strings.Contains(help, "--check-update") {
		t.Errorf("help text missing --check-update flag:\n%s", help)
	}
	if !strings.Contains(help, "--update") {
		t.Errorf("help text missing --update flag:\n%s", help)
	}
}

func TestRunCheckUpdateCLI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/MohamedElashri/agrep/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(update.Release{
				TagName: "v0.2.0",
				HTMLURL: "https://github.com/MohamedElashri/agrep/releases/tag/v0.2.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	orig := newUpdater
	defer func() { newUpdater = orig }()
	newUpdater = func(v string) *update.Updater {
		u := update.New("v0.1.0")
		u.Client.BaseAPIURL = server.URL
		u.Client.BaseHTMLURL = server.URL
		return u
	}

	var stdout, stderr strings.Builder
	code := run([]string{"--check-update"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--check-update) exit code = %d; want 0, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "A new version of agrep is available: v0.1.0 -> v0.2.0") {
		t.Errorf("unexpected check-update output:\n%s", out)
	}
	if !strings.Contains(out, "agrep --update") {
		t.Errorf("expected upgrade instruction in output:\n%s", out)
	}
}

func TestRunCheckUpdateUpToDateCLI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/MohamedElashri/agrep/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(update.Release{
				TagName: "v0.2.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	orig := newUpdater
	defer func() { newUpdater = orig }()
	newUpdater = func(v string) *update.Updater {
		u := update.New("v0.2.0")
		u.Client.BaseAPIURL = server.URL
		u.Client.BaseHTMLURL = server.URL
		return u
	}

	var stdout, stderr strings.Builder
	code := run([]string{"--check-update"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--check-update) exit code = %d; want 0, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "agrep is up to date (v0.2.0)") {
		t.Errorf("unexpected check-update output:\n%s", out)
	}
}

func TestRunUpdateCLI(t *testing.T) {
	newBinaryContent := "#!/bin/sh\necho agrep-v0.2.0"
	archiveData := createCLITestTarGz(t, newBinaryContent)
	h := sha256.Sum256(archiveData)
	archiveHash := hex.EncodeToString(h[:])
	checksumsContent := fmt.Sprintf("%s  agrep_0.2.0_linux_amd64.tar.gz\n", archiveHash)

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/MohamedElashri/agrep/releases/latest":
			rel := update.Release{
				TagName: "v0.2.0",
				Assets: []update.Asset{
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
	if err := os.WriteFile(targetExec, []byte("initial-binary"), 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	orig := newUpdater
	defer func() { newUpdater = orig }()
	newUpdater = func(v string) *update.Updater {
		u := update.New("v0.1.0")
		u.TargetExecutable = targetExec
		u.GOOS = "linux"
		u.GOARCH = "amd64"
		u.Client.BaseAPIURL = server.URL
		u.Client.BaseHTMLURL = server.URL
		return u
	}

	var stdout, stderr strings.Builder
	code := run([]string{"--update"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--update) exit code = %d; want 0, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Successfully updated agrep to v0.2.0!") {
		t.Errorf("unexpected update output:\n%s", out)
	}

	// Verify target was replaced
	got, err := os.ReadFile(targetExec)
	if err != nil {
		t.Fatalf("failed to read updated executable: %v", err)
	}
	if string(got) != newBinaryContent {
		t.Errorf("binary content mismatch: got %q, want %q", string(got), newBinaryContent)
	}
}

func TestRunSelfUpdateAliasCLI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/MohamedElashri/agrep/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(update.Release{
				TagName: "v0.2.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	orig := newUpdater
	defer func() { newUpdater = orig }()
	newUpdater = func(v string) *update.Updater {
		u := update.New("v0.2.0")
		u.Client.BaseAPIURL = server.URL
		u.Client.BaseHTMLURL = server.URL
		return u
	}

	var stdout, stderr strings.Builder
	code := run([]string{"self-update", "check"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(self-update check) exit code = %d; want 0, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "agrep is up to date (v0.2.0)") {
		t.Errorf("unexpected check output from self-update alias:\n%s", out)
	}
}
