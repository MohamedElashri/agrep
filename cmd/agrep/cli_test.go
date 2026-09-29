package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	unicodeencoding "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func TestRunJSONLines(t *testing.T) {
	input := "ignore\nالمَدِينَة <tag>\n"
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "مدينه"}, strings.NewReader(input), &stdout, &stderr)
	want := "{\"line\":2,\"text\":\"المَدِينَة <tag>\",\"spans\":[[4,20]]}\n"
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
	path := filepath.Join(t.TempDir(), "input.txt")
	writeTestFile(t, path, "x\n")
	var stderr strings.Builder
	if code := run([]string{"x", path}, strings.NewReader(""), failingWriter{}, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "broken output") {
		t.Fatalf("stderr = %q; want buffered write failure", stderr.String())
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

func TestRunFlushesBufferedOutputBeforeInputError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	writeTestFile(t, path, "hit\n"+string([]byte{0xff})+"\n")
	var stdout, stderr strings.Builder
	if code := run([]string{"hit", path}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
	if stdout.String() != "hit\n" || !strings.Contains(stderr.String(), "not valid UTF-8") {
		t.Fatalf("stdout=%q stderr=%q; want emitted line and input error", stdout.String(), stderr.String())
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type outputAwareReader struct {
	written *bool
	sent    bool
}

func (r *outputAwareReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "hit\n"), nil
	}
	if !*r.written {
		return 0, errors.New("stdin output was delayed")
	}
	return 0, io.EOF
}

type outputAwareWriter struct {
	written *bool
	text    strings.Builder
}

func (w *outputAwareWriter) Write(p []byte) (int, error) {
	*w.written = true
	return w.text.Write(p)
}

func TestRunStreamsStdinOutputBeforeNextRead(t *testing.T) {
	written := false
	input := &outputAwareReader{written: &written}
	output := &outputAwareWriter{written: &written}
	var stderr strings.Builder
	if code := run([]string{"hit"}, input, output, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%q; want successful streaming search", code, stderr.String())
	}
	if output.text.String() != "hit\n" {
		t.Fatalf("output = %q, want hit line", output.text.String())
	}
}

// withOverride returns a copy of base with mutate applied, so each test
// table row below can say precisely which fields it expects to change from
// the selected preset, instead of writing out a full Profile literal (and
// risking a transcription mistake in fields it didn't mean to
// touch).
func withOverride(base arabic.Profile, mutate func(*arabic.Profile)) arabic.Profile {
	p := base
	mutate(&p)
	return p
}

// TestResolveProfile checks that each --keep-*/--fold-* flag changes only its
// intended field relative to the selected preset, both alone and combined.
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
		{"rasm alone", "search", profileOverrides{rasm: true},
			withOverride(arabic.ProfileSearch, func(p *arabic.Profile) { p.Rasm = true })},

		{"all overrides together",
			"search",
			profileOverrides{
				keepHamza: true, keepTaMarbuta: true, keepTashkil: true,
				keepPresentationForms: true, keepJoiners: true, keepBidiMarks: true, keepQuranicMarks: true,
				foldDigits: true, foldPunctuation: true, rasm: true,
			},
			// StripTatweel and FoldAlefMaksura have no --keep-* flag, so
			// they must survive from the preset untouched: asserting the
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
				p.Rasm = true
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

func TestRunLanguageSelectionChangesMatching(t *testing.T) {
	input := "کتاب\n"
	query := "كتاب"
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"default Arabic preserves Persian kaf", []string{query}, 1},
		{"Persian alone preserves Arabic and Persian kaf", []string{"--lang=fa", query}, 1},
		{"Arabic and Persian cross-fold shared letters", []string{"--lang=ar,fa", query}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tt.args, strings.NewReader(input), &stdout, &stderr)
			if code != tt.want || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q; want code %d", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestRunRejectsUnknownLanguage(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--lang=xx", "x"}, strings.NewReader("x\n"), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "unknown language") {
		t.Fatalf("code=%d stderr=%q; want exit 2 with an unknown-language error", code, stderr.String())
	}
}

func TestRunLanguageFoldMapsOriginalSpan(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--lang=ar,fa", "--json", "كتاب"}, strings.NewReader("کتاب\n"), &stdout, &stderr)
	want := "{\"line\":1,\"text\":\"کتاب\",\"spans\":[[0,8]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunPersianWordBoundaryKeepsZWNJInsideWord(t *testing.T) {
	var stdout, stderr strings.Builder
	input := "می\u200cروم\nمی،\n"
	code := run([]string{"--lang=fa", "-w", "می"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 || stdout.String() != "می،\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
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

func TestRunRepeatedPatternsAndUnicodeIgnoreCase(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-i", "-e", "STRASSE", "-e", "missing"}, strings.NewReader("Straße\nother\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "Straße\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunInvertCount(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-v", "-c", "hit"}, strings.NewReader("hit\nmiss\nother\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "2\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunContextAndGroups(t *testing.T) {
	var stdout, stderr strings.Builder
	input := "hit\na\nb\nc\nhit\n"
	code := run([]string{"-n", "-A", "1", "hit"}, strings.NewReader(input), &stdout, &stderr)
	want := "1:hit\n2-a\n--\n5:hit\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunMultipleFilesKeepsArgumentOrder(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	writeTestFile(t, a, "hit a\n")
	writeTestFile(t, b, "hit b\n")

	var stdout, stderr strings.Builder
	code := run([]string{"--threads=2", "hit", b, a}, strings.NewReader(""), &stdout, &stderr)
	want := b + ":hit b\n" + a + ":hit a\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunRecursiveHonorsIgnoreAndInclude(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "ignored.txt\n")
	writeTestFile(t, filepath.Join(root, "found.txt"), "hit\n")
	writeTestFile(t, filepath.Join(root, "ignored.txt"), "hit\n")
	writeTestFile(t, filepath.Join(root, "skip.md"), "hit\n")

	var stdout, stderr strings.Builder
	code := run([]string{"-r", "--include=*.txt", "hit", root}, strings.NewReader(""), &stdout, &stderr)
	want := filepath.Join(root, "found.txt") + ":hit\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-r", "--no-ignore", "--include=*.txt", "hit", root}, strings.NewReader(""), &stdout, &stderr)
	want += filepath.Join(root, "ignored.txt") + ":hit\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("no-ignore: code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunFileSelectionModesAndFilenameOverrides(t *testing.T) {
	root := t.TempDir()
	hit := filepath.Join(root, "hit.txt")
	miss := filepath.Join(root, "miss.txt")
	writeTestFile(t, hit, "needle\n")
	writeTestFile(t, miss, "haystack\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"files with matches", []string{"-l", "needle", hit, miss}, hit + "\n"},
		{"files without matches", []string{"-L", "needle", hit, miss}, miss + "\n"},
		{"suppress filename", []string{"-h", "needle", hit, miss}, "needle\n"},
		{"force filename", []string{"-H", "needle", hit}, hit + ":needle\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 || stdout.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), tt.want, stderr.String())
			}
		})
	}
}

func TestRunFilenameModesReportErrorsAfterMatch(t *testing.T) {
	root := t.TempDir()
	invalid := filepath.Join(root, "invalid.txt")
	writeTestFile(t, invalid, "hit\n"+string([]byte{0xff})+"\n")
	long := filepath.Join(root, "long.txt")
	writeTestFile(t, long, "hit\nabcdef\n")
	for _, flag := range []string{"-l", "-L"} {
		for _, tt := range []struct {
			name string
			args []string
		}{
			{"invalid UTF-8", []string{flag, "hit", invalid}},
			{"line limit", []string{flag, "--max-line-bytes=5", "hit", long}},
		} {
			t.Run(flag+"/"+tt.name, func(t *testing.T) {
				var stdout, stderr strings.Builder
				if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			})
		}
	}
}

func TestRunJSONFileField(t *testing.T) {
	name := filepath.Join(t.TempDir(), "input.txt")
	writeTestFile(t, name, "needle\n")
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "needle", name}, strings.NewReader(""), &stdout, &stderr)
	want := "{\"file\":" + strconvQuote(name) + ",\"line\":1,\"text\":\"needle\",\"spans\":[[0,6]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunJSONNeverContainsColorControls(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "--color=always", "مدينه"}, strings.NewReader("مَدِينَة\n"), &stdout, &stderr)
	if code != 0 || strings.Contains(stdout.String(), "\x1b[") || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunOnlyMatchingUsesOriginalText(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-o", "مدينه"}, strings.NewReader("هذه مَدِينَة جميلة\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "مَدِينَة\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunRegexUsesNormalizedText(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"normalized expression matches", []string{"--regex", `^مدرس[هة]$`}, 0},
		{"raw ta-marbuta does not match default folded text", []string{"--regex", `ة`}, 1},
		{"raw ta-marbuta matches strict text", []string{"--regex", "--profile=strict", `ة`}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := run(tt.args, strings.NewReader("مَدْرَسَة\n"), &stdout, &stderr); code != tt.want {
				t.Fatalf("code=%d want=%d stdout=%q stderr=%q", code, tt.want, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunRasmMatching(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want int
	}{
		{"default keeps dots", []string{"بنت"}, "ثني\n", 1},
		{"rasm folds ijam", []string{"--rasm", "بنت"}, "ثني\n", 0},
		{"loose includes rasm", []string{"--profile=loose", "بنت"}, "ثني\n", 0},
		{"Urdu retroflex remains distinct", []string{"--rasm", "--profile=strict", "--lang=ur", "ت"}, "ٹ\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tt.args, strings.NewReader(tt.in), &stdout, &stderr)
			if code != tt.want || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q; want %d", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestRunFuzzyMatchingAndMappedSpan(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--fuzzy", "--json", "كتاب"}, strings.NewReader("هذا كتااب جيد\n"), &stdout, &stderr)
	want := "{\"line\":1,\"text\":\"هذا كتااب جيد\",\"spans\":[[7,17]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--fuzzy", "--json", "كتاب"}, strings.NewReader("هذا كُتب جيد\n"), &stdout, &stderr)
	want = "{\"line\":1,\"text\":\"هذا كُتب جيد\",\"spans\":[[7,15]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("deletion span: code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--fuzzy=0", "كتاب"}, strings.NewReader("كتلب\n"), &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("distance zero: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunFuzzySpanComposesWithTransliterationOutput(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--fuzzy", "--translit=buckwalter", "--translit-out", "--json", "ktAb"}, strings.NewReader("كتااب\n"), &stdout, &stderr)
	want := "{\"line\":1,\"text\":\"ktAAb\",\"spans\":[[0,5]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunRejectsInvalidFuzzyOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--fuzzy=-1", "x"},
		{"--fuzzy=bad", "x"},
		{"--fuzzy", "--regex", "x"},
	} {
		var stdout, stderr strings.Builder
		if code := run(args, strings.NewReader("x\n"), &stdout, &stderr); code != 2 || stderr.Len() == 0 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunWordRegexpUsesOriginalText(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-w", "كتاب"}, strings.NewReader("كتابه\nكتاب،\nالــكتاب\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "كتاب،\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunColorUsesRTLIsolates(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--color=always", "مدينه"}, strings.NewReader("هذه مَدِينَة جميلة\n"), &stdout, &stderr)
	rli, pdi := string(rune(0x2067)), string(rune(0x2069))
	want := "هذه " + rli + ansiMatchStart + "مَدِينَة" + ansiMatchEnd + pdi + " جميلة\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--color=always", "--no-bidi-isolate", "مدينه"}, strings.NewReader("هذه مَدِينَة جميلة\n"), &stdout, &stderr)
	want = "هذه " + ansiMatchStart + "مَدِينَة" + ansiMatchEnd + " جميلة\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("no-isolate: code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunColorFullyVoweledQuranicLine(t *testing.T) {
	var stdout, stderr strings.Builder
	input := "قَالَ ٱللَّهُۖ غَفُورٌ\n"
	code := run([]string{"--color=always", "الله"}, strings.NewReader(input), &stdout, &stderr)
	rli, pdi := string(rune(0x2067)), string(rune(0x2069))
	want := "قَالَ " + rli + ansiMatchStart + "ٱللَّهُۖ" + ansiMatchEnd + pdi + " غَفُورٌ\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunLegacyEncodingsUseDecodedUTF8Spans(t *testing.T) {
	const input = "هذه مَدِينَة جميلة\n"
	tests := []struct {
		name    string
		flag    string
		encoder *encoding.Encoder
	}{
		{"cp1256", "cp1256", charmap.Windows1256.NewEncoder()},
		{"iso-8859-6", "iso-8859-6", charmap.ISO8859_6.NewEncoder()},
		{"utf16le", "utf16le", unicodeencoding.UTF16(unicodeencoding.LittleEndian, unicodeencoding.UseBOM).NewEncoder()},
		{"utf16be", "utf16be", unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.IgnoreBOM).NewEncoder()},
		{"auto cp1256", "auto", charmap.Windows1256.NewEncoder()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _, err := transform.Bytes(tt.encoder, []byte(input))
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr strings.Builder
			code := run([]string{"--json", "--encoding=" + tt.flag, "مدينه"}, bytes.NewReader(raw), &stdout, &stderr)
			want := "{\"line\":1,\"text\":\"هذه مَدِينَة جميلة\",\"spans\":[[7,23]]}\n"
			if code != 0 || stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
			}
		})
	}
}

func TestRunInvalidUTF8SuggestsAutoEncoding(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"x"}, bytes.NewReader([]byte{'x', 0xff, '\n'}), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--encoding=auto") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunTransliteratedQueries(t *testing.T) {
	for _, tt := range []struct {
		scheme string
		query  string
	}{
		{"buckwalter", "ktAb"},
		{"arabtex", "kitAb"},
		{"iso233", "kitāb"},
	} {
		t.Run(tt.scheme, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run([]string{"--translit=" + tt.scheme, tt.query}, strings.NewReader("هذا كتاب\n"), &stdout, &stderr)
			if code != 0 || stdout.String() != "هذا كتاب\n" || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunTranslitOutputRemapsSpans(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "--translit-out", "كتاب"}, strings.NewReader("كتاب\n"), &stdout, &stderr)
	want := "{\"line\":1,\"text\":\"ktAb\",\"spans\":[[0,4]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("JSON: code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-o", "--translit-out", "كتاب"}, strings.NewReader("هذا كتاب جيد\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "ktAb\n" || stderr.Len() != 0 {
		t.Fatalf("only matching: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--color=always", "--translit-out", "كتاب"}, strings.NewReader("كتاب\n"), &stdout, &stderr)
	want = ansiMatchStart + "ktAb" + ansiMatchEnd + "\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("color: code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestRunRejectsInvalidSearchOutputOptions(t *testing.T) {
	tests := [][]string{
		{"--regex", "["},
		{"--color=bogus", "x"},
		{"-o", "-v", "x"},
		{"-o", "-A", "1", "x"},
	}
	for _, args := range tests {
		var stdout, stderr strings.Builder
		if code := run(args, strings.NewReader("x\n"), &stdout, &stderr); code != 2 {
			t.Fatalf("args=%v code=%d; want 2 (stdout=%q stderr=%q)", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunRejectsInvalidEncodingTransliterationOptions(t *testing.T) {
	tests := [][]string{
		{"--encoding=unknown", "x"},
		{"--translit=unknown", "x"},
		{"--regex", "--translit=buckwalter", "x"},
	}
	for _, args := range tests {
		var stdout, stderr strings.Builder
		if code := run(args, strings.NewReader("x\n"), &stdout, &stderr); code != 2 {
			t.Fatalf("args=%v code=%d; want 2 (stdout=%q stderr=%q)", args, code, stdout.String(), stderr.String())
		}
	}
}

func writeTestFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
