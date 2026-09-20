package match

import (
	"errors"
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
)

func TestNewLiteralRejectsEmptyKey(t *testing.T) {
	for _, query := range []string{"", "َّ", "ــ"} {
		if _, err := NewLiteral(query); !errors.Is(err, arabic.ErrEmptyKey) {
			t.Fatalf("NewLiteral(%q) error = %v; want errors.Is(err, arabic.ErrEmptyKey)", query, err)
		}
	}
}

func TestNewLiteralRejectsInvalidUTF8(t *testing.T) {
	if _, err := NewLiteral(string([]byte{'x', 0xff})); err == nil {
		t.Fatal("NewLiteral(invalid UTF-8) returned no error")
	}
}

func TestNewLiteralString(t *testing.T) {
	m, err := NewLiteral("مَدْرَسَةٌ")
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	if got := m.String(); got != "مَدْرَسَةٌ" {
		t.Fatalf("String() = %q; want the original, unnormalized query", got)
	}
}

func TestFindAllNormalizesQueryAndHaystack(t *testing.T) {
	// Query is voweled; haystack (already normalized by the caller, as scan
	// would do) is not. They must still collapse to the same key.
	m, err := NewLiteral("مَدْرَسَةٌ")
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	normalized := arabic.Normalize("مدرسه")
	spans := m.FindAll(normalized)
	if len(spans) != 1 || spans[0] != (Span{Start: 0, End: len(normalized)}) {
		t.Fatalf("FindAll(%q) = %v; want one span covering the whole string", normalized, spans)
	}
}

func TestFindAllMultipleNonOverlapping(t *testing.T) {
	m, err := NewLiteral("aa")
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	// "aaaa" contains "aa" at offsets 0 and 2 under non-overlapping search.
	spans := m.FindAll("aaaa")
	want := []Span{{0, 2}, {2, 4}}
	if len(spans) != len(want) || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("FindAll(\"aaaa\") = %v; want %v", spans, want)
	}
}

func TestFindAllNoMatch(t *testing.T) {
	m, err := NewLiteral("missing")
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	if spans := m.FindAll("present"); spans != nil {
		t.Fatalf("FindAll(no match) = %v; want nil", spans)
	}
}

func FuzzNewLiteralNoPanic(f *testing.F) {
	f.Add("query")
	f.Add("")
	f.Add("مدرسه")
	f.Add(string([]byte{0xff, 0xfe}))

	f.Fuzz(func(t *testing.T, query string) {
		m, err := NewLiteral(query)
		if err != nil {
			return
		}
		_ = m.FindAll(arabic.Normalize(query))
		_ = m.FindAll("")
		_ = m.String()
	})
}
