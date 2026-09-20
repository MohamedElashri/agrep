package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Match is one matching logical line. Text excludes the CR/LF line ending.
type Match struct {
	Line int64  `json:"line"`
	Text string `json:"text"`
}

type searchOptions struct {
	// MaxLineBytes is measured after removing CR/LF. Zero means unlimited.
	MaxLineBytes uint64
}

// search reads arbitrary-length logical lines without bufio.Scanner's token
// ceiling. It normalizes the query once and preserves every matching line.
func search(r io.Reader, query string, opts searchOptions, onMatch func(Match) error) (bool, error) {
	normalizedQuery, err := normalizeQuery(query)
	if err != nil {
		return false, err
	}

	reader := bufio.NewReader(r)
	var lineNumber int64
	found := false

	for {
		line, done, err := readLine(reader, opts.MaxLineBytes, lineNumber+1)
		if err != nil {
			return false, err
		}
		if done {
			break
		}

		lineNumber++
		if !utf8.ValidString(line) {
			return false, fmt.Errorf("line %d is not valid UTF-8", lineNumber)
		}

		if strings.Contains(normalizeArabic(line), normalizedQuery) {
			found = true
			if err := onMatch(Match{Line: lineNumber, Text: line}); err != nil {
				return false, err
			}
		}
	}

	return found, nil
}

// readLine assembles one logical line from ReadSlice fragments. When a limit
// is configured, it rejects oversized input incrementally instead of first
// allocating the entire hostile line. done is true only for EOF with no data.
func readLine(reader *bufio.Reader, maxBytes uint64, lineNumber int64) (line string, done bool, err error) {
	var buf bytes.Buffer

	for {
		fragment, readErr := reader.ReadSlice('\n')
		if len(fragment) > 0 {
			_, _ = buf.Write(fragment) // bytes.Buffer.Write never returns an error.
		}

		switch {
		case readErr == nil:
			line = trimLineEnding(buf.String())
		case errors.Is(readErr, bufio.ErrBufferFull):
			if maxBytes > 0 && uint64(buf.Len()) > maxBytes {
				return "", false, fmt.Errorf("line %d exceeds --max-line-bytes=%d", lineNumber, maxBytes)
			}
			continue
		case errors.Is(readErr, io.EOF):
			if buf.Len() == 0 {
				return "", true, nil
			}
			line = buf.String()
		default:
			return "", false, readErr
		}

		if maxBytes > 0 && uint64(len(line)) > maxBytes {
			return "", false, fmt.Errorf("line %d exceeds --max-line-bytes=%d", lineNumber, maxBytes)
		}
		return line, false, nil
	}
}

func trimLineEnding(line string) string {
	if strings.HasSuffix(line, "\n") {
		line = line[:len(line)-1]
		if strings.HasSuffix(line, "\r") {
			line = line[:len(line)-1]
		}
	}
	return line
}
