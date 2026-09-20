//go:build formats

package inputformat

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func TestOpenHTML(t *testing.T) {
	name := filepath.Join(t.TempDir(), "page.html")
	contents := `<html><head><style>hidden</style></head><body><h1>عنوان</h1><p>هذه <b>مدينة</b> جميلة</p><script>hidden</script></body></html>`
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	reader, utf8Text, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8Text || string(got) != "عنوان\nهذه مدينة جميلة\n" {
		t.Fatalf("utf8=%v text=%q", utf8Text, got)
	}
}

func TestOpenHTMLHonorsDeclaredLegacyEncoding(t *testing.T) {
	name := filepath.Join(t.TempDir(), "legacy.html")
	contents := `<html><head><meta charset="windows-1256"></head><body><p>مدينة</p></body></html>`
	raw, _, err := transform.Bytes(charmap.Windows1256.NewEncoder(), []byte(contents))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	reader, utf8Text, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8Text || string(got) != "مدينة\n" {
		t.Fatalf("utf8=%v text=%q", utf8Text, got)
	}
}

func TestOpenEPUBUsesSpineOrder(t *testing.T) {
	name := filepath.Join(t.TempDir(), "book.epub")
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OPS/book.opf"/></rootfiles></container>`,
		"OPS/book.opf":           `<package><manifest><item id="second" href="b.xhtml" media-type="application/xhtml+xml"/><item id="first" href="a.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="first"/><itemref idref="second"/></spine></package>`,
		"OPS/a.xhtml":            `<html><body><p>الفصل الأول</p></body></html>`,
		"OPS/b.xhtml":            `<html><body><p>الفصل الثاني</p></body></html>`,
	}
	for path, contents := range entries {
		entry, err := archive.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(entry, strings.NewReader(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	reader, utf8Text, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8Text || string(got) != "الفصل الأول\nالفصل الثاني\n" {
		t.Fatalf("utf8=%v text=%q", utf8Text, got)
	}
}
