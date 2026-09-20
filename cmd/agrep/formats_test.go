//go:build formats

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSearchesExtractedHTML(t *testing.T) {
	name := filepath.Join(t.TempDir(), "page.html")
	contents := `<html><body><p>هذه <b>مَدِينَة</b> جميلة</p></body></html>`
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := run([]string{"--json", "مدينه", name}, strings.NewReader(""), &stdout, &stderr)
	want := "{\"file\":" + strconvQuote(name) + ",\"line\":1,\"text\":\"هذه مَدِينَة جميلة\",\"spans\":[[7,23]]}\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q want=%q stderr=%q", code, stdout.String(), want, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--encoding=cp1256", "مدينه", name}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "extracted document text is UTF-8") {
		t.Fatalf("encoding conflict: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
