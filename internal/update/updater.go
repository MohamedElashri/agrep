package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Updater manages checking for updates and applying them.
type Updater struct {
	CurrentVersion   string
	Client           *Client
	TargetExecutable string
	GOOS             string
	GOARCH           string
}

// New creates an Updater with default settings for the given version.
func New(currentVersion string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		Client:         NewClient(currentVersion),
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	}
}

// CheckResult contains information about available updates.
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	IsDev           bool
	Release         *Release
}

// Check queries GitHub for the latest release and determines whether an update is available.
func (u *Updater) Check(ctx context.Context) (*CheckResult, error) {
	rel, err := u.Client.LatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	isDev := IsDev(u.CurrentVersion)
	updateAvailable := isDev || IsNewer(u.CurrentVersion, rel.TagName)

	return &CheckResult{
		CurrentVersion:  u.CurrentVersion,
		LatestVersion:   rel.TagName,
		UpdateAvailable: updateAvailable,
		IsDev:           isDev,
		Release:         rel,
	}, nil
}

// UpdateOptions configures the update operation.
type UpdateOptions struct {
	TargetTag    string
	Force        bool
	StatusWriter io.Writer
}

// UpdateResult contains the result of an update operation.
type UpdateResult struct {
	PreviousVersion string
	NewVersion      string
	ExecutablePath  string
	AlreadyUpToDate bool
	InstallMethod   string
}

// Update performs the self-update process:
// 1. Locates the current executable
// 2. Fetches release metadata
// 3. Resolves and downloads checksums
// 4. Downloads the matching archive
// 5. Cryptographically verifies SHA-256
// 6. Extracts the binary safely
// 7. Atomically replaces the current executable
func (u *Updater) Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	execPath := u.TargetExecutable
	if execPath == "" {
		var err error
		execPath, err = CurrentExecutable()
		if err != nil {
			return nil, err
		}
	}

	status := func(format string, a ...any) {
		if opts.StatusWriter != nil {
			fmt.Fprintf(opts.StatusWriter, format, a...)
		}
	}

	var rel *Release
	var err error

	if opts.TargetTag != "" {
		status("Fetching release %s...\n", opts.TargetTag)
		rel, err = u.Client.ReleaseByTag(ctx, opts.TargetTag)
		if err != nil {
			return nil, err
		}
	} else {
		status("Checking for updates (current version: %s)...\n", u.CurrentVersion)
		rel, err = u.Client.LatestRelease(ctx)
		if err != nil {
			return nil, err
		}

		if !opts.Force && !IsDev(u.CurrentVersion) && !IsNewer(u.CurrentVersion, rel.TagName) {
			status("agrep is already up to date (%s).\n", rel.TagName)
			return &UpdateResult{
				PreviousVersion: u.CurrentVersion,
				NewVersion:      rel.TagName,
				ExecutablePath:  execPath,
				AlreadyUpToDate: true,
				InstallMethod:   DetectInstallMethod(execPath),
			}, nil
		}
	}

	status("Target release: %s\n", rel.TagName)

	asset, err := u.Client.FindAsset(rel, u.GOOS, u.GOARCH)
	if err != nil {
		return nil, fmt.Errorf("resolving asset for %s/%s: %w", u.GOOS, u.GOARCH, err)
	}

	chkAsset, err := u.Client.FindChecksums(rel)
	if err != nil {
		return nil, fmt.Errorf("resolving checksums: %w", err)
	}

	// 1. Fetch checksums.txt
	status("Fetching checksums (%s)...\n", chkAsset.Name)
	var chkBuf bytes.Buffer
	if err := u.Client.Download(ctx, chkAsset.DownloadURL, &chkBuf); err != nil {
		return nil, fmt.Errorf("downloading checksums from %s: %w", chkAsset.DownloadURL, err)
	}

	checksums, err := ParseChecksums(&chkBuf)
	if err != nil {
		return nil, fmt.Errorf("parsing checksums: %w", err)
	}

	expectedHash, ok := checksums[asset.Name]
	if !ok {
		return nil, fmt.Errorf("checksum for %s not found in %s", asset.Name, chkAsset.Name)
	}

	// 2. Download release archive to temporary file
	status("Downloading %s...\n", asset.Name)
	tempDir, err := os.MkdirTemp("", "agrep-update-*")
	if err != nil {
		return nil, fmt.Errorf("creating temporary download directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, asset.Name)
	archiveFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("creating archive file: %w", err)
	}

	if err := u.Client.Download(ctx, asset.DownloadURL, archiveFile); err != nil {
		_ = archiveFile.Close()
		return nil, fmt.Errorf("downloading %s: %w", asset.Name, err)
	}

	// 3. Verify SHA-256 checksum
	status("Verifying SHA-256 checksum... ")
	if _, err := archiveFile.Seek(0, io.SeekStart); err != nil {
		_ = archiveFile.Close()
		return nil, fmt.Errorf("seeking archive file: %w", err)
	}

	if err := VerifyChecksum(archiveFile, expectedHash); err != nil {
		status("FAILED\n")
		_ = archiveFile.Close()
		return nil, fmt.Errorf("integrity check failed: %w", err)
	}
	status("OK\n")

	// 4. Extract binary from archive
	status("Extracting binary... ")
	if _, err := archiveFile.Seek(0, io.SeekStart); err != nil {
		_ = archiveFile.Close()
		return nil, fmt.Errorf("seeking archive file: %w", err)
	}

	binaryPath := filepath.Join(tempDir, "extracted-agrep")
	binaryFile, err := os.OpenFile(binaryPath, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		_ = archiveFile.Close()
		return nil, fmt.Errorf("creating extracted binary file: %w", err)
	}

	binaryName := "agrep"
	if u.GOOS == "windows" {
		binaryName = "agrep.exe"
	}

	if err := ExtractBinary(archiveFile, asset.Name, binaryName, binaryFile); err != nil {
		_ = archiveFile.Close()
		_ = binaryFile.Close()
		status("FAILED\n")
		return nil, fmt.Errorf("extracting binary from %s: %w", asset.Name, err)
	}
	_ = archiveFile.Close()

	if _, err := binaryFile.Seek(0, io.SeekStart); err != nil {
		_ = binaryFile.Close()
		return nil, fmt.Errorf("seeking extracted binary: %w", err)
	}
	status("OK\n")

	// 5. Replace executable
	status("Replacing %s... ", execPath)
	if err := ReplaceBinary(execPath, binaryFile); err != nil {
		_ = binaryFile.Close()
		status("FAILED\n")
		return nil, err
	}
	_ = binaryFile.Close()
	status("OK\n")

	installMethod := DetectInstallMethod(execPath)
	if installMethod == "homebrew" {
		status("Note: agrep was installed via Homebrew. You can also update with: brew upgrade agrep\n")
	} else if installMethod == "go-install" {
		status("Note: agrep was installed via 'go install'. You can also update with: go install github.com/MohamedElashri/agrep/cmd/agrep@latest\n")
	}

	status("Successfully updated agrep to %s!\n", rel.TagName)

	return &UpdateResult{
		PreviousVersion: u.CurrentVersion,
		NewVersion:      rel.TagName,
		ExecutablePath:  execPath,
		AlreadyUpToDate: false,
		InstallMethod:   installMethod,
	}, nil
}
