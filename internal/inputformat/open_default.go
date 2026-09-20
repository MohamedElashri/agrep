//go:build !formats

// Package inputformat opens plain-text input in the default build.
package inputformat

import (
	"io"
	"os"
)

// Open opens path as an untransformed byte stream. utf8Text reports whether
// the returned stream has already been converted to UTF-8.
func Open(path string) (reader io.ReadCloser, utf8Text bool, err error) {
	reader, err = os.Open(path)
	return reader, false, err
}
