package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunJSONLines(t *testing.T) {
	input := "ignore\n\u0627\u0644\u0645\u064E\u062F\u0650\u064A\u0646\u064E\u0629 <tag>\n"
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "\u0645\u062F\u064A\u0646\u0647"}, strings.NewReader(input), &stdout, &stderr)
	want := "{\"line\":2,\"text\":\"\u0627\u0644\u0645\u064E\u062F\u0650\u064A\u0646\u064E\u0629 <tag>\"}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunHumanOutput(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"-n", "needle"}, strings.NewReader("x\nneedle here\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "2:needle here\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDoubleDashAllowsHyphenQuery(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--", "-needle"}, strings.NewReader("a -needle here\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "a -needle here\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want int
	}{
		{"match", []string{"x"}, "x\n", 0},
		{"no match", []string{"x"}, "y\n", 1},
		{"missing query", nil, "", 2},
		{"too many args", []string{"x", "a", "b"}, "", 2},
		{"empty query", []string{""}, "x\n", 2},
		{"marks-only query", []string{"\u064E"}, "x\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if got := run(tt.args, strings.NewReader(tt.in), &stdout, &stderr); got != tt.want {
				t.Fatalf("exit code = %d; want %d", got, tt.want)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestRunWriteFailure(t *testing.T) {
	var stderr strings.Builder
	if code := run([]string{"x"}, strings.NewReader("x\n"), failingWriter{}, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}} {
		var stdout, stderr strings.Builder
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunReadFailure(t *testing.T) {
	reader := io.MultiReader(strings.NewReader("safe\n"), errorReader{})
	var stdout, stderr strings.Builder
	if code := run([]string{"missing"}, reader, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d; want 2", code)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
