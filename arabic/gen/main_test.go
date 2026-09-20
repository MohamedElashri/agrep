package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A small, hand-picked fixture in the same format as UnicodeData.txt,
// covering: a single-codepoint contextual form, a multi-codepoint
// ligature, an entry with no decomposition (should be skipped), an entry
// with a tag we don't want (canonical, no tag at all: should be skipped),
// and a codepoint outside the two presentation-form blocks (should be
// skipped even though it has a qualifying tag).
const fixture = `0041;LATIN CAPITAL LETTER A;Lu;0;L;;;;;N;;;;0061;
00C0;LATIN CAPITAL LETTER A WITH GRAVE;Lu;0;L;0041 0300;;;;N;LATIN CAPITAL LETTER A GRAVE;;;00E0;
FB50;ARABIC LETTER ALEF WASLA ISOLATED FORM;Lo;0;AL;<isolated> 0671;;;;N;;;;;
FEFB;ARABIC LIGATURE LAM WITH ALEF ISOLATED FORM;Lo;0;AL;<isolated> 0644 0627;;;;N;;;;;
FBB2;ARABIC SYMBOL DOT ABOVE;Sk;0;ON;;;;;N;;;;;
FDFD;ARABIC LIGATURE BISMILLAH AR-RAHMAN AR-RAHEEM;So;0;ON;;;;;N;;;;;
`

func TestParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "UnicodeData.txt")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := map[rune]string{
		0xFB50: "ٱ",
		0xFEFB: "لا",
	}
	if len(entries) != len(want) {
		t.Fatalf("parse() returned %d entries; want %d: %+v", len(entries), len(want), entries)
	}
	for _, e := range entries {
		wantExpansion, ok := want[e.codepoint]
		if !ok {
			t.Errorf("unexpected entry for U+%04X (%s): should have been filtered out", e.codepoint, e.name)
			continue
		}
		if e.expansion != wantExpansion {
			t.Errorf("U+%04X expansion = %q; want %q", e.codepoint, e.expansion, wantExpansion)
		}
	}
}

func TestDecodeCodepoints(t *testing.T) {
	tests := []struct {
		field, want string
	}{
		{"0644 0627", "لا"},
		{"0671", "ٱ"},
		{"0020 064E", " َ"},
	}
	for _, tt := range tests {
		got, err := decodeCodepoints(tt.field)
		if err != nil {
			t.Fatalf("decodeCodepoints(%q): %v", tt.field, err)
		}
		if got != tt.want {
			t.Errorf("decodeCodepoints(%q) = %q; want %q", tt.field, got, tt.want)
		}
	}

	if _, err := decodeCodepoints("not-hex"); err == nil {
		t.Fatal("decodeCodepoints(\"not-hex\") returned no error")
	}
}

func TestRenderIsValidGoAndSorted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "UnicodeData.txt")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := parse(path)
	if err != nil {
		t.Fatal(err)
	}

	src, err := render(entries, "test-version")
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := string(src)
	if !strings.Contains(out, "package arabic") {
		t.Error("generated source is missing \"package arabic\"")
	}
	if !strings.Contains(out, "DO NOT EDIT") {
		t.Error("generated source is missing a DO NOT EDIT warning")
	}
	if !strings.Contains(out, "test-version") {
		t.Error("generated source does not record the Unicode version it was passed")
	}
	// FB50 must appear before FEFB: render() sorts by codepoint.
	if i, j := strings.Index(out, "0xFB50"), strings.Index(out, "0xFEFB"); i < 0 || j < 0 || i > j {
		t.Errorf("entries not sorted by codepoint: 0xFB50 at %d, 0xFEFB at %d", i, j)
	}
}
