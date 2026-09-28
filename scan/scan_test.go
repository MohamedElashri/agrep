package scan

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
)

func mustLiteral(t testing.TB, query string) match.Matcher {
	t.Helper()
	m, err := match.NewLiteral(query, arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("match.NewLiteral(%q): %v", query, err)
	}
	return m
}

func TestSearchReturnsExactOriginalLines(t *testing.T) {
	input := "ignore\r\nهذِهِ مَدْرَسَةٌ\r\nمدرسه"
	m := mustLiteral(t, "مدرسه")

	var got []Match
	found, err := Search(strings.NewReader(input), m, Options{}, func(mt Match) error {
		got = append(got, mt)
		return nil
	})
	if err != nil || !found {
		t.Fatalf("Search() = found %v, err %v", found, err)
	}
	if len(got) != 2 || got[0].Line != 2 || got[1].Line != 3 {
		t.Fatalf("matches = %+v", got)
	}
	if got[0].Text != "هذِهِ مَدْرَسَةٌ" {
		t.Fatalf("original text changed: %q", got[0].Text)
	}
}

func TestSearchHandlesBlankAndLongLines(t *testing.T) {
	long := strings.Repeat("x", 17*1024*1024) + " أحمد"
	input := "\n" + long
	m := mustLiteral(t, "احمد")

	var got Match
	found, err := Search(strings.NewReader(input), m, Options{}, func(mt Match) error {
		got = mt
		return nil
	})
	if err != nil || !found || got.Line != 2 || got.Text != long {
		t.Fatalf("long-line search failed: found=%v line=%d err=%v", found, got.Line, err)
	}
}

func TestSearchLineLimit(t *testing.T) {
	m := mustLiteral(t, "1")
	_, err := Search(strings.NewReader("12345\n"), m, Options{MaxLineBytes: 4}, func(Match) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchLineLimitAllowsCRLFAtLimit(t *testing.T) {
	m := mustLiteral(t, "1")
	var got Match
	found, err := Search(strings.NewReader("1234\r\n"), m, Options{MaxLineBytes: 4}, func(mt Match) error {
		got = mt
		return nil
	})
	if err != nil || !found || got.Text != "1234" {
		t.Fatalf("found=%v match=%+v err=%v", found, got, err)
	}
}

func TestSearchRejectsInvalidUTF8(t *testing.T) {
	m := mustLiteral(t, "x")
	_, err := Search(strings.NewReader(string([]byte{'x', 0xff, '\n'})), m, Options{}, func(Match) error { return nil })
	if err == nil || !errors.Is(err, ErrInvalidUTF8) || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v", err)
	}
}

func TestRawRejectionPreservesSelectionAndErrors(t *testing.T) {
	m := mustLiteral(t, "غ")
	var got []Match
	found, err := Search(strings.NewReader("سطر\nغ\nآخر\n"), m,
		Options{InvertMatch: true, AfterContext: 1}, func(mt Match) error {
			got = append(got, mt)
			return nil
		})
	if err != nil || !found || len(got) != 3 {
		t.Fatalf("Search = found %v, err %v, matches %+v", found, err, got)
	}
	if got[0].Line != 1 || got[0].Context || got[1].Line != 2 || !got[1].Context ||
		got[2].Line != 3 || got[2].Context {
		t.Fatalf("inverted/context output changed: %+v", got)
	}

	if _, err := Search(strings.NewReader("abcdef\n"), m,
		Options{MaxLineBytes: 5}, func(Match) error { return nil }); err == nil {
		t.Fatal("raw rejection bypassed the line limit")
	}
	if _, err := Search(strings.NewReader(string([]byte{'x', 0xff, '\n'})), m,
		Options{}, func(Match) error { return nil }); !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("raw rejection bypassed UTF-8 validation: %v", err)
	}
}

func TestSearchPropagatesCallbackFailure(t *testing.T) {
	want := errors.New("write failed")
	m := mustLiteral(t, "match")
	_, err := Search(strings.NewReader("match\n"), m, Options{}, func(Match) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want %v", err, want)
	}
}

func TestSearchNoMatch(t *testing.T) {
	m := mustLiteral(t, "missing")
	called := false
	found, err := Search(strings.NewReader("one\ntwo\n"), m, Options{}, func(Match) error {
		called = true
		return nil
	})
	if err != nil || found || called {
		t.Fatalf("found=%v called=%v err=%v", found, called, err)
	}
}

func TestSearchInvertAndContext(t *testing.T) {
	m := mustLiteral(t, "hit")
	var got []Match
	found, err := Search(strings.NewReader("hit\none\ntwo\nhit\nfour\n"), m, Options{
		File:          "input.txt",
		InvertMatch:   true,
		BeforeContext: 1,
		AfterContext:  1,
	}, func(mt Match) error {
		got = append(got, mt)
		return nil
	})
	if err != nil || !found {
		t.Fatalf("Search = found %v, err %v", found, err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d records: %+v", len(got), got)
	}
	if got[0].Line != 1 || !got[0].Context || got[1].Line != 2 || got[1].Context {
		t.Fatalf("unexpected first group: %+v", got[:2])
	}
	if got[0].File != "input.txt" {
		t.Fatalf("file = %q", got[0].File)
	}
}

func TestSearchContextGroupBoundaries(t *testing.T) {
	m := mustLiteral(t, "hit")
	var got []Match
	_, err := Search(strings.NewReader("hit\na\nb\nc\nhit\n"), m, Options{
		AfterContext: 1,
	}, func(mt Match) error {
		got = append(got, mt)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || !got[2].GroupStart {
		t.Fatalf("records = %+v; want second match to begin a new group", got)
	}
}

func TestSearchRejectsNegativeContext(t *testing.T) {
	m := mustLiteral(t, "hit")
	if _, err := Search(strings.NewReader("hit\n"), m, Options{BeforeContext: -1}, func(Match) error { return nil }); err == nil {
		t.Fatal("Search with negative context returned no error")
	}
}

func TestSearchOmitText(t *testing.T) {
	m := mustLiteral(t, "غ")
	var got []Match
	found, err := Search(strings.NewReader("سطر\nغابة\nآخر\n"), m,
		Options{OmitText: true, InvertMatch: true}, func(mt Match) error {
			got = append(got, mt)
			return nil
		})
	if err != nil || !found || len(got) != 2 || got[0].Line != 1 || got[1].Line != 3 ||
		got[0].Text != "" || got[1].Text != "" {
		t.Fatalf("summary selection changed: found=%v, matches=%+v, err=%v", found, got, err)
	}
	for _, opts := range []Options{
		{OmitText: true, BeforeContext: 1},
		{OmitText: true, AfterContext: 1},
		{OmitText: true, MapSpans: true},
	} {
		if _, err := Search(strings.NewReader("غابة\n"), m, opts, func(Match) error { return nil }); err == nil {
			t.Fatalf("accepted incompatible options: %+v", opts)
		}
	}
}

func TestSearchMapsSpansToOriginalText(t *testing.T) {
	m := mustLiteral(t, "مد")
	input := "مُدُ next\n"
	var got Match
	found, err := Search(strings.NewReader(input), m, Options{MapSpans: true}, func(mt Match) error {
		got = mt
		return nil
	})
	if err != nil || !found {
		t.Fatalf("Search = found %v, err %v", found, err)
	}
	want := Span{0, len("مُدُ")}
	if len(got.Spans) != 1 || got.Spans[0] != want {
		t.Fatalf("spans = %v; want %v", got.Spans, want)
	}
}

func TestSearchMapsPresentationComponentToWholeLigature(t *testing.T) {
	m := mustLiteral(t, "ل")
	var got Match
	_, err := Search(strings.NewReader("ﻻ\n"), m, Options{MapSpans: true}, func(mt Match) error {
		got = mt
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (Span{0, len("ﻻ")}); len(got.Spans) != 1 || got.Spans[0] != want {
		t.Fatalf("spans = %v; want %v", got.Spans, want)
	}
}

func TestSearchMappedLongLineThenShortLine(t *testing.T) {
	m := mustLiteral(t, "غ")
	long := strings.Repeat("ا", (1<<17)+1) + " غ"
	var got []Match
	found, err := Search(strings.NewReader(long+"\nغ\n"), m, Options{MapSpans: true}, func(mt Match) error {
		got = append(got, mt)
		return nil
	})
	if err != nil || !found || len(got) != 2 ||
		len(got[0].Spans) != 1 || got[0].Spans[0] != (Span{len(long) - len("غ"), len(long)}) ||
		len(got[1].Spans) != 1 || got[1].Spans[0] != (Span{0, len("غ")}) {
		t.Fatalf("long then short mapped spans: found=%v, matches=%+v, err=%v", found, got, err)
	}
}

func TestSearchWordRegexpUsesOriginalBoundaries(t *testing.T) {
	m := mustLiteral(t, "كتاب")
	input := "كتابه\nكتاب،\nالــكتاب\n"
	var lines []int64
	found, err := Search(strings.NewReader(input), m, Options{MapSpans: true, WordRegexp: true}, func(mt Match) error {
		lines = append(lines, mt.Line)
		return nil
	})
	if err != nil || !found {
		t.Fatalf("Search = found %v, err %v", found, err)
	}
	if len(lines) != 1 || lines[0] != 2 {
		t.Fatalf("word matches = %v; want [2]", lines)
	}
}

func TestSearchWordRegexpTreatsPersianZWNJAsWordInternal(t *testing.T) {
	p := arabic.ProfileSearch
	p.Languages = arabic.LanguagePersian
	m, err := match.NewLiteral("می", p)
	if err != nil {
		t.Fatal(err)
	}
	input := "می\u200cروم\nمی،\n"
	var lines []int64
	found, err := Search(strings.NewReader(input), m, Options{WordRegexp: true}, func(mt Match) error {
		lines = append(lines, mt.Line)
		return nil
	})
	if err != nil || !found {
		t.Fatalf("Search = found %v, err %v", found, err)
	}
	if len(lines) != 1 || lines[0] != 2 {
		t.Fatalf("word matches = %v; want [2]", lines)
	}
}

type matcherWithoutRawShortcuts struct{ match.Matcher }

func TestMappedRawRejectionMatchesFullPath(t *testing.T) {
	input := "سطر بعيد\nﻍ\nغَ،\nكِتاب\nثوب\nغَيِّر\n"
	for _, queries := range [][]string{{"غ"}, {"غ", "ث"}} {
		m, err := match.NewLiterals(queries, arabic.ProfileSearch, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range []Options{
			{MapSpans: true},
			{MapSpans: true, BeforeContext: 1, AfterContext: 1},
			{MapSpans: true, WordRegexp: true},
			{MapSpans: true, InvertMatch: true},
			{WordRegexp: true, OmitText: true},
		} {
			search := func(m match.Matcher) (bool, []Match, error) {
				var got []Match
				found, err := Search(strings.NewReader(input), m, opts, func(mt Match) error {
					got = append(got, mt)
					return nil
				})
				return found, got, err
			}
			found, got, err := search(m)
			wantFound, want, wantErr := search(matcherWithoutRawShortcuts{m})
			if err != nil || wantErr != nil || found != wantFound || !reflect.DeepEqual(got, want) {
				t.Fatalf("queries=%q opts=%+v: optimized=(%v,%+v,%v), full=(%v,%+v,%v)",
					queries, opts, found, got, err, wantFound, want, wantErr)
			}
		}
	}
}

func TestMappedRawRejectionPreservesReadErrors(t *testing.T) {
	m := mustLiteral(t, "غ")
	for _, opts := range []Options{{MapSpans: true}, {WordRegexp: true}} {
		opts.MaxLineBytes = 5
		if _, err := Search(strings.NewReader("abcdef\n"), m, opts, func(Match) error { return nil }); err == nil {
			t.Fatalf("mapped rejection bypassed line limit: %+v", opts)
		}
		opts.MaxLineBytes = 0
		if _, err := Search(bytes.NewReader([]byte{'x', 0xff, '\n'}), m, opts,
			func(Match) error { return nil }); !errors.Is(err, ErrInvalidUTF8) {
			t.Fatalf("mapped rejection bypassed UTF-8 validation: %+v, %v", opts, err)
		}
	}
}

// FuzzSearchNoPanic feeds arbitrary input bytes, an arbitrary query, and an
// arbitrary profile through match.NewLiteral and Search, and only requires
// that it returns (an error is fine; a panic is not). It exercises the
// reader/line-assembly path in readLine together with normalization and
// matching, which the arabic and match package fuzz targets don't reach
// together.
func FuzzSearchNoPanic(f *testing.F) {
	f.Add([]byte("هذِهِ مَدْرَسَةٌ\r\nمدرسه\n"), "مدرسه", uint64(0), 0)
	f.Add([]byte("plain\nascii\ntext\n"), "text", uint64(0), 1)
	f.Add([]byte{'x', 0xff, '\n'}, "x", uint64(0), 2)
	f.Add([]byte("one\ntwo\n"), "unused", uint64(0), 3)
	f.Add([]byte("12345\n"), "1", uint64(4), 4)
	f.Add([]byte(""), "q", uint64(0), 0)

	profiles := [...]arabic.Profile{
		arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose,
		arabic.ProfileLucene, arabic.ProfileCAMeL,
	}

	f.Fuzz(func(t *testing.T, data []byte, query string, maxLineBytes uint64, profileIdx int) {
		i := profileIdx % len(profiles)
		if i < 0 {
			i += len(profiles)
		}

		m, err := match.NewLiteral(query, profiles[i])
		if err != nil {
			return
		}
		// Cap maxLineBytes so a fuzzer-discovered huge value can't turn this
		// into a slow/OOM test run instead of a correctness one.
		maxLineBytes %= 1 << 20

		_, _ = Search(bytes.NewReader(data), m, Options{MaxLineBytes: maxLineBytes}, func(Match) error {
			return nil
		})
	})
}

func FuzzSearchMappedNoPanic(f *testing.F) {
	f.Add([]byte("هذه مَدِينَة\nﻻ\n"), "مدينه", 0, false)
	f.Add([]byte("كتابه\nكتاب،\n"), "كتاب", 1, true)
	f.Add([]byte{'x', 0xff, '\n'}, "x", 2, false)

	profiles := [...]arabic.Profile{
		arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose,
		arabic.ProfileLucene, arabic.ProfileCAMeL,
	}
	f.Fuzz(func(t *testing.T, data []byte, query string, profileIdx int, word bool) {
		i := profileIdx % len(profiles)
		if i < 0 {
			i += len(profiles)
		}
		m, err := match.NewLiteral(query, profiles[i])
		if err != nil {
			return
		}
		_, _ = Search(bytes.NewReader(data), m, Options{MapSpans: true, WordRegexp: word}, func(Match) error {
			return nil
		})
	})
}
