package decode

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	unicodeencoding "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name string
		want Encoding
	}{
		{"utf8", UTF8},
		{"cp1256", Windows1256},
		{"iso-8859-6", ISO88596},
		{"utf16le", UTF16LE},
		{"utf16be", UTF16BE},
		{"auto", Auto},
	} {
		got, err := Parse(tt.name)
		if err != nil || got != tt.want {
			t.Fatalf("Parse(%q) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	if _, err := Parse("unknown"); err == nil {
		t.Fatal("Parse accepted an unknown encoding")
	}
}

func TestNewReaderAutoHandlesUTF8RuneAcrossSniffBoundary(t *testing.T) {
	input := strings.Repeat("a", sniffBytes-1) + "م\n"
	reader, detected, err := NewReader(strings.NewReader(input), Auto)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if detected != UTF8 || string(got) != input {
		t.Fatalf("detected=%v len(text)=%d; want utf8 and %d", detected, len(got), len(input))
	}
}

func FuzzAutoDecodeValidUTF8(f *testing.F) {
	f.Add([]byte("هذه مدينة\n"))
	f.Add([]byte{0xDF, 0xD1, 0xC8, 0xED})
	f.Add([]byte{0xFF, 0xFE, 0x43, 0x06})
	f.Fuzz(func(t *testing.T, raw []byte) {
		reader, _, err := NewReader(bytes.NewReader(raw), Auto)
		if err != nil {
			return
		}
		got, err := io.ReadAll(reader)
		if err == nil && !utf8.Valid(got) {
			t.Fatalf("auto decoding returned invalid UTF-8: %x", got)
		}
	})
}

func TestNewReaderExplicitEncodings(t *testing.T) {
	const text = "كتاب، مدينة\n"
	for _, tt := range []struct {
		name     string
		selected Encoding
		encoder  *encoding.Encoder
	}{
		{"cp1256", Windows1256, charmap.Windows1256.NewEncoder()},
		{"iso-8859-6", ISO88596, charmap.ISO8859_6.NewEncoder()},
		{"utf16le", UTF16LE, unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.UseBOM).NewEncoder()},
		{"utf16be", UTF16BE, unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.UseBOM).NewEncoder()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, _, err := transform.Bytes(tt.encoder, []byte(text))
			if err != nil {
				t.Fatal(err)
			}
			reader, detected, err := NewReader(bytes.NewReader(raw), tt.selected)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if detected != tt.selected || string(got) != text {
				t.Fatalf("detected=%v text=%q; want %v and %q", detected, got, tt.selected, text)
			}
		})
	}
}

func TestNewReaderAuto(t *testing.T) {
	const text = "هذه مدينة\n"
	for _, tt := range []struct {
		name    string
		want    Encoding
		encoder *encoding.Encoder
		prefix  []byte
	}{
		{"utf8", UTF8, nil, nil},
		{"utf8 BOM", UTF8, nil, []byte{0xEF, 0xBB, 0xBF}},
		{"cp1256 heuristic", Windows1256, charmap.Windows1256.NewEncoder(), nil},
		{"utf16le BOM", UTF16LE, unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.UseBOM).NewEncoder(), nil},
		{"utf16le without BOM", UTF16LE, unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.IgnoreBOM).NewEncoder(), nil},
		{"utf16be without BOM", UTF16BE, unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.IgnoreBOM).NewEncoder(), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := append([]byte(nil), tt.prefix...)
			if tt.encoder == nil {
				raw = append(raw, text...)
			} else {
				encoded, _, err := transform.Bytes(tt.encoder, []byte(text))
				if err != nil {
					t.Fatal(err)
				}
				raw = append(raw, encoded...)
			}
			reader, detected, err := NewReader(bytes.NewReader(raw), Auto)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if detected != tt.want || string(got) != text {
				t.Fatalf("detected=%v text=%q; want %v and %q", detected, got, tt.want, text)
			}
		})
	}
}
