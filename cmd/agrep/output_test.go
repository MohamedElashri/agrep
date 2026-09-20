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
