package playground

import (
	"slices"
	"testing"
)

func TestEvaluate(t *testing.T) {
	got, err := Evaluate("ﻻ كِتـاب", "search", "ar")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "لا كتاب" {
		t.Fatalf("key=%q", got.Key)
	}
	for _, rule := range []string{"presentation forms", "tashkil", "tatweel"} {
		if !slices.Contains(got.Rules, rule) {
			t.Fatalf("%q missing from rules %v", rule, got.Rules)
		}
	}
	if _, err := Evaluate("x", "unknown", "ar"); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if _, err := Evaluate("x", "search", "unknown"); err == nil {
		t.Fatal("unknown language accepted")
	}
}
