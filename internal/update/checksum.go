package update

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// ParseChecksums parses standard checksums.txt content (as produced by sha256sum or GoReleaser)
// into a map of filename -> expected sha256 hex string.
func ParseChecksums(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	checksums := make(map[string]string)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		hash := strings.ToLower(fields[0])
		// Check that hash is a 64-char hex string
		if len(hash) != 64 {
			continue
		}

		filename := fields[1]
		// In binary mode, sha256sum prefixes filename with '*'
		filename = strings.TrimPrefix(filename, "*")
		filename = strings.TrimPrefix(filename, "./")
		filename = strings.TrimSpace(filename)

		if filename != "" {
			checksums[filename] = hash
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}

	return checksums, nil
}

// VerifyChecksum computes the SHA-256 hash of data read from r and compares it
// in constant time with expectedHash.
func VerifyChecksum(r io.Reader, expectedHash string) error {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(expectedHash))

	if subtle.ConstantTimeCompare([]byte(actualHash), []byte(expected)) != 1 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actualHash)
	}

	return nil
}
