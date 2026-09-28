package arabic

import "testing"

func TestRasmGroups(t *testing.T) {
	p := ProfileStrict
	p.Rasm = true
	tests := []struct {
		input string
		want  string
	}{
		{"بتثني", "ٮٮٮٮٮ"},
		{"جحخ", "ححح"},
		{"دذ رز سش صض طظ عغ فق", "دد رر سس صص طط عع ٯٯ"},
	}
	for _, tt := range tests {
		if got := p.Normalize(tt.input); got != tt.want {
			t.Fatalf("Normalize(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestRasmRunsAfterLanguageFolds(t *testing.T) {
	p := ProfileStrict
	p.Languages = LanguageArabic | LanguagePersian
	p.Rasm = true
	if got := p.Normalize("یي"); got != "ٮٮ" {
		t.Fatalf("Normalize mixed Yeh = %q; want %q", got, "ٮٮ")
	}
}

func TestRasmPreservesLanguageSpecificLetters(t *testing.T) {
	p := ProfileStrict
	p.Languages = LanguageAll
	p.Rasm = true
	input := "ٹڈڑ ښږځڅډړټڼۍ ڵڕۆێ ېۈۆۇ"
	if got := p.Normalize(input); got != input {
		t.Fatalf("Normalize(%q) = %q; language-specific letters changed", input, got)
	}
}

func TestRasmMappedMatchesDirect(t *testing.T) {
	p := ProfileSearch
	p.Rasm = true
	input := "مُسْتَشْرِق"
	direct := p.Normalize(input)
	mapped, idx := p.NormalizeMapped(input)
	if mapped != direct {
		t.Fatalf("NormalizeMapped key = %q; Normalize = %q", mapped, direct)
	}
	if len(idx) != len(mapped)+1 || idx[len(mapped)] != int32(len(input)) {
		t.Fatalf("invalid mapping: len=%d final=%d", len(idx), idx[len(mapped)])
	}
}

func TestRasmYehHamzaIsIdempotent(t *testing.T) {
	p := ProfileStrict
	p.Rasm = true
	once := p.Normalize("ئ")
	if once != "ٮ" {
		t.Fatalf("Normalize(ئ) = %q; want ٮ", once)
	}
	if twice := p.Normalize(once); twice != once {
		t.Fatalf("rasm normalization is not idempotent: %q then %q", once, twice)
	}
}
