package walk

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectHonorsIgnoreAndFilters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "ignored.txt\ncache/\n*.log\n!keep.log\n")
	writeFile(t, root, "a.txt", "a")
	writeFile(t, root, "ignored.txt", "ignored")
	writeFile(t, root, "keep.log", "keep")
	writeFile(t, root, "skip.md", "skip")
	writeFile(t, root, "cache/data.txt", "cache")
	writeFile(t, root, "nested/.gitignore", "local.txt\n")
	writeFile(t, root, "nested/b.txt", "b")
	writeFile(t, root, "nested/local.txt", "local")

	got, err := Collect([]string{root}, Options{
		Recursive: true,
		Includes:  []string{"*.txt", "*.log"},
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "keep.log"),
		filepath.Join(root, "nested/b.txt"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() = %q; want %q", got, want)
	}
}

func TestCollectNoIgnore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "ignored.txt\n")
	writeFile(t, root, "ignored.txt", "ignored")

	got, err := Collect([]string{root}, Options{Recursive: true, NoIgnore: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	want := []string{filepath.Join(root, ".gitignore"), filepath.Join(root, "ignored.txt")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() = %q; want %q", got, want)
	}
}

func TestCollectIgnoreAnchoringAndEscapedNegation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "/root-only.txt\n\\!literal.txt\n")
	writeFile(t, root, "root-only.txt", "ignored")
	writeFile(t, root, "nested/root-only.txt", "kept")
	writeFile(t, root, "!literal.txt", "ignored")

	got, err := Collect([]string{root}, Options{Recursive: true, Includes: []string{"*.txt"}})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	want := []string{filepath.Join(root, "nested/root-only.txt")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() = %q; want %q", got, want)
	}
}

func TestCollectRejectsDirectoryWithoutRecursive(t *testing.T) {
	if _, err := Collect([]string{t.TempDir()}, Options{}); err == nil {
		t.Fatal("Collect(directory) returned no error")
	}
}

func TestCollectPreservesExplicitFileOrderAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	writeFile(t, root, "a.txt", "a")
	writeFile(t, root, "b.txt", "b")

	got, err := Collect([]string{b, a, b}, Options{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if want := []string{b, a}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() = %q; want %q", got, want)
	}
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
