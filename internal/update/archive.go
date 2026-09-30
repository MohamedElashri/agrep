package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// maxExtractSize limits binary extraction to 150 MB to prevent decompression bombs.
const maxExtractSize = 150 << 20

// ExtractBinary extracts the specified executable binary from a .tar.gz or .zip archive.
func ExtractBinary(archiveReader io.Reader, archiveName, binaryName string, destWriter io.Writer) error {
	if strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		return extractBinaryFromZip(archiveReader, binaryName, destWriter)
	}
	return extractBinaryFromTarGz(archiveReader, binaryName, destWriter)
}

func extractBinaryFromTarGz(r io.Reader, binaryName string, dest io.Writer) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	targetBase := strings.ToLower(binaryName)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar archive: %w", err)
		}

		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("archive contains unsafe path: %q", hdr.Name)
		}

		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}

		base := strings.ToLower(filepath.Base(cleanName))
		if base == targetBase || base == targetBase+".exe" {
			limitReader := io.LimitReader(tr, maxExtractSize)
			n, err := io.Copy(dest, limitReader)
			if err != nil {
				return fmt.Errorf("extracting binary %q: %w", binaryName, err)
			}
			if n >= maxExtractSize {
				return fmt.Errorf("extracted binary exceeds maximum allowed size of %d bytes", maxExtractSize)
			}
			return nil
		}
	}

	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractBinaryFromZip(r io.Reader, binaryName string, dest io.Writer) error {
	buf, err := io.ReadAll(io.LimitReader(r, maxExtractSize*2))
	if err != nil {
		return fmt.Errorf("reading zip archive: %w", err)
	}

	readerAt := bytes.NewReader(buf)
	zr, err := zip.NewReader(readerAt, int64(len(buf)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}

	targetBase := strings.ToLower(binaryName)
	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("zip contains unsafe path: %q", f.Name)
		}

		base := strings.ToLower(filepath.Base(cleanName))
		if base == targetBase || base == targetBase+".exe" {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("opening file in zip: %w", err)
			}
			defer rc.Close()

			limitReader := io.LimitReader(rc, maxExtractSize)
			n, err := io.Copy(dest, limitReader)
			if err != nil {
				return fmt.Errorf("extracting binary from zip: %w", err)
			}
			if n >= maxExtractSize {
				return fmt.Errorf("extracted binary exceeds maximum allowed size of %d bytes", maxExtractSize)
			}
			return nil
		}
	}

	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}
