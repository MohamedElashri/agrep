//go:build windows

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

	tempFile, err := os.CreateTemp(dir, "."+base+"-update-*.tmp")
	if err != nil {
		if errors.Is(err, os.ErrPermission) || os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s: please run as Administrator", dir)
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

	if _, err := io.Copy(tempFile, newBinary); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write updated binary: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync updated binary to disk: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close updated binary: %w", err)
	}

	// On Windows, a running executable cannot be directly overwritten or renamed into.
	// Step 1: Rename targetPath to targetPath.old
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath) // remove previous .old if exists

	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("failed to move existing binary to %s: %w", oldPath, err)
		}
	}

	// Step 2: Rename temp file to targetPath
	if err := os.Rename(tempPath, targetPath); err != nil {
		// Attempt rollback
		_ = os.Rename(oldPath, targetPath)
		return fmt.Errorf("failed to replace %s: %w", targetPath, err)
	}

	// Step 3: Best effort cleanup of .old file (may be locked until process exits)
	_ = os.Remove(oldPath)

	cleanup = false
	return nil
}
