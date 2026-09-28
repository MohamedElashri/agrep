// Package decode converts supported input encodings to UTF-8 before scanning.
package decode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	unicodeencoding "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Encoding identifies an input character encoding.
type Encoding uint8

const (
	UTF8 Encoding = iota
	Windows1256
	ISO88596
	UTF16LE
	UTF16BE
	Auto
)

const sniffBytes = 64 << 10

// Parse converts a command-line encoding name to an Encoding.
func Parse(name string) (Encoding, error) {
	switch name {
	case "", "utf8", "utf-8":
		return UTF8, nil
	case "cp1256", "windows-1256":
		return Windows1256, nil
	case "iso-8859-6", "iso8859-6":
		return ISO88596, nil
	case "utf16le", "utf-16le":
		return UTF16LE, nil
	case "utf16be", "utf-16be":
		return UTF16BE, nil
	case "auto":
		return Auto, nil
	default:
		return UTF8, fmt.Errorf("unknown encoding %q (want utf8, cp1256, iso-8859-6, utf16le, utf16be, or auto)", name)
	}
}

// String returns the canonical command-line spelling.
func (e Encoding) String() string {
	switch e {
	case Windows1256:
		return "cp1256"
	case ISO88596:
		return "iso-8859-6"
	case UTF16LE:
		return "utf16le"
	case UTF16BE:
		return "utf16be"
	case Auto:
		return "auto"
	default:
		return "utf8"
	}
}

// NewReader returns a UTF-8 reader and the encoding actually selected. Explicit
// UTF-16 modes accept an optional BOM. Auto gives BOMs priority, accepts valid
// UTF-8, then scores legacy candidates by Arabic-codepoint frequency.
func NewReader(r io.Reader, requested Encoding) (io.Reader, Encoding, error) {
	if requested != Auto {
		return readerFor(r, requested), requested, nil
	}

	buffered := bufio.NewReaderSize(r, sniffBytes)
	sample, err := buffered.Peek(sniffBytes)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, UTF8, fmt.Errorf("decode: sniff input: %w", err)
	}

	switch {
	case bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}):
		_, _ = buffered.Discard(3)
		return transform.NewReader(buffered, unicodeencoding.UTF8.NewDecoder()), UTF8, nil
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}):
		return readerFor(buffered, UTF16LE), UTF16LE, nil
	case bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		return readerFor(buffered, UTF16BE), UTF16BE, nil
	case plausibleUTF8(sample, len(sample) == sniffBytes):
		// The sniff only covers a prefix. Validate/repair later malformed
		// sequences too, so Auto always produces UTF-8 on a successful read.
		return transform.NewReader(buffered, unicodeencoding.UTF8.NewDecoder()), UTF8, nil
	}

	best := Windows1256
	bestScore := scoreCandidate(sample, Windows1256)
	candidates := []Encoding{ISO88596, UTF16LE, UTF16BE}
	for _, candidate := range candidates {
		if score := scoreCandidate(sample, candidate); score > bestScore {
			best, bestScore = candidate, score
		}
	}
	return readerFor(buffered, best), best, nil
}

func plausibleUTF8(sample []byte, allowIncompleteSuffix bool) bool {
	validPrefix := sample
	if !utf8.Valid(validPrefix) {
		if !allowIncompleteSuffix {
			return false
		}
		validPrefix = nil
		for suffixLength := 1; suffixLength < utf8.UTFMax && suffixLength <= len(sample); suffixLength++ {
			prefix := sample[:len(sample)-suffixLength]
			suffix := sample[len(sample)-suffixLength:]
			if utf8.Valid(prefix) && !utf8.FullRune(suffix) {
				validPrefix = prefix
				break
			}
		}
		if validPrefix == nil {
			return false
		}
	}
	for _, r := range string(validPrefix) {
		if r == 0 || (unicode.IsControl(r) && !unicode.IsSpace(r)) {
			return false
		}
	}
	return true
}

func readerFor(r io.Reader, selected Encoding) io.Reader {
	var decoder *encoding.Decoder
	switch selected {
	case Windows1256:
		decoder = charmap.Windows1256.NewDecoder()
	case ISO88596:
		decoder = charmap.ISO8859_6.NewDecoder()
	case UTF16LE:
		decoder = unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.UseBOM).NewDecoder()
	case UTF16BE:
		decoder = unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.UseBOM).NewDecoder()
	default:
		return r
	}
	return transform.NewReader(r, decoder)
}

func scoreCandidate(sample []byte, candidate Encoding) int {
	if (candidate == UTF16LE || candidate == UTF16BE) && len(sample)%2 != 0 {
		sample = sample[:len(sample)-1]
	}
	decoded, _, err := transform.Bytes(decoderFor(candidate), sample)
	if err != nil {
		return -1 << 30
	}
	score := 0
	for _, r := range string(decoded) {
		switch {
		case unicode.Is(unicode.Arabic, r):
			score += 12
		case r == utf8.RuneError || r == 0:
			score -= 20
		case unicode.IsControl(r) && !unicode.IsSpace(r):
			score -= 8
		case unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r) || unicode.IsSpace(r):
			score++
		}
	}
	return score
}

func decoderFor(selected Encoding) transform.Transformer {
	switch selected {
	case Windows1256:
		return charmap.Windows1256.NewDecoder()
	case ISO88596:
		return charmap.ISO8859_6.NewDecoder()
	case UTF16LE:
		return unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.UseBOM).NewDecoder()
	case UTF16BE:
		return unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.UseBOM).NewDecoder()
	default:
		return unicodeencoding.UTF8BOM.NewDecoder()
	}
}
