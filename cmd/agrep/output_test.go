package main

import (
	"strings"
	"testing"

	"github.com/MohamedElashri/agrep/scan"
)

func TestColorizeMergesOverlappingSpans(t *testing.T) {
	got := colorize("abcdef", []scan.Span{{1, 4}, {2, 5}}, false)
	want := "a" + ansiMatchStart + "bcde" + ansiMatchEnd + "f"
	if got != want {
		t.Fatalf("colorize = %q; want %q", got, want)
	}
}

func TestColorizeDoesNotIsolateLatin(t *testing.T) {
	got := colorize("hello", []scan.Span{{0, 5}}, true)
	want := ansiMatchStart + "hello" + ansiMatchEnd
	if got != want {
		t.Fatalf("colorize = %q; want %q", got, want)
	}
}

func TestColorEnabledModes(t *testing.T) {
	var output strings.Builder
	if colorEnabled(colorAuto, &output) {
		t.Fatal("auto color enabled for a non-terminal writer")
	}
	if !colorEnabled(colorAlways, &output) {
		t.Fatal("always color was not enabled")
	}
	if colorEnabled(colorNever, &output) {
		t.Fatal("never color was enabled")
	}
}

func TestHumanEmitterPlainPrefixesAndContext(t *testing.T) {
	var output strings.Builder
	emit := humanEmitter(&output, humanOutputOptions{lineNumbers: true, filenames: true})
	for _, mt := range []scan.Match{
		{File: "notes.txt", Line: 3, Text: "كتاب"},
		{File: "notes.txt", Line: 8, Text: "سياق", Context: true, GroupStart: true},
	} {
		if err := emit(mt); err != nil {
			t.Fatal(err)
		}
	}
	if want := "notes.txt:3:كتاب\n--\nnotes.txt-8-سياق\n"; output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}
