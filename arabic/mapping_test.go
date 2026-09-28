package arabic

import (
	"testing"
	"unicode/utf8"
)

func TestNormalizeMapped(t *testing.T) {
	tests := []struct {
		name, input string
		profile     Profile
	}{
		{"canonical expansion", "é", Profile{}},
		{"presentation ligature", "ﻻ", ProfileSearch},
		{"stripped marks", "مُدُ", ProfileSearch},
		{"tatweel inside", "العــربية", ProfileSearch},
		{"hamza canonical reorder", "أُ", ProfileStrict},
		{"presentation phrase", "ﻻ يوجد ﻛﺘﺎﺏ", ProfileSearch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, idx := tt.profile.NormalizeMapped(tt.input)
			if want := tt.profile.Normalize(tt.input); key != want {
				t.Fatalf("NormalizeMapped key = %q; Normalize = %q", key, want)
			}
			assertValidMapping(t, tt.input, key, idx)
		})
	}
}

func TestMapSpanCoversPresentationExpansion(t *testing.T) {
	input := "ﻻ"
	key, idx := ProfileSearch.NormalizeMapped(input)
	start, end := MapSpan(idx, 0, len("ل"))
	if key != "لا" || start != 0 || end != len(input) {
		t.Fatalf("key=%q mapped=[%d,%d); want key لا and [0,%d)", key, start, end, len(input))
	}
}

func TestMapSpanCoversTrailingStrippedMarks(t *testing.T) {
	input := "مُدُ next"
	key, idx := ProfileSearch.NormalizeMapped(input)
	start, end := MapSpan(idx, 0, len("مد"))
	wantEnd := len("مُدُ")
	if key != "مد next" || start != 0 || end != wantEnd {
		t.Fatalf("key=%q mapped=[%d,%d); want [0,%d)", key, start, end, wantEnd)
	}
}

func TestMapSpanCoversTatweelInside(t *testing.T) {
	input := "العــربية"
	key, idx := ProfileSearch.NormalizeMapped(input)
	start, end := MapSpan(idx, 0, len(key))
	if start != 0 || end != len(input) {
		t.Fatalf("mapped=[%d,%d); want [0,%d)", start, end, len(input))
	}
}

func TestNormalizeMappedIntoReusesIndexStorage(t *testing.T) {
	dst := make([]int32, 0, 64)
	_, first := ProfileSearch.NormalizeMappedInto("مدرسة", dst)
	ptr := &first[0]
	_, second := ProfileSearch.NormalizeMappedInto("كتاب", first[:0])
	if &second[0] != ptr {
		t.Fatal("NormalizeMappedInto did not reuse sufficient destination capacity")
	}
}

func FuzzNormalizeMappedMatchesNormalize(f *testing.F) {
	addProfileFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, s string, bits uint16, scope int) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		p := profileFromBits(bits, scope)
		key, idx := p.NormalizeMapped(s)
		if want := p.Normalize(s); key != want {
			t.Fatalf("NormalizeMapped(%q) = %q; Normalize = %q under %+v", s, key, want, p)
		}
		assertValidMapping(t, s, key, idx)
	})
}

func assertValidMapping(t testing.TB, input, key string, idx []int32) {
	t.Helper()
	if len(idx) != len(key)+1 {
		t.Fatalf("len(idx) = %d; want len(key)+1 = %d", len(idx), len(key)+1)
	}
	if got := idx[len(idx)-1]; got != int32(len(input)) {
		t.Fatalf("idx[len(key)] = %d; want %d", got, len(input))
	}
	for i, offset := range idx {
		if offset < 0 || offset > int32(len(input)) {
			t.Fatalf("idx[%d] = %d outside input", i, offset)
		}
		if i > 0 && offset < idx[i-1] {
			t.Fatalf("idx is not monotonic at %d: %d < %d", i, offset, idx[i-1])
		}
	}
}
