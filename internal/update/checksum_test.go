package update

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	input := `
# Checksums for agrep v0.1.0
c42d138da74a8fa23c97e2429f45ae0abd07dc70eb86d8cb4a53dc32ad5c2bd0  agrep_0.1.0_linux_amd64.tar.gz
99e218486b808a61aee6251734e9a3bb609e197ea600308512bebd7aaba8aceb *agrep_0.1.0_linux_arm64.tar.gz
489ec5a22fac5799865246c040decc2d3077b65337d086590630051cbabde3dc  ./agrep_0.1.0_darwin_amd64.tar.gz
invalid_line_without_hash
1234 invalid_short_hash file.txt
`
	m, err := ParseChecksums(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseChecksums failed: %v", err)
	}

	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}

	if got := m["agrep_0.1.0_linux_amd64.tar.gz"]; got != "c42d138da74a8fa23c97e2429f45ae0abd07dc70eb86d8cb4a53dc32ad5c2bd0" {
		t.Errorf("unexpected hash for linux_amd64: %s", got)
	}
	if got := m["agrep_0.1.0_linux_arm64.tar.gz"]; got != "99e218486b808a61aee6251734e9a3bb609e197ea600308512bebd7aaba8aceb" {
		t.Errorf("unexpected hash for linux_arm64: %s", got)
	}
	if got := m["agrep_0.1.0_darwin_amd64.tar.gz"]; got != "489ec5a22fac5799865246c040decc2d3077b65337d086590630051cbabde3dc" {
		t.Errorf("unexpected hash for darwin_amd64: %s", got)
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := "hello world from agrep updater\n"
	h := sha256.Sum256([]byte(data))
	expectedHash := hex.EncodeToString(h[:])

	// Correct hash
	if err := VerifyChecksum(strings.NewReader(data), expectedHash); err != nil {
		t.Errorf("VerifyChecksum with correct hash failed: %v", err)
	}

	// Uppercase hash should also pass
	if err := VerifyChecksum(strings.NewReader(data), strings.ToUpper(expectedHash)); err != nil {
		t.Errorf("VerifyChecksum with uppercase hash failed: %v", err)
	}

	// Mismatched hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := VerifyChecksum(strings.NewReader(data), wrongHash); err == nil {
		t.Error("VerifyChecksum with wrong hash should have failed")
	}
}
