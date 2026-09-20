package arabic

import "testing"

// zwnj and zwj are built from their codepoints, not embedded as literal
// invisible bytes in a string literal, so the source stays auditable
// (staticcheck ST1018 flags a string literal that contains a raw format
// character).
var (
	zwnj = string(rune(0x200C)) // ZERO WIDTH NON-JOINER
	zwj  = string(rune(0x200D)) // ZERO WIDTH JOINER
)

func TestStripJoiners(t *testing.T) {
	p := Profile{StripJoiners: true}
	input := "م" + zwnj + "ر" + zwj + "حبا" // م ZWNJ ر ZWJ حبا
	want := "مرحبا"
	if got := p.Normalize(input); got != want {
		t.Fatalf("Normalize(%q) = %q; want %q", input, got, want)
	}

	// Off by default (zero value).
	if got := (Profile{}).Normalize(input); got != input {
		t.Fatalf("Profile{}.Normalize(%q) = %q; want unchanged", input, got)
	}
}

func TestStripBidi(t *testing.T) {
	tests := []struct {
		name string
		mark rune
	}{
		{"ALM", '؜'},
		{"LRM", '‎'},
		{"RLM", '‏'},
		{"LRE", '‪'},
		{"RLE", '‫'},
		{"PDF", '‬'},
		{"LRO", '‭'},
		{"RLO", '‮'},
		{"LRI", '⁦'},
		{"RLI", '⁧'},
		{"FSI", '⁨'},
		{"PDI", '⁩'},
	}
	p := Profile{StripBidi: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "a" + string(tt.mark) + "b"
			if got := p.Normalize(input); got != "ab" {
				t.Fatalf("Normalize(%q) = %q; want %q", input, got, "ab")
			}
		})
	}

	alm := string(rune(0x061C)) // ARABIC LETTER MARK
	if got := (Profile{}).Normalize("a" + alm + "b"); got != "a"+alm+"b" {
		t.Fatalf("Profile{}.Normalize did not leave ALM untouched: %q", got)
	}
}

func TestStripQuranic(t *testing.T) {
	p := Profile{StripQuranic: true}
	tests := []struct {
		name string
		mark rune
		isMn bool // documents which category this member of the range belongs to
	}{
		{"SMALL HIGH LIGATURE SAD WITH LAM WITH ALEF MAKSURA", 'ۖ', true},
		{"SMALL HIGH MEEM INITIAL FORM", 'ۘ', true},
		{"END OF AYAH (Cf, not Mn)", '۝', false},
		{"START OF RUB EL HIZB (So, not Mn)", '۞', false},
		{"SMALL WAW (Lm, not Mn)", 'ۥ', false},
		{"SMALL YEH (Lm, not Mn)", 'ۦ', false},
		{"PLACE OF SAJDAH (So, not Mn)", '۩', false},
		{"SMALL LOW MEEM (last in range)", 'ۭ', true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "a" + string(tt.mark) + "b"
			if got := p.Normalize(input); got != "ab" {
				t.Fatalf("StripQuranic did not strip %U: Normalize(%q) = %q", tt.mark, input, got)
			}
			// The non-Mn members are the whole point of StripQuranic being
			// its own range check rather than riding on TashkilScope: a
			// generic Mn-based strip would never catch them.
			if !tt.isMn {
				generic := (Profile{StripTashkil: true, TashkilScope: TashkilAllMn}).Normalize(input)
				if generic == "ab" {
					t.Fatalf("%U turned out to be Mn after all; the non-Mn test premise is stale", tt.mark)
				}
			}
		})
	}

	// One before the range and one after: must survive even with
	// StripQuranic on.
	if got := p.Normalize("ە"); got != "ە" {
		t.Fatalf("StripQuranic touched U+06D5, one below the range: %q", got)
	}
	if got := p.Normalize("ۮ"); got != "ۮ" {
		t.Fatalf("StripQuranic touched U+06EE, one above the range: %q", got)
	}
}

func TestFoldDigits(t *testing.T) {
	p := Profile{FoldDigits: true}
	if got := p.Normalize("٠١٢٣٤٥٦٧٨٩"); got != "0123456789" {
		t.Fatalf("Arabic-Indic digits: got %q", got)
	}
	if got := p.Normalize("۰۱۲۳۴۵۶۷۸۹"); got != "0123456789" {
		t.Fatalf("Extended Arabic-Indic digits: got %q", got)
	}
	if got := (Profile{}).Normalize("٠١"); got != "٠١" {
		t.Fatalf("digits folded with FoldDigits off: %q", got)
	}
}

func TestFoldPunctuation(t *testing.T) {
	p := Profile{FoldPunctuation: true}
	tests := []struct {
		name, input, want string
	}{
		{"Arabic comma", "،", ","},
		{"Arabic semicolon", "؛", ";"},
		{"Arabic question mark", "؟", "?"},
		{"Arabic percent sign", "٪", "%"},
		{"Arabic decimal separator", "٫", "."},
		{"Arabic thousands separator", "٬", ","},
		{"Arabic full stop", "۔", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Normalize(tt.input); got != tt.want {
				t.Fatalf("Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
			if got := (Profile{}).Normalize(tt.input); got != tt.input {
				t.Fatalf("folded with FoldPunctuation off: Normalize(%q) = %q", tt.input, got)
			}
		})
	}
}

// TestPhase4RulesComposeWithHamzaLogic is a direct check that the new
// drop cases (joiners here; bidi and Quranic marks follow the identical
// code path) integrate correctly with the hamza-lookback logic from
// Phase 3: dropping one of them between a hamza-carrying letter and its
// hamza mark must not break the fold decision, the same way a dropped
// tatweel doesn't (see the "drop || Mn" branch in Normalize's
// context-tracking switch) — this is what actually distinguishes "was this
// bug fixed generally" from "was it fixed only for the tatweel case that
// happened to be fuzzed first".
func TestPhase4RulesComposeWithHamzaLogic(t *testing.T) {
	p := ProfileSearch // FoldAlefHamza and StripJoiners both on

	// Already-NFD-decomposed alef + ZWJ + combining hamza-above: an
	// unusual but well-formed sequence. With StripJoiners on, the ZWJ
	// must vanish AND the hamza mark that follows it must still resolve
	// as attached to the alef before it (dropping FoldAlefHamza), exactly
	// as it would with no ZWJ there at all.
	withJoiner := p.Normalize("ا" + zwj + "ٔ")
	withoutJoiner := p.Normalize("أ")
	if withJoiner != "ا" || withoutJoiner != "ا" || withJoiner != withoutJoiner {
		t.Fatalf("dropped joiner broke hamza-fold context: with ZWJ = %q, without = %q; want both %q",
			withJoiner, withoutJoiner, "ا")
	}

	// And with StripJoiners off, the ZWJ survives and — being neither Mn
	// nor alef/waw/yeh — correctly breaks the context, so the hamza mark
	// now falls to "unrecognized context" and is only removed by
	// StripTashkil, not by FoldAlefHamza.
	kept := Profile{FoldAlefHamza: true, StripJoiners: false}.Normalize("ا" + zwj + "ٔ")
	if kept == "ا" {
		t.Fatalf("a kept ZWJ should have broken hamza-fold context, but got %q", kept)
	}
}
