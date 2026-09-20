package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSearchReturnsExactOriginalLines(t *testing.T) {
	input := "ignore\r\n\u0647\u0630\u0650\u0647\u0650 \u0645\u064E\u062F\u0652\u0631\u064E\u0633\u064E\u0629\u064C\r\n\u0645\u062F\u0631\u0633\u0647"
	var got []Match
	found, err := search(strings.NewReader(input), "\u0645\u062F\u0631\u0633\u0647", searchOptions{}, func(m Match) error {
		got = append(got, m)
		return nil
	})
	if err != nil || !found {
		t.Fatalf("search() = found %v, err %v", found, err)
	}
	if len(got) != 2 || got[0].Line != 2 || got[1].Line != 3 {
		t.Fatalf("matches = %+v", got)
	}
	if got[0].Text != "\u0647\u0630\u0650\u0647\u0650 \u0645\u064E\u062F\u0652\u0631\u064E\u0633\u064E\u0629\u064C" {
		t.Fatalf("original text changed: %q", got[0].Text)
	}
}

func TestSearchHandlesBlankAndLongLines(t *testing.T) {
	long := strings.Repeat("x", 17*1024*1024) + " \u0623\u062D\u0645\u062F"
	input := "\n" + long
	var got Match
	found, err := search(strings.NewReader(input), "\u0627\u062D\u0645\u062F", searchOptions{}, func(m Match) error {
		got = m
		return nil
	})
	if err != nil || !found || got.Line != 2 || got.Text != long {
		t.Fatalf("long-line search failed: found=%v line=%d err=%v", found, got.Line, err)
	}
}

func TestSearchLineLimit(t *testing.T) {
	_, err := search(strings.NewReader("12345\n"), "1", searchOptions{MaxLineBytes: 4}, func(Match) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchLineLimitAllowsCRLFAtLimit(t *testing.T) {
	var got Match
	found, err := search(strings.NewReader("1234\r\n"), "1", searchOptions{MaxLineBytes: 4}, func(m Match) error {
		got = m
		return nil
	})
	if err != nil || !found || got.Text != "1234" {
		t.Fatalf("found=%v match=%+v err=%v", found, got, err)
	}
}

func TestSearchRejectsInvalidUTF8(t *testing.T) {
	_, err := search(strings.NewReader(string([]byte{'x', 0xff, '\n'})), "x", searchOptions{}, func(Match) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchPropagatesCallbackFailure(t *testing.T) {
	want := errors.New("write failed")
	_, err := search(strings.NewReader("match\n"), "match", searchOptions{}, func(Match) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want %v", err, want)
	}
}

func TestSearchNoMatch(t *testing.T) {
	called := false
	found, err := search(strings.NewReader("one\ntwo\n"), "missing", searchOptions{}, func(Match) error {
		called = true
		return nil
	})
	if err != nil || found || called {
		t.Fatalf("found=%v called=%v err=%v", found, called, err)
	}
}
