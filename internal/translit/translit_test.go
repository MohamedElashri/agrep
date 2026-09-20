package translit

import (
	"testing"
	"unicode/utf8"
)

func TestFromLatin(t *testing.T) {
	for _, tt := range []struct {
		name   string
		scheme Scheme
		input  string
		want   string
	}{
		{"buckwalter", Buckwalter, "ktAb", "كتاب"},
		{"arabtex", ArabTeX, "kitAb", "كِتاب"},
		{"iso233", ISO233, "kitāb", "كِتاب"},
		{"arabtex longest token", ArabTeX, "^sams", "شَمس"},
		{"unknown copied", Buckwalter, "1!", "1!"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromLatin(tt.input, tt.scheme); got != tt.want {
				t.Fatalf("FromLatin(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromLatinPreservesInvalidUTF8ForCallerValidation(t *testing.T) {
	input := string([]byte{0xff, 'k'})
	if got := FromLatin(input, Buckwalter); got != input {
		t.Fatalf("FromLatin changed invalid UTF-8: got %x want %x", got, input)
	}
}

func TestToLatinRoundTrip(t *testing.T) {
	for _, scheme := range []Scheme{Buckwalter, ArabTeX, ISO233} {
		const input = "كِتَاب"
		latin, boundaries := ToLatin(input, scheme)
		if got := FromLatin(latin, scheme); got != input {
			t.Fatalf("scheme=%s latin=%q round trip=%q; want %q", scheme, latin, got, input)
		}
		start, end, ok := MapSpan(boundaries, 0, len(input))
		if !ok || start != 0 || end != len(latin) {
			t.Fatalf("scheme=%s mapped=[%d,%d], ok=%v; want [0,%d]", scheme, start, end, ok, len(latin))
		}
	}
}

func TestToLatinMappedSpan(t *testing.T) {
	got, boundaries := ToLatin("قبل كتاب بعد", Buckwalter)
	start, end, ok := MapSpan(boundaries, len("قبل "), len("قبل كتاب"))
	if got != "qbl ktAb bEd" || !ok || got[start:end] != "ktAb" {
		t.Fatalf("got=%q span=[%d,%d] text=%q ok=%v", got, start, end, got[start:end], ok)
	}
}

func FuzzTransliterationValidMapping(f *testing.F) {
	f.Add("كِتَاب")
	f.Add("ﻻ يوجد")
	f.Add("kitāb")
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip()
		}
		for _, scheme := range []Scheme{Buckwalter, ArabTeX, ISO233} {
			arabic := FromLatin(input, scheme)
			if !utf8.ValidString(arabic) {
				t.Fatalf("FromLatin returned invalid UTF-8 under %s", scheme)
			}
			rendered, boundaries := ToLatin(input, scheme)
			if !utf8.ValidString(rendered) || len(boundaries) != len(input)+1 {
				t.Fatalf("ToLatin mapping invalid under %s", scheme)
			}
			for i := 1; i < len(boundaries); i++ {
				if boundaries[i] < boundaries[i-1] {
					t.Fatalf("ToLatin mapping is not monotonic under %s", scheme)
				}
			}
		}
	})
}
