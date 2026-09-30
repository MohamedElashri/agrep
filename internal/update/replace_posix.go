//go:build !windows

package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func replaceBinaryOS(targetPath string, newBinary io.Reader) error {
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)

	// Create temp file in the SAME directory as the target executable.
	// This ensures both files reside on the same filesystem/mount point,
	// allowing atomic rename(2) without EXDEV errors.
	tempFile, err := os.CreateTemp(dir, "."+base+"-update-*.tmp")
	if err != nil {
		if errors.Is(err, os.ErrPermission) || os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s: please run with elevated permissions (e.g. 'sudo agrep --update')", dir)
		}
		return fmt.Errorf("failed to create temporary update file in %s: %w", dir, err)
	}

	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	// Write new binary
	if _, err := io.Copy(tempFile, newBinary); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write updated binary: %w", err)
	}

	// Flush to disk
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync updated binary to disk: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close updated binary: %w", err)
	}

	// Make executable (rwxr-xr-x)
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions on %s: %w", tempPath, err)
	}

	// Atomically replace target binary
	if err := os.Rename(tempPath, targetPath); err != nil {
		if errors.Is(err, os.ErrPermission) || os.IsPermission(err) {
			return fmt.Errorf("permission denied replacing %s: please run with elevated permissions (e.g. 'sudo agrep --update')", targetPath)
		}
		return fmt.Errorf("failed to replace %s: %w", targetPath, err)
	}

	cleanup = false
	return nil
}
