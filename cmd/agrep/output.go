package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/scan"
)

func jsonEmitter(w io.Writer) func(scan.Match) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return func(m scan.Match) error {
		if m.File == "(standard input)" {
			m.File = ""
		}
		return encoder.Encode(m)
	}
}

type humanOutputOptions struct {
	lineNumbers  bool
	filenames    bool
	onlyMatching bool
	highlight    bool
	bidiIsolate  bool
}

func humanEmitter(w io.Writer, opts humanOutputOptions) func(scan.Match) error {
	// Ordinary lines can reuse one byte buffer across emissions. Highlighted
	// and match-only output keep the span-aware renderer below.
	if !opts.onlyMatching && !opts.highlight {
		var line []byte
		return func(m scan.Match) error {
			if m.GroupStart {
				if _, err := io.WriteString(w, "--\n"); err != nil {
					return err
				}
			}
			line = line[:0]
			separator := byte(':')
			if m.Context {
				separator = '-'
			}
			if opts.filenames {
				line = append(line, m.File...)
				line = append(line, separator)
			}
			if opts.lineNumbers {
				line = strconv.AppendInt(line, m.Line, 10)
				line = append(line, separator)
			}
			line = append(line, m.Text...)
			line = append(line, '\n')
			_, err := w.Write(line)
			return err
		}
	}
	return func(m scan.Match) error {
		if m.GroupStart {
			if _, err := fmt.Fprintln(w, "--"); err != nil {
				return err
			}
		}
		separator := ":"
		if m.Context {
			separator = "-"
		}
		var prefix strings.Builder
		if opts.filenames {
			prefix.WriteString(m.File)
			prefix.WriteString(separator)
		}
		if opts.lineNumbers {
			prefix.WriteString(strconv.FormatInt(m.Line, 10))
			prefix.WriteString(separator)
		}
		if opts.onlyMatching && !m.Context {
			for _, span := range m.Spans {
				if span[0] == span[1] {
					continue
				}
				text := m.Text[span[0]:span[1]]
				if opts.highlight {
					text = colorize(text, []scan.Span{{0, len(text)}}, opts.bidiIsolate)
				}
				if _, err := fmt.Fprintln(w, prefix.String()+text); err != nil {
					return err
				}
			}
			return nil
		}
		text := m.Text
		if opts.highlight && !m.Context {
			text = colorize(text, m.Spans, opts.bidiIsolate)
		}
		_, err := fmt.Fprintln(w, prefix.String()+text)
		return err
	}
}

const (
	ansiMatchStart = "\x1b[01;31m"
	ansiMatchEnd   = "\x1b[m"
)

func colorize(text string, spans []scan.Span, bidiIsolate bool) string {
	spans = mergedSpans(spans, len(text))
	if len(spans) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text) + len(spans)*(len(ansiMatchStart)+len(ansiMatchEnd)+6))
	cursor := 0
	for _, span := range spans {
		b.WriteString(text[cursor:span[0]])
		matched := text[span[0]:span[1]]
		isolate := bidiIsolate && containsRTL(matched)
		if isolate {
			b.WriteRune(rune(0x2067)) // RLI
		}
		b.WriteString(ansiMatchStart)
		b.WriteString(matched)
		b.WriteString(ansiMatchEnd)
		if isolate {
			b.WriteRune(rune(0x2069)) // PDI
		}
		cursor = span[1]
	}
	b.WriteString(text[cursor:])
	return b.String()
}

func mergedSpans(spans []scan.Span, textLen int) []scan.Span {
	merged := make([]scan.Span, 0, len(spans))
	for _, span := range spans {
		if span[0] < 0 || span[1] <= span[0] || span[1] > textLen {
			continue
		}
		if len(merged) > 0 && span[0] <= merged[len(merged)-1][1] {
			if span[1] > merged[len(merged)-1][1] {
				merged[len(merged)-1][1] = span[1]
			}
			continue
		}
		merged = append(merged, span)
	}
	return merged
}

func containsRTL(s string) bool {
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Hebrew, r) {
			return true
		}
		s = s[size:]
	}
	return false
}
