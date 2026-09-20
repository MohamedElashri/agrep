package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
)

func TestRunJSONLines(t *testing.T) {
	input := "ignore\nالمَدِينَة <tag>\n"
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "مدينه"}, strings.NewReader(input), &stdout, &stderr)
	want := "{\"line\":2,\"text\":\"المَدِينَة <tag>\"}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunHumanOutput(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-n", "needle"}, strings.NewReader("x\nneedle here\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "2:needle here\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDoubleDashAllowsHyphenQuery(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--", "-needle"}, strings.NewReader("a -needle here\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "a -needle here\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want int
	}{
		{"match", []string{"x"}, "x\n", 0},
		{"no match", []string{"x"}, "y\n", 1},
		{"missing query", nil, "", 2},
		{"too many args", []string{"x", "a", "b"}, "", 2},
		{"empty query", []string{""}, "x\n", 2},
		{"marks-only query", []string{"َ"}, "x\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if got := run(tt.args, strings.NewReader(tt.in), &stdout, &stderr); got != tt.want {
				t.Fatalf("exit code = %d; want %d", got, tt.want)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestRunWriteFailure(t *testing.T) {
	var stderr strings.Builder
	if code := run([]string{"x"}, strings.NewReader("x\n"), failingWriter{}, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}} {
		var stdout, stderr strings.Builder
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunReadFailure(t *testing.T) {
	reader := io.MultiReader(strings.NewReader("safe\n"), errorReader{})
	var stdout, stderr strings.Builder
	if code := run([]string{"missing"}, reader, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// withOverride returns a copy of base with mutate applied, so each test
// table row below can say precisely which fields it expects to change from
// the selected preset, instead of writing out a full Profile literal (and
// risking a transcription mistake in the eight fields it didn't mean to
// touch).
func withOverride(base arabic.Profile, mutate func(*arabic.Profile)) arabic.Profile {
	p := base
	mutate(&p)
	return p
}

// TestResolveProfile is the "precedence and conflicts" table test Phase 3
// calls for: each --keep-*/--fold-* flag only ever clears or sets one field
// relative to the selected preset, so there is no real conflict between
// any of them, but this locks in that every one of the nine, individually
// and combined, behaves exactly as documented rather than leaving it
// implicit.
func TestResolveProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   string
		overrides profileOverrides
		want      arabic.Profile
	}{
		{"default preset, no overrides", "", profileOverrides{}, arabic.ProfileSearch},
		{"search preset, no overrides", "search", profileOverrides{}, arabic.ProfileSearch},

		{"keep-hamza alone", "search", profileOverrides{keepHamza: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) {
				p.FoldAlefHamza, p.FoldAlefWasla, p.FoldHamzaSeat = false, false, false
			})},
		{"keep-tamarbuta alone", "search", profileOverrides{keepTaMarbuta: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.FoldTaMarbuta = false })},
		{"keep-tashkil alone", "search", profileOverrides{keepTashkil: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.StripTashkil = false })},
		{"keep-presentation-forms alone", "search", profileOverrides{keepPresentationForms: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.FoldPresentation = false })},
		{"keep-joiners alone", "search", profileOverrides{keepJoiners: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.StripJoiners = false })},
		{"keep-bidi-marks alone", "search", profileOverrides{keepBidiMarks: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.StripBidi = false })},
		{"keep-quranic-marks alone", "search", profileOverrides{keepQuranicMarks: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.StripQuranic = false })},
		{"fold-digits alone", "search", profileOverrides{foldDigits: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.FoldDigits = true })},
		{"fold-punctuation alone", "search", profileOverrides{foldPunctuation: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.FoldPunctuation = true })},

		{"all nine overrides together",
			"search",
			profileOverrides{
				keepHamza: true, keepTaMarbuta: true, keepTashkil: true,
				keepPresentationForms: true, keepJoiners: true, keepBidiMarks: true, keepQuranicMarks: true,
				foldDigits: true, foldPunctuation: true,
			},
			// StripTatweel and FoldAlefMaksura have no --keep-* flag, so
			// they must survive from the preset untouched — asserting the
			// full Profile here (not just the touched fields) is what
			// catches that kind of omission.
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) {
				p.FoldAlefHamza, p.FoldAlefWasla, p.FoldHamzaSeat = false, false, false
				p.FoldTaMarbuta = false
				p.StripTashkil = false
				p.FoldPresentation = false
				p.StripJoiners = false
				p.StripBidi = false
				p.StripQuranic = false
				p.FoldDigits = true
				p.FoldPunctuation = true
			})},

		{"strict preset already has hamza/ta-marbuta off; keep-* is a no-op on top",
			"strict", profileOverrides{keepHamza: true, keepTaMarbuta: true}, arabic.ProfileStrict},

		{"lucene preset, keep-hamza overrides its own hamza fold", "lucene", profileOverrides{keepHamza: true},
			withOverride(arabic.ProfileLucene, func(p *arabic.Profile) {
				p.FoldAlefHamza, p.FoldAlefWasla, p.FoldHamzaSeat = false, false, false
			})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveProfile(tt.profile, tt.overrides)
			if err != nil {
				t.Fatalf("resolveProfile: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveProfile(%q, %+v) = %+v; want %+v", tt.profile, tt.overrides, got, tt.want)
			}
		})
	}
}

func TestResolveProfileRejectsUnknownName(t *testing.T) {
	if _, err := resolveProfile("bogus", profileOverrides{}); err == nil {
		t.Fatal("resolveProfile(\"bogus\", ...) returned no error")
	}
}

func TestRunUnknownProfileExitsWithError(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--profile=bogus", "x"}, strings.NewReader("x\n"), &stdout, &stderr)
	if code != 2 || stderr.Len() == 0 {
		t.Fatalf("code=%d stderr=%q; want exit 2 with an error message", code, stderr.String())
	}
}

// TestRunProfileFlagChangesMatching is an end-to-end check that --profile
// actually reaches the matcher: a heh-spelled query matches a ta-marbuta
// haystack line under the default profile (which folds the two together)
// but not under --profile=strict (which keeps them apart), and
// --keep-tamarbuta reproduces the same effect from the default profile.
func TestRunProfileFlagChangesMatching(t *testing.T) {
	input := "مدرسة\n" // ta-marbuta
	query := "مدرسه"   // heh

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"default profile folds ta-marbuta to heh: matches", []string{query}, 0},
		{"profile=strict keeps them apart: no match", []string{"--profile=strict", query}, 1},
		{"keep-tamarbuta on the default profile: no match", []string{"--keep-tamarbuta", query}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if got := run(tt.args, strings.NewReader(input), &stdout, &stderr); got != tt.want {
				t.Fatalf("exit code = %d; want %d (stderr=%q)", got, tt.want, stderr.String())
			}
		})
	}
}
