package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func recursiveBenchTree(b *testing.B, files, linesPerFile int) string {
	b.Helper()
	root := b.TempDir()
	line := "أعلنت المدينة افتتاح مكتبة جديدة.\n"
	contents := line + strings.Repeat("سطر عادي بلا تطابق.\n", linesPerFile-1)
	for i := range files {
		name := filepath.Join(root, "part-"+strconv.Itoa(i)+".txt")
		if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func benchmarkRecursiveTree(b *testing.B, files, linesPerFile int) {
	root := recursiveBenchTree(b, files, linesPerFile)
	args := []string{"-r", "--threads=4", "-l", "اعلنت", root}
	b.ResetTimer()
	for range b.N {
		if code := run(args, strings.NewReader(""), io.Discard, io.Discard); code != 0 {
			b.Fatalf("run code = %d", code)
		}
	}
}

func BenchmarkRecursiveManySmall(b *testing.B) { benchmarkRecursiveTree(b, 1000, 8) }

func BenchmarkRecursiveFewLarge(b *testing.B) { benchmarkRecursiveTree(b, 4, 50000) }

func BenchmarkDiscoverManySmall(b *testing.B) {
	root := recursiveBenchTree(b, 1000, 8)
	opts := cliOptions{recursive: true}
	b.ResetTimer()
	for range b.N {
		files, err := discoverInputs([]string{root}, opts)
		if err != nil || len(files) != 1000 {
			b.Fatalf("discoverInputs: files=%d, err=%v", len(files), err)
		}
	}
}
