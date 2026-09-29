package scan

import (
	"errors"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
	simdutf8 "github.com/segmentio/asm/utf8"
)

// Embedding only Matcher keeps the original per-line path available as an
// independent reference for the summary specialization.
type matcherWithoutRaw struct{ match.Matcher }

func TestSummarySearchMatchesPerLineSearch(t *testing.T) {
	long := strings.Repeat("س", 40<<10)
	nearBlockEnd := strings.Repeat("a", (64<<10)-1) + "مكتبة\n"
	cases := []struct {
		name, input, query string
		existence          bool
	}{
		{"many rejected lines", strings.Repeat("سطر بلا نتيجة\n", 6000), "غيرموجود", false},
		{"sparse raw and folded hits", strings.Repeat("بلا نتيجة\r\n", 5000) + "مكتبة\r\nمكتبه\n", "مكتبه", false},
		{"long line and boundary", long + "\n" + nearBlockEnd + "خاتمة", "مكتبة", false},
		{"hit before invalid tail", "مكتبة\n" + strings.Repeat("سطر\n", 8000) + string([]byte{0xff}) + "\n", "مكتبة", true},
		{"invalid rejected group", strings.Repeat("سطر\n", 3000) + string([]byte{0xff}) + "\n", "غيرموجود", false},
		{"split rune", strings.Repeat("a", (64<<10)-1) + "مكتبة\n", "مكتبة", false},
		{"unterminated final hit", "لا شيء\nمكتبة", "مكتبة", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := mustLiteral(t, tc.query)
			opts := Options{File: "input.txt", OmitText: true, ExistenceOnly: tc.existence}
			run := func(m match.Matcher) (bool, []Match, error) {
				var got []Match
				found, err := Search(strings.NewReader(tc.input), m, opts, func(mt Match) error {
					got = append(got, mt)
					return nil
				})
				return found, got, err
			}
			wantFound, wantMatches, wantErr := run(matcherWithoutRaw{m})
			gotFound, gotMatches, gotErr := run(m)
			if gotFound != wantFound || !reflect.DeepEqual(gotMatches, wantMatches) ||
				!sameSummaryError(gotErr, wantErr) {
				t.Fatalf("fast=(%v, %+v, %v), reference=(%v, %+v, %v)",
					gotFound, gotMatches, gotErr, wantFound, wantMatches, wantErr)
			}
		})
	}
}

func TestSummarySearchPropagatesCallbackError(t *testing.T) {
	want := errors.New("output failed")
	m := mustLiteral(t, "مكتبة")
	found, err := Search(strings.NewReader("مكتبة\n"), m, Options{OmitText: true}, func(Match) error {
		return want
	})
	if found || !errors.Is(err, want) {
		t.Fatalf("found=%v err=%v; want callback error", found, err)
	}
}

type failingTailReader struct {
	data []byte
	err  error
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, nil }

func TestSummarySearchNoProgressReader(t *testing.T) {
	m := mustLiteral(t, "مكتبة")
	for _, candidate := range []match.Matcher{m, matcherWithoutRaw{m}} {
		found, err := Search(emptyReader{}, candidate, Options{OmitText: true}, func(Match) error {
			return nil
		})
		if found || !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("found=%v err=%v; want io.ErrNoProgress", found, err)
		}
	}
}

func (r *failingTailReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestSummarySearchReadErrorMatchesPerLine(t *testing.T) {
	m := mustLiteral(t, "مكتبة")
	input := []byte(strings.Repeat("سطر\n", 14000) + "مكتبة\npartial")
	wantErr := errors.New("read failed")
	run := func(m match.Matcher) (bool, []Match, error) {
		var got []Match
		found, err := Search(&failingTailReader{data: input, err: wantErr}, m,
			Options{OmitText: true}, func(mt Match) error {
				got = append(got, mt)
				return nil
			})
		return found, got, err
	}
	wantFound, wantMatches, referenceErr := run(matcherWithoutRaw{m})
	gotFound, gotMatches, gotErr := run(m)
	if gotFound != wantFound || !reflect.DeepEqual(gotMatches, wantMatches) ||
		!errors.Is(gotErr, wantErr) || !errors.Is(referenceErr, wantErr) {
		t.Fatalf("fast=(%v, %+v, %v), reference=(%v, %+v, %v)",
			gotFound, gotMatches, gotErr, wantFound, wantMatches, referenceErr)
	}
}

func sameSummaryError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}

func FuzzSummaryMatchesPerLine(f *testing.F) {
	f.Add([]byte("مكتبة\nﻛﺘﺎﺏ\nمَكْتَبَة\n"), "مكتبه", false, uint8(0))
	f.Add([]byte("غ\n"+string([]byte{0xff})+"\n"), "غ", true, uint8(1))
	f.Add([]byte(strings.Repeat("س", 1<<16)+"\nغ\n"), "غ", false, uint8(2))
	profiles := [...]arabic.Profile{arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose}
	f.Fuzz(func(t *testing.T, data []byte, query string, existence bool, profileIndex uint8) {
		if len(data) > 1<<17 || len(query) > 128 {
			return
		}
		m, err := match.NewLiteral(query, profiles[int(profileIndex)%len(profiles)])
		if err != nil {
			return
		}
		opts := Options{OmitText: true, ExistenceOnly: existence}
		run := func(m match.Matcher) (bool, []Match, error) {
			var got []Match
			found, err := Search(strings.NewReader(string(data)), m, opts, func(mt Match) error {
				got = append(got, mt)
				return nil
			})
			return found, got, err
		}
		wantFound, wantMatches, wantErr := run(matcherWithoutRaw{m})
		gotFound, gotMatches, gotErr := run(m)
		if gotFound != wantFound || !reflect.DeepEqual(gotMatches, wantMatches) ||
			!sameSummaryError(gotErr, wantErr) {
			t.Fatalf("fast=(%v, %+v, %v), reference=(%v, %+v, %v)",
				gotFound, gotMatches, gotErr, wantFound, wantMatches, wantErr)
		}
	})
}

func TestSIMDUTF8ValidationMatchesStandard(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 10000; i++ {
		var data []byte
		if i%2 == 0 {
			target := rng.Intn(512)
			for len(data) < target {
				r := rune(rng.Intn(utf8.MaxRune + 1))
				if !utf8.ValidRune(r) {
					continue
				}
				data = utf8.AppendRune(data, r)
			}
		} else {
			data = make([]byte, rng.Intn(512))
			_, _ = rng.Read(data)
		}
		if got, want := simdutf8.Valid(data), utf8.Valid(data); got != want {
			t.Fatalf("SIMD UTF-8 validation differs for %x: got %v, want %v", data, got, want)
		}
	}
}
