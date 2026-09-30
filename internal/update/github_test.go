package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientLatestRelease(t *testing.T) {
	mockRelease := Release{
		TagName: "v0.2.0",
		Name:    "agrep 0.2.0",
		HTMLURL: "https://github.com/MohamedElashri/agrep/releases/tag/v0.2.0",
		Assets: []Asset{
			{
				Name:        "agrep_0.2.0_linux_amd64.tar.gz",
				DownloadURL: "http://example.com/agrep_0.2.0_linux_amd64.tar.gz",
			},
			{
				Name:        "checksums.txt",
				DownloadURL: "http://example.com/checksums.txt",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/MohamedElashri/agrep/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockRelease)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient("0.1.0")
	client.BaseAPIURL = server.URL
	client.BaseHTMLURL = server.URL

	rel, err := client.LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease failed: %v", err)
	}

	if rel.TagName != "v0.2.0" {
		t.Errorf("expected tag v0.2.0, got %s", rel.TagName)
	}

	asset, err := client.FindAsset(rel, "linux", "amd64")
	if err != nil {
		t.Fatalf("FindAsset failed: %v", err)
	}
	if asset.Name != "agrep_0.2.0_linux_amd64.tar.gz" {
		t.Errorf("unexpected asset name: %s", asset.Name)
	}

	chk, err := client.FindChecksums(rel)
	if err != nil {
		t.Fatalf("FindChecksums failed: %v", err)
	}
	if chk.Name != "checksums.txt" {
		t.Errorf("unexpected checksums name: %s", chk.Name)
	}
}

func TestClientRateLimitFallback(t *testing.T) {
	// Server returns 403 on API, but 302 on HTML /releases/latest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}
		if r.URL.Path == "/MohamedElashri/agrep/releases/latest" {
			w.Header().Set("Location", "/MohamedElashri/agrep/releases/tag/v0.3.0")
			w.WriteHeader(http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient("0.1.0")
	client.BaseAPIURL = server.URL
	client.BaseHTMLURL = server.URL

	rel, err := client.LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease fallback failed: %v", err)
	}

	if rel.TagName != "v0.3.0" {
		t.Errorf("expected tag v0.3.0 from fallback, got %s", rel.TagName)
	}

	// In fallback mode, assets are generated synthetically
	asset, err := client.FindAsset(rel, "linux", "arm64")
	if err != nil {
		t.Fatalf("FindAsset failed: %v", err)
	}
	if asset.Name != "agrep_0.3.0_linux_arm64.tar.gz" {
		t.Errorf("expected agrep_0.3.0_linux_arm64.tar.gz, got %s", asset.Name)
	}

	winAsset, err := client.FindAsset(rel, "windows", "amd64")
	if err != nil {
		t.Fatalf("FindAsset for windows failed: %v", err)
	}
	if winAsset.Name != "agrep_0.3.0_windows_amd64.zip" {
		t.Errorf("expected agrep_0.3.0_windows_amd64.zip, got %s", winAsset.Name)
	}
}
