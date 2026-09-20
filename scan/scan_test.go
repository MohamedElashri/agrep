package scan

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/MohamedElashri/agrep/match"
)

func mustLiteral(t testing.TB, query string) match.Matcher {
	t.Helper()
	m, err := match.NewLiteral(query)
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
	if err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v", err)
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

// FuzzSearchNoPanic feeds arbitrary input bytes and an arbitrary query
// through match.NewLiteral and Search, and only requires that it returns (an
// error is fine; a panic is not). It exercises the reader/line-assembly path
// in readLine together with normalization and matching, which the arabic and
// match package fuzz targets don't reach together.
func FuzzSearchNoPanic(f *testing.F) {
	f.Add([]byte("هذِهِ مَدْرَسَةٌ\r\nمدرسه\n"), "مدرسه", uint64(0))
	f.Add([]byte("plain\nascii\ntext\n"), "text", uint64(0))
	f.Add([]byte{'x', 0xff, '\n'}, "x", uint64(0))
	f.Add([]byte("one\ntwo\n"), "unused", uint64(0))
	f.Add([]byte("12345\n"), "1", uint64(4))
	f.Add([]byte(""), "q", uint64(0))

	f.Fuzz(func(t *testing.T, data []byte, query string, maxLineBytes uint64) {
		m, err := match.NewLiteral(query)
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
