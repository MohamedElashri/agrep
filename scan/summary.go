package scan

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/MohamedElashri/agrep/match"
	simdutf8 "github.com/segmentio/asm/utf8"
)

// searchSummary handles searches that only need selected line numbers. A
// literal matcher's negative proof is also valid over a group of complete
// lines: when the group lacks every required raw anchor, none of its lines
// can match after normalization. Complete groups can then be validated and
// counted without constructing one Match per line.
func searchSummary(r io.Reader, m match.Matcher, opts Options,
	negative interface {
		CannotMatchRawBytes([]byte) bool
		FirstPossibleRawByteIndex([]byte) int
	}, onMatch func(Match) error) (bool, error) {
	reader := bufio.NewReader(r)
	buf := make([]byte, 256<<10)
	var pending []byte
	var lineNumber int64
	var found bool
	emptyReads := 0
	profile := m.Profile()
	positive, hasPositive := m.(interface{ MatchesStableRawBytes([]byte) bool })
	fast, hasFast := m.(interface{ Matches(string) bool })

	processLine := func(line []byte, validated bool) error {
		lineNumber++
		if !validated && !simdutf8.Valid(line) {
			return fmt.Errorf("%w on line %d", ErrInvalidUTF8, lineNumber)
		}
		if opts.ExistenceOnly && found {
			return nil
		}
		selected := hasPositive && positive.MatchesStableRawBytes(line)
		if !selected && !negative.CannotMatchRawBytes(line) {
			normalized := profile.Normalize(string(line))
			if hasFast {
				selected = fast.Matches(normalized)
			} else {
				selected = len(m.FindAll(normalized)) > 0
			}
		}
		if selected {
			found = true
			return onMatch(Match{File: opts.File, Line: lineNumber})
		}
		return nil
	}

	processLines := func(group []byte) error {
		for len(group) > 0 {
			end := bytes.IndexByte(group, '\n') + 1
			if err := processLine(trimLineEndingBytes(group[:end]), false); err != nil {
				return err
			}
			group = group[end:]
		}
		return nil
	}
	processGroup := func(group []byte) error {
		if !simdutf8.Valid(group) {
			return processLines(group)
		}
		for len(group) > 0 {
			if opts.ExistenceOnly && found {
				lineNumber += int64(bytes.Count(group, []byte{'\n'}))
				return nil
			}
			candidate := negative.FirstPossibleRawByteIndex(group)
			if candidate < 0 {
				lineNumber += int64(bytes.Count(group, []byte{'\n'}))
				return nil
			}
			lineStart := bytes.LastIndexByte(group[:candidate], '\n') + 1
			if lineStart > 0 {
				prefix := group[:lineStart]
				lineNumber += int64(bytes.Count(prefix, []byte{'\n'}))
			}
			group = group[lineStart:]
			end := bytes.IndexByte(group, '\n') + 1
			if err := processLine(trimLineEndingBytes(group[:end]), true); err != nil {
				return err
			}
			group = group[end:]
		}
		return nil
	}

	for {
		n, readErr := reader.Read(buf)
		if n == 0 && readErr == nil {
			emptyReads++
			if emptyReads >= 100 {
				return false, io.ErrNoProgress
			}
			continue
		}
		emptyReads = 0
		data := buf[:n]
		if len(pending) > 0 {
			if end := bytes.IndexByte(data, '\n'); end >= 0 {
				pending = append(pending, data[:end+1]...)
				if err := processLine(trimLineEndingBytes(pending), false); err != nil {
					return false, err
				}
				pending = pending[:0]
				data = data[end+1:]
			} else {
				pending = append(pending, data...)
				data = nil
			}
		}
		if last := bytes.LastIndexByte(data, '\n'); last >= 0 {
			if err := processGroup(data[:last+1]); err != nil {
				return false, err
			}
			data = data[last+1:]
		}
		pending = append(pending, data...)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return false, readErr
			}
			if len(pending) > 0 {
				if err := processLine(pending, false); err != nil {
					return false, err
				}
			}
			return found, nil
		}
	}
}
