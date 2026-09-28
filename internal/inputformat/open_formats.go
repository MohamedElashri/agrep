//go:build formats

// Package inputformat extracts searchable UTF-8 text from optional document
// formats while leaving ordinary files as raw byte streams.
package inputformat

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const maxExtractedDocumentBytes = 256 << 20

// Open extracts HTML or EPUB files and opens other paths unchanged. Extracted
// document text is UTF-8 regardless of the surrounding input encoding option.
func Open(name string) (reader io.ReadCloser, utf8Text bool, err error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm":
		file, err := os.Open(name)
		if err != nil {
			return nil, false, err
		}
		defer file.Close()
		limited := &io.LimitedReader{R: file, N: maxExtractedDocumentBytes + 1}
		text, err := extractHTML(limited)
		if err != nil {
			return nil, false, fmt.Errorf("extract HTML: %w", err)
		}
		if limited.N == 0 {
			return nil, false, errors.New("extract HTML: document exceeds 256 MiB")
		}
		return io.NopCloser(strings.NewReader(text)), true, nil
	case ".epub":
		text, err := extractEPUB(name)
		if err != nil {
			return nil, false, fmt.Errorf("extract EPUB: %w", err)
		}
		return io.NopCloser(strings.NewReader(text)), true, nil
	default:
		file, err := os.Open(name)
		return file, false, err
	}
}

func extractHTML(r io.Reader) (string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		selected, _, _ := charset.DetermineEncoding(raw, "")
		raw, err = selected.NewDecoder().Bytes(raw)
		if err != nil {
			return "", fmt.Errorf("decode HTML character set: %w", err)
		}
	}
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	var lines []string
	var current strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, skip bool) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style", "noscript", "svg", "template":
				skip = true
			case "br", "hr":
				flushHTMLLine(&current, &lines)
			}
		}
		if node.Type == html.TextNode && !skip {
			for _, field := range strings.Fields(node.Data) {
				if current.Len() > 0 {
					current.WriteByte(' ')
				}
				current.WriteString(field)
			}
		}
		block := node.Type == html.ElementNode && isHTMLBlock(node.Data)
		if block {
			flushHTMLLine(&current, &lines)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skip)
		}
		if block {
			flushHTMLLine(&current, &lines)
		}
	}
	walk(doc, false)
	flushHTMLLine(&current, &lines)
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func flushHTMLLine(current *strings.Builder, lines *[]string) {
	if current.Len() == 0 {
		return
	}
	*lines = append(*lines, current.String())
	current.Reset()
}

func isHTMLBlock(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "dd", "div", "dl", "dt", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "ul":
		return true
	default:
		return false
	}
}

type containerDocument struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type packageDocument struct {
	Manifest []struct {
		ID        string `xml:"id,attr"`
		Href      string `xml:"href,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

func extractEPUB(name string) (string, error) {
	archive, err := zip.OpenReader(name)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	files := make(map[string]*zip.File, len(archive.File))
	for _, file := range archive.File {
		files[path.Clean(file.Name)] = file
	}
	containerFile := files["META-INF/container.xml"]
	if containerFile == nil {
		return "", errors.New("missing META-INF/container.xml")
	}
	var container containerDocument
	if err := decodeZipXML(containerFile, &container); err != nil {
		return "", fmt.Errorf("read container: %w", err)
	}
	if len(container.Rootfiles) == 0 {
		return "", errors.New("container has no rootfile")
	}
	opfName := path.Clean(container.Rootfiles[0].FullPath)
	opfFile := files[opfName]
	if opfFile == nil {
		return "", fmt.Errorf("missing package document %q", opfName)
	}
	var pkg packageDocument
	if err := decodeZipXML(opfFile, &pkg); err != nil {
		return "", fmt.Errorf("read package document: %w", err)
	}
	manifest := make(map[string]struct {
		name, mediaType string
	}, len(pkg.Manifest))
	base := path.Dir(opfName)
	for _, item := range pkg.Manifest {
		href, _, _ := strings.Cut(item.Href, "#")
		href, _, _ = strings.Cut(href, "?")
		manifest[item.ID] = struct {
			name, mediaType string
		}{path.Clean(path.Join(base, href)), item.MediaType}
	}

	var output strings.Builder
	var total int64
	for _, itemref := range pkg.Spine {
		item, ok := manifest[itemref.IDRef]
		if !ok || (item.mediaType != "application/xhtml+xml" && item.mediaType != "text/html") {
			continue
		}
		file := files[item.name]
		if file == nil {
			return "", fmt.Errorf("missing spine item %q", item.name)
		}
		opened, err := file.Open()
		if err != nil {
			return "", err
		}
		remaining := maxExtractedDocumentBytes - total
		if remaining <= 0 {
			opened.Close()
			return "", errors.New("extracted text exceeds 256 MiB")
		}
		limited := &io.LimitedReader{R: opened, N: remaining + 1}
		text, extractErr := extractHTML(limited)
		closeErr := opened.Close()
		if extractErr != nil {
			return "", extractErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if limited.N == 0 {
			return "", errors.New("extracted text exceeds 256 MiB")
		}
		total += int64(len(text))
		if total > maxExtractedDocumentBytes {
			return "", errors.New("extracted text exceeds 256 MiB")
		}
		output.WriteString(text)
	}
	return output.String(), nil
}

func decodeZipXML(file *zip.File, target any) error {
	opened, err := file.Open()
	if err != nil {
		return err
	}
	defer opened.Close()
	return xml.NewDecoder(io.LimitReader(opened, maxExtractedDocumentBytes+1)).Decode(target)
}
