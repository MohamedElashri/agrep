package walk

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func BenchmarkCollect(b *testing.B) {
	root := walkerFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := Collect([]string{root}, Options{Recursive: true})
		if err != nil {
			b.Fatal(err)
		}
		if len(files) != 10_000 {
			b.Fatalf("got %d files", len(files))
		}
	}
}

func BenchmarkRipgrepFiles(b *testing.B) {
	if _, err := exec.LookPath("rg"); err != nil {
		b.Skip("ripgrep is not installed")
	}
	root := walkerFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output, err := exec.Command("rg", "--files", "--no-ignore", root).Output()
		if err != nil {
			b.Fatal(err)
		}
		if count := bytes.Count(output, []byte{'\n'}); count != 10_000 {
			b.Fatalf("got %d files", count)
		}
	}
}

func walkerFixture(tb testing.TB) string {
	tb.Helper()
	root := tb.TempDir()
	for dir := 0; dir < 100; dir++ {
		for file := 0; file < 100; file++ {
			name := filepath.Join(root, fmt.Sprintf("d%03d/f%03d.txt", dir, file))
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(name, []byte("fixture\n"), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return root
}
