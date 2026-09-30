package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CurrentExecutable determines the canonical filesystem path of the running executable,
// resolving any symbolic links.
func CurrentExecutable() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine executable path: %w", err)
	}

	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	if isTempPath(realPath) {
		return "", fmt.Errorf("agrep is running from temporary path %s (possibly via 'go run'); cannot self-update", realPath)
	}

	return realPath, nil
}

func isTempPath(p string) bool {
	tmpDir := os.TempDir()
	cleanPath := filepath.Clean(p)
	cleanTmp := filepath.Clean(tmpDir)

	if strings.HasPrefix(cleanPath, cleanTmp) {
		return true
	}
	if strings.Contains(cleanPath, "go-build") {
		return true
	}
	return false
}

// DetectInstallMethod checks if agrep was installed via a package manager like Homebrew or go install.
func DetectInstallMethod(execPath string) string {
	clean := filepath.ToSlash(execPath)
	if strings.Contains(clean, "/Cellar/") || strings.Contains(clean, "/homebrew/") || strings.Contains(clean, "/.linuxbrew/") {
		return "homebrew"
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			gopath = filepath.Join(home, "go")
		}
	}
	if gopath != "" {
		goBin := filepath.ToSlash(filepath.Join(gopath, "bin"))
		if strings.HasPrefix(clean, goBin) {
			return "go-install"
		}
	}

	return "binary"
}

// ReplaceBinary safely and atomically replaces targetPath with the content read from newBinary.
func ReplaceBinary(targetPath string, newBinary io.Reader) error {
	return replaceBinaryOS(targetPath, newBinary)
}
