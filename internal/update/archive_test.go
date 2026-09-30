package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func createTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar body: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create file in zip: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

func TestExtractBinaryTarGz(t *testing.T) {
	binaryContent := "#!/bin/sh\necho agrep binary"
	archiveData := createTestTarGz(t, map[string]string{
		"agrep":     binaryContent,
		"README.md": "readme documentation",
		"LICENSE":   "MIT license",
	})

	var out bytes.Buffer
	err := ExtractBinary(bytes.NewReader(archiveData), "agrep_0.1.0_linux_amd64.tar.gz", "agrep", &out)
	if err != nil {
		t.Fatalf("ExtractBinary failed: %v", err)
	}

	if out.String() != binaryContent {
		t.Fatalf("extracted binary content mismatch: got %q, want %q", out.String(), binaryContent)
	}
}

func TestExtractBinaryZip(t *testing.T) {
	binaryContent := "#!/bin/sh\necho agrep binary"
	archiveData := createTestZip(t, map[string]string{
		"agrep.exe": binaryContent,
		"README.md": "readme",
	})

	var out bytes.Buffer
	err := ExtractBinary(bytes.NewReader(archiveData), "agrep_0.1.0_windows_amd64.zip", "agrep", &out)
	if err != nil {
		t.Fatalf("ExtractBinary from zip failed: %v", err)
	}

	if out.String() != binaryContent {
		t.Fatalf("extracted binary content mismatch: got %q, want %q", out.String(), binaryContent)
	}
}

func TestExtractBinaryMissing(t *testing.T) {
	archiveData := createTestTarGz(t, map[string]string{
		"README.md": "readme",
	})

	var out bytes.Buffer
	err := ExtractBinary(bytes.NewReader(archiveData), "agrep.tar.gz", "agrep", &out)
	if err == nil {
		t.Fatal("expected error when binary missing, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestExtractBinaryUnsafePath(t *testing.T) {
	archiveData := createTestTarGz(t, map[string]string{
		"../../agrep": "malicious content",
	})

	var out bytes.Buffer
	err := ExtractBinary(bytes.NewReader(archiveData), "agrep.tar.gz", "agrep", &out)
	if err == nil {
		t.Fatal("expected error on unsafe path, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("expected 'unsafe path' error, got %v", err)
	}
}
