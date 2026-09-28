package scan

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
)

// ErrInvalidUTF8 identifies input that must be decoded before Search. The CLI
// uses it to suggest --encoding=auto while library callers can use errors.Is.
var ErrInvalidUTF8 = errors.New("scan: input is not valid UTF-8")

// Span is a half-open byte range in Match.Text. Its array representation keeps
// the stable JSON form compact: [start,end].
type Span [2]int

// Match is one selected or context logical line. Text excludes the CR/LF line
// ending. Context identifies neighboring output rather than a selected line.
type Match struct {
	File       string `json:"file,omitempty"`
	Line       int64  `json:"line"`
	Text       string `json:"text"`
	Context    bool   `json:"context,omitempty"`
	GroupStart bool   `json:"-"`
	Spans      []Span `json:"spans,omitempty"`
}

// Options controls how Search reads input.
type Options struct {
	// MaxLineBytes is measured after removing CR/LF. Zero means unlimited.
	MaxLineBytes uint64
	// File is copied into emitted Match values. Empty identifies stdin or an
	// unnamed reader and is omitted from JSON.
	File string
	// InvertMatch selects lines the matcher does not find.
	InvertMatch bool
	// BeforeContext and AfterContext emit neighboring lines around selected
	// lines. Context lines have Match.Context set.
	BeforeContext int
	AfterContext  int
	// MapSpans maps matcher offsets back into the original line and includes
	// them in Match.Spans. It is intentionally opt-in because mapping allocates.
	MapSpans bool
	// WordRegexp keeps only matches bounded by non-word runes in the original
	// text. It implies mapped matching even when MapSpans is false.
	WordRegexp bool
	// OmitText leaves Match.Text empty for summary callers that need only
	// selection. It cannot be combined with context or MapSpans.
	OmitText bool
}

var mappedIndexPool sync.Pool

const maxPooledIndexCapacity = 4 << 20 // 16 MiB of int32 storage

// Search reads arbitrary-length logical lines from r without
// bufio.Scanner's token ceiling, and reports every line whose normalized text
// m selects, optionally inverted, plus requested context. Unless OmitText is
// set, it preserves the original, unnormalized text of every emitted line.
func Search(r io.Reader, m match.Matcher, opts Options, onMatch func(Match) error) (bool, error) {
	if opts.BeforeContext < 0 || opts.AfterContext < 0 {
		return false, errors.New("scan: context values must be non-negative")
	}
	if opts.OmitText && (opts.BeforeContext > 0 || opts.AfterContext > 0 || opts.MapSpans) {
		return false, errors.New("scan: OmitText cannot be combined with context or MapSpans")
	}
	reader := bufio.NewReader(r)
	profile := m.Profile()
	var lineNumber int64
	found := false
	before := make([]Match, 0, opts.BeforeContext)
	afterRemaining := 0
	var lastEmitted int64
	emitted := false
	contextEnabled := opts.BeforeContext > 0 || opts.AfterContext > 0
	needsMapped := opts.WordRegexp || (opts.MapSpans && !opts.InvertMatch)
	rawPositive, hasRawPositive := m.(interface{ MatchesStableRawBytes([]byte) bool })
	rawNegative, hasRawNegative := m.(interface{ CannotMatchRawBytes([]byte) bool })

	emit := func(mt Match, context bool) error {
		mt.Context = context
		mt.GroupStart = contextEnabled && emitted && mt.Line > lastEmitted+1
		if err := onMatch(mt); err != nil {
			return err
		}
		emitted = true
		lastEmitted = mt.Line
		return nil
	}

	for {
		line, done, err := readLineBytes(reader, opts.MaxLineBytes, lineNumber+1)
		if err != nil {
			return false, err
		}
		if done {
			break
		}

		lineNumber++
		if !utf8.Valid(line) {
			return false, fmt.Errorf("%w on line %d", ErrInvalidUTF8, lineNumber)
		}

		matched := false
		var originalSpans []Span
		var lineText string
		if !needsMapped && hasRawPositive && rawPositive.MatchesStableRawBytes(line) {
			matched = true
		} else if !needsMapped && hasRawNegative && rawNegative.CannotMatchRawBytes(line) {
			// No literal can match. Keep normal selection, inversion, and
			// context handling below without normalizing this line.
		} else if needsMapped {
			lineText = string(line)
			var scratch []int32
			var indexBuffer *[]int32
			if pooled := mappedIndexPool.Get(); pooled != nil {
				indexBuffer = pooled.(*[]int32)
				scratch = (*indexBuffer)[:0]
			} else {
				indexBuffer = new([]int32)
			}
			normalized, idx := profile.NormalizeMappedInto(lineText, scratch)
			normalizedSpans := m.FindAll(normalized)
			for _, span := range normalizedSpans {
				if span.Start < 0 || span.End < span.Start || span.End >= len(idx) {
					continue
				}
				start, end := arabic.MapSpan(idx, span.Start, span.End)
				if opts.WordRegexp && !wordBounded(lineText, start, end, profile.Languages) {
					continue
				}
				matched = true
				if opts.MapSpans {
					mapped := Span{start, end}
					if len(originalSpans) == 0 || originalSpans[len(originalSpans)-1] != mapped {
						originalSpans = append(originalSpans, mapped)
					}
				}
			}
			if cap(idx) <= maxPooledIndexCapacity {
				*indexBuffer = idx[:0]
				mappedIndexPool.Put(indexBuffer)
			}
		} else {
			lineText = string(line)
			normalized := profile.Normalize(lineText)
			if fast, ok := m.(interface{ Matches(string) bool }); ok {
				matched = fast.Matches(normalized)
			} else {
				matched = len(m.FindAll(normalized)) > 0
			}
		}
		selected := matched != opts.InvertMatch
		if !selected && afterRemaining == 0 && opts.BeforeContext == 0 {
			continue
		}
		text := lineText
		if opts.OmitText {
			text = ""
		} else if text == "" && len(line) > 0 {
			text = string(line)
		}
		mt := Match{File: opts.File, Line: lineNumber, Text: text, Spans: originalSpans}
		if selected {
			found = true
			for _, prior := range before {
				if prior.Line > lastEmitted {
					if err := emit(prior, true); err != nil {
						return false, err
					}
				}
			}
			before = before[:0]
			if err := emit(mt, false); err != nil {
				return false, err
			}
			afterRemaining = opts.AfterContext
			continue
		}

		if afterRemaining > 0 {
			if err := emit(mt, true); err != nil {
				return false, err
			}
			afterRemaining--
			continue
		}
		if opts.BeforeContext > 0 {
			if len(before) == opts.BeforeContext {
				copy(before, before[1:])
				before = before[:len(before)-1]
			}
			before = append(before, mt)
		}
	}

	return found, nil
}

func wordBounded(text string, start, end int, languages arabic.LanguageSet) bool {
	if start < 0 || end < start || end > len(text) {
		return false
	}
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if isWordRune(before, languages) {
			return false
		}
	}
	if end < len(text) {
		after, _ := utf8.DecodeRuneInString(text[end:])
		if isWordRune(after, languages) {
			return false
		}
	}
	return true
}

func isWordRune(r rune, languages arabic.LanguageSet) bool {
	return r == '_' || (r == '\u200c' && languages.PreservesZWNJ()) ||
		unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

// readLineBytes assembles one logical line from ReadSlice fragments. When a limit
// is configured, it rejects oversized input incrementally instead of first
// allocating the entire hostile line. done is true only for EOF with no data.
// The returned slice is valid until the reader's next read.
func readLineBytes(reader *bufio.Reader, maxBytes uint64, lineNumber int64) (line []byte, done bool, err error) {
	var buf bytes.Buffer
	for {
		fragment, readErr := reader.ReadSlice('\n')
		switch {
		case readErr == nil:
			if buf.Len() == 0 {
				line = trimLineEndingBytes(fragment)
			} else {
				_, _ = buf.Write(fragment)
				line = trimLineEndingBytes(buf.Bytes())
			}
		case errors.Is(readErr, bufio.ErrBufferFull):
			_, _ = buf.Write(fragment)
			if maxBytes > 0 && uint64(buf.Len()) > maxBytes {
				return nil, false, fmt.Errorf("line %d exceeds --max-line-bytes=%d", lineNumber, maxBytes)
			}
			continue
		case errors.Is(readErr, io.EOF):
			if buf.Len() == 0 && len(fragment) == 0 {
				return nil, true, nil
			}
			if buf.Len() == 0 {
				line = fragment
			} else {
				_, _ = buf.Write(fragment)
				line = buf.Bytes()
			}
		default:
			return nil, false, readErr
		}

		if maxBytes > 0 && uint64(len(line)) > maxBytes {
			return nil, false, fmt.Errorf("line %d exceeds --max-line-bytes=%d", lineNumber, maxBytes)
		}
		return line, false, nil
	}
}

func trimLineEndingBytes(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
	}
	return line
}
