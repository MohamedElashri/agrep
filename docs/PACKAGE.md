---
title: "Go Package & API Guide"
description: "Comprehensive guide to using agrep as an embedded Go library: linguistic normalization, literal and fuzzy matchers, streaming scanner, and span mapping."
category: "Tools & Ecosystem"
order: 2
---

# Go Package & API Guide

While `agrep` is distributed as a high-performance command-line binary, its underlying engine is designed and implemented as a modular suite of decoupled Go packages. You can import these packages directly into your Go services, data pipelines, search engines, and linguistic analysis tools.

---

## Package Overview & Architecture

The `agrep` Go engine is partitioned into three core packages under the `github.com/MohamedElashri/agrep` module:

```text
┌─────────────────────────────────────────────────────────────┐
│                       Your Application                      │
└──────┬───────────────────────┬───────────────────────┬──────┘
       │                       │                       │
       ▼                       ▼                       ▼
┌──────────────┐        ┌──────────────┐        ┌──────────────┐
│    arabic    │        │    match     │        │     scan     │
├──────────────┤        ├──────────────┤        ├──────────────┤
│ Normalization│        │ Matcher      │        │ Streaming    │
│ Profiles     │ ◄────  │ Literal      │ ◄────  │ Line Scanner │
│ Languages    │        │ Myers Fuzzy  │        │ Context Lines│
│ Span Mapping │        │ Regex        │        │ JSON Spans   │
└──────────────┘        └──────────────┘        └──────────────┘
```

| Package | Import Path | Primary Responsibility |
| :--- | :--- | :--- |
| `arabic` | `github.com/MohamedElashri/agrep/arabic` | Linguistic Unicode normalization, diacritic stripping, presentation expansion, multi-language orthographies, and exact source byte span mapping. |
| `match` | `github.com/MohamedElashri/agrep/match` | Precompiled search engines implementing the `Matcher` interface: literal substrings, bit-parallel Myers fuzzy edit distance, and regular expressions. |
| `scan` | `github.com/MohamedElashri/agrep/scan` | High-throughput streaming line scanner over arbitrary `io.Reader` streams with zero-allocation buffers, context windows, and span remapping. |

### Installation

Add `agrep` to your Go module dependencies:

```sh
go get github.com/MohamedElashri/agrep
```

Requirements: Go 1.22 or higher. The packages have no CGO requirements and compile across Linux, macOS, and Windows.

---

## 1. Package `arabic`: Linguistic Normalization

The `arabic` package normalizes Arabic-script text into canonical comparison keys. Normalization is intentionally lossy: it allows queries typed without vowel marks, with alternative hamza placements, or from different regional keyboards to match text in digitized corpora.

### Quickstart

For common search scenarios, `arabic.Normalize` applies `ProfileSearch` (agrep's default behavior):

```go
package main

import (
	"fmt"

	"github.com/MohamedElashri/agrep/arabic"
)

func main() {
	voweled := "مَدْرَسَةٌ"
	unvoweled := "مدرسه"

	key1 := arabic.Normalize(voweled)
	key2 := arabic.Normalize(unvoweled)

	fmt.Println(key1 == key2) // Output: true
	fmt.Println(key1)         // Output: مدرسه
}
```

### Normalization Profiles & Presets

Different applications require different degrees of linguistic tolerance. Package `arabic` provides five preconfigured profiles:

| Profile | Tashkil | Tatweel | Hamza / Wasla | Hamza Seat | Ta-Marbuta | Alef Maksura | Digits | Rasm |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `ProfileSearch` | Stripped | Stripped | Folded | Folded | Folded | Folded | Preserved | Off |
| `ProfileStrict` | Stripped | Stripped | Preserved | Preserved | Preserved | Preserved | Preserved | Off |
| `ProfileLoose` | Stripped | Stripped | Folded | Folded | Folded | Folded | Folded | On |
| `ProfileLucene` | 8 Harakat | Stripped | Folded (no wasla)| Preserved | Folded | Folded | Preserved | Off |
| `ProfileCAMeL` | 9 Marks | Preserved | Folded (with wasla)| Preserved | Folded | Folded | Preserved | Off |

#### Using Built-in Profiles

```go
// Strict: preserves letter-level orthographic distinctions (e.g. ة vs ه, ى vs ي)
strictKey := arabic.ProfileStrict.Normalize("مكتبة") // retains 'ة'

// Loose: folds digits (٠-٩ -> 0-9), punctuation, and consonant skeletons (rasm)
looseKey := arabic.ProfileLoose.Normalize("عام ١٩٨٤، صـ ٤٢")

// Lucene: exact compatibility with Apache Lucene's ArabicNormalizer
luceneKey := arabic.ProfileLucene.Normalize(text)
```

#### Custom Profile Construction

You can build a tailored `Profile` struct with fine-grained control:

```go
customProfile := arabic.Profile{
	Languages:        arabic.LanguageArabic,
	StripTashkil:     true,
	TashkilScope:     arabic.TashkilAllMn,
	StripTatweel:     true,
	FoldAlefHamza:    true,
	FoldAlefWasla:    true,
	FoldHamzaSeat:    false, // preserve ؤ and ئ
	FoldTaMarbuta:    true,  // fold ة to ه
	FoldAlefMaksura:  false, // keep ى distinct from ي
	FoldPresentation: true,  // expand ligatures like ﻻ and ﷲ
	StripJoiners:     true,  // strip ZWJ/ZWNJ
	StripBidi:        true,  // strip LTR/RTL marks
	StripQuranic:     true,  // strip ayah marks and recitation symbols
	FoldDigits:       true,  // fold Arabic-Indic digits to ASCII
	FoldPunctuation:  true,  // fold Arabic comma/semicolon/question mark
}

if err := customProfile.Validate(); err != nil {
	log.Fatalf("Invalid profile: %v", err)
}
```

### Multi-Language Orthographies

By default, profiles assume Arabic orthography. When searching mixed corpora or non-Arabic languages, pass an `arabic.LanguageSet`:

```go
// Bitwise combinations of supported languages
persianAndArabic := arabic.LanguageArabic | arabic.LanguagePersian
urduOnly := arabic.LanguageUrdu

// Or parse from standard CLI language codes ("ar", "fa", "ur", "ps", "ku", "ug")
langs, err := arabic.ParseLanguages("ar,fa,ur")
if err != nil {
	log.Fatal(err)
}

profile := arabic.ProfileSearch
profile.Languages = langs

// With LanguagePersian | LanguageArabic enabled:
// Persian Keheh (ک) matches Arabic Kaf (ك)
// Farsi Yeh (ی) matches Arabic Yeh (ي)
k1 := profile.Normalize("کتاب") // Persian
k2 := profile.Normalize("كتاب") // Arabic
fmt.Println(k1 == k2)           // true
```

> [!NOTE]
> When `LanguagePersian`, `LanguageKurdish`, or `LanguageUyghur` is selected, `profile.Languages.PreservesZWNJ()` returns `true`. The engine preserves Zero-Width Non-Joiners (ZWNJ, `U+200C`) where they represent morphological suffixes (such as Persian plural *ha* in `خانه‌ها`).

### Consonant Skeleton (*Rasm*)

For unpointed historical manuscripts or early Islamic texts, enable `Rasm = true`:

```go
rasmProfile := arabic.ProfileSearch
rasmProfile.Rasm = true

// Folds ب, ت, ث, ن, ي to dotless skeleton ٮ
// Folds ج, ح, خ to ح
// Folds ف, ق to ٯ
fmt.Println(rasmProfile.Normalize("نستعين") == rasmProfile.Normalize("ٮسٮعىں")) // true
```

### Exact Span Mapping: `NormalizeMapped` & `MapSpan`

When text is normalized, characters are stripped (tashkil, tatweel) or expanded (presentation ligatures like `ﻻ` expand to `ل` + `ا`). As a result, **byte offsets in the normalized string do not match byte offsets in the original text**.

To highlight or extract matches in the original document without altering formatting or vowels, use `NormalizeMapped` and `MapSpan`:

```go
text := "هذِهِ مَدْرَسَةٌ مُبَارَكَةٌ"

// 1. Normalize and generate offset index
normKey, idx := arabic.ProfileSearch.NormalizeMapped(text)
// normKey == "هذه مدرسه مباركه"

// 2. Locate substring in normalized key
normStart := 5  // start of "مدرسه"
normEnd := 10   // end of "مدرسه"

// 3. Map back to exact slice in the original text
origStart, origEnd := arabic.MapSpan(idx, normStart, normEnd)
matchedSlice := text[origStart:origEnd]

fmt.Printf("Original slice: %q\n", matchedSlice)
// Output: Original slice: "مَدْرَسَةٌ"
```

#### Zero-Allocation Span Mapping in Loops

In high-throughput loops, avoid reallocating index slices with `NormalizeMappedInto`:

```go
var idxBuf []int32

for _, line := range lines {
	normKey, idx := profile.NormalizeMappedInto(line, idxBuf)
	idxBuf = idx // retain allocated capacity for next line
	// ... process spans ...
}
```

---

## 2. Package `match`: Search Matchers

Package `match` defines the `Matcher` interface and provides three compiled search implementations:

```go
type Matcher interface {
	FindAll(normalized string) []Span
	String() string
	Profile() arabic.Profile
}

type Span struct {
	Start, End int // Half-open byte bounds [Start, End) in normalized string
}
```

A `Matcher` is compiled **once** with a target `Profile`, and then executed against normalized text. Matchers are safe for concurrent use across goroutines.

### Literal Matcher (`NewLiteral` & `NewLiterals`)

Searches for exact normalized substring occurrences. Employs SIMD acceleration and raw invariant anchor rejection to bypass normalization on non-matching lines:

```go
import "github.com/MohamedElashri/agrep/match"

profile := arabic.ProfileSearch

// Single query
m, err := match.NewLiteral("مدرسه", profile)
if err != nil {
	log.Fatal(err)
}

// Multi-query with case-folding
queries := []string{"جامعة", "معهد", "كلية"}
multiM, err := match.NewLiterals(queries, profile, false)

// Find all matches in normalized text
haystack := profile.Normalize("تخرج من جامعة القاهرة")
for _, span := range multiM.FindAll(haystack) {
	fmt.Printf("Match: %q [%d:%d]\n", haystack[span.Start:span.End], span.Start, span.End)
}
```

### Myers Fuzzy Matcher (`NewFuzzy`)

Finds occurrences within a specified Levenshtein edit distance ($k \ge 0$) using bit-parallel Myers algorithms. Distance is measured over **Unicode codepoints** (runes), not raw UTF-8 bytes:

```go
// Allow up to 1 codepoint edit (insertion, deletion, or substitution)
fuzzyM, err := match.NewFuzzy([]string{"خوارزمي"}, arabic.ProfileSearch, false, 1)
if err != nil {
	log.Fatal(err)
}

// Typo: "خوارزمى" (with yeh) or "خوارزم" (missing final letter)
input := arabic.ProfileSearch.Normalize("عالم الرياضيات الخوارزمي الشهير")
spans := fuzzyM.FindAll(input)
for _, span := range spans {
	fmt.Printf("Fuzzy match: %q\n", input[span.Start:span.End])
}
```

### Regular Expression Matcher (`NewRegex`)

Evaluates regular expressions against normalized text. Expressions retain standard syntax (anchors, character classes, quantifiers):

```go
patterns := []string{`\bمدر[سص]ة\b`, `كتاب\s+\w+`}
reM, err := match.NewRegex(patterns, arabic.ProfileSearch, false)
if err != nil {
	log.Fatal(err)
}

spans := reM.FindAll(profile.Normalize("مدرسة جديدة"))
```

---

## 3. Package `scan`: Streaming Line Search

The `scan` package provides `scan.Search`, a high-throughput line scanner that reads from any `io.Reader` without the 64 KiB line limit of Go's standard `bufio.Scanner`.

```go
func Search(
	r io.Reader,
	m match.Matcher,
	opts Options,
	onMatch func(Match) error,
) (found bool, err error)
```

### Scanner Options

```go
type Options struct {
	MaxLineBytes  uint64 // Max line length; 0 means unlimited
	File          string // Path label copied to Match.File
	InvertMatch   bool   // Report non-matching lines (grep -v)
	BeforeContext int    // Preceding context lines (grep -B)
	AfterContext  int    // Trailing context lines (grep -A)
	MapSpans      bool   // Automatically map Match.Spans back to raw text
	WordRegexp    bool   // Enforce word boundaries on matches
	OmitText      bool   // Leave Match.Text empty for fast line counting
	ExistenceOnly bool   // Stop scanning after first match (grep -q)
}
```

### Scanner Match Record

```go
type Match struct {
	File       string    `json:"file,omitempty"`
	Line       int64     `json:"line"`
	Text       string    `json:"text"`              // Original line without CR/LF
	Context    bool      `json:"context,omitempty"` // True for context lines
	GroupStart bool      `json:"-"`                 // Marks discontinuity between match groups
	Spans      [][2]int  `json:"spans,omitempty"`  // Exact byte offsets in Text
}
```

### Basic Streaming Search

```go
package main

import (
	"fmt"
	"strings"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
	"github.com/MohamedElashri/agrep/scan"
)

func main() {
	corpus := `مقدمة الكتاب
هذِهِ مَدْرَسَةٌ قديمة في القاهرة.
تاريخ التعليم وتطوره.
`

	m, err := match.NewLiteral("مدرسه", arabic.ProfileSearch)
	if err != nil {
		panic(err)
	}

	opts := scan.Options{
		File:     "corpus.txt",
		MapSpans: true,
	}

	found, err := scan.Search(strings.NewReader(corpus), m, opts, func(mt scan.Match) error {
		fmt.Printf("[%s:%d] %s\n", mt.File, mt.Line, mt.Text)
		for _, span := range mt.Spans {
			matchText := mt.Text[span[0]:span[1]]
			fmt.Printf("  Highlighted span [%d:%d]: %q\n", span[0], span[1], matchText)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}
	fmt.Println("Matches found:", found)
}
```

---

## 4. Practical Code Recipes

### Recipe 1: In-Memory Search Index for Arabic Documents

Index a list of documents in memory and perform ultrafast searches:

```go
package main

import (
	"fmt"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
)

type Document struct {
	ID      int
	Content string
	normKey string
	idx     []int32
}

type SearchIndex struct {
	profile arabic.Profile
	docs    []Document
}

func NewIndex(profile arabic.Profile) *SearchIndex {
	return &SearchIndex{profile: profile}
}

func (idx *SearchIndex) Add(id int, text string) {
	normKey, mapIdx := idx.profile.NormalizeMapped(text)
	idx.docs = append(idx.docs, Document{
		ID:      id,
		Content: text,
		normKey: normKey,
		idx:     mapIdx,
	})
}

func (idx *SearchIndex) Search(query string) []string {
	m, err := match.NewLiteral(query, idx.profile)
	if err != nil {
		return nil
	}

	var results []string
	for _, doc := range idx.docs {
		spans := m.FindAll(doc.normKey)
		if len(spans) > 0 {
			// Extract original text of the first match
			s, e := arabic.MapSpan(doc.idx, spans[0].Start, spans[0].End)
			results = append(results, fmt.Sprintf("Doc %d: %q (match: %q)",
				doc.ID, doc.Content, doc.Content[s:e]))
		}
	}
	return results
}

func main() {
	idx := NewIndex(arabic.ProfileSearch)
	idx.Add(1, "تأسست مَدْرَسَةُ الطب سنة ١٨٢٧")
	idx.Add(2, "جامعة الأزهر بالقاهرة")
	idx.Add(3, "مدرسة الهندسة التطبيقية")

	results := idx.Search("مدرسه")
	for _, r := range results {
		fmt.Println(r)
	}
}
```

### Recipe 2: Highlight Matches with HTML `<mark>` Tags

Safely highlight matched terms in the original voweled text:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
)

func HighlightHTML(original string, query string, p arabic.Profile) (string, error) {
	m, err := match.NewLiteral(query, p)
	if err != nil {
		return "", err
	}

	normKey, idx := p.NormalizeMapped(original)
	spans := m.FindAll(normKey)
	if len(spans) == 0 {
		return original, nil
	}

	var sb strings.Builder
	lastEnd := 0

	for _, span := range spans {
		origStart, origEnd := arabic.MapSpan(idx, span.Start, span.End)
		if origStart < lastEnd {
			continue // skip overlapping spans
		}
		sb.WriteString(original[lastEnd:origStart])
		sb.WriteString("<mark>")
		sb.WriteString(original[origStart:origEnd])
		sb.WriteString("</mark>")
		lastEnd = origEnd
	}
	sb.WriteString(original[lastEnd:])
	return sb.String(), nil
}

func main() {
	src := "تَعَلَّمَ فِي مَدْرَسَةِ الحِكْمَةِ فِي بَغْدَادَ"
	highlighted, _ := HighlightHTML(src, "مدرسه", arabic.ProfileSearch)
	fmt.Println(highlighted)
	// Output: تَعَلَّمَ فِي <mark>مَدْرَسَةِ</mark> الحِكْمَةِ فِي بَغْدَادَ
}
```

### Recipe 3: Multi-Language Document Matching (Farsi & Arabic)

Handle bilingual Persian/Arabic corpora where keyboard variations occur:

```go
package main

import (
	"fmt"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
)

func main() {
	p := arabic.ProfileSearch
	p.Languages = arabic.LanguageArabic | arabic.LanguagePersian

	// Matcher with Persian query
	m, err := match.NewLiteral("دانشگاه", p)
	if err != nil {
		panic(err)
	}

	corpus := []string{
		"دانشگاه تهران در ایران",     // Persian spelling
		"دَانِشْگَاه في طهران",       // Arabicized vocalization
	}

	for _, line := range corpus {
		norm, idx := p.NormalizeMapped(line)
		spans := m.FindAll(norm)
		for _, s := range spans {
			start, end := arabic.MapSpan(idx, s.Start, s.End)
			fmt.Printf("Matched: %q in line: %q\n", line[start:end], line)
		}
	}
}
```

### Recipe 4: Scanning Real Files with Context Lines

Stream a large text file from disk, printing matching lines with 2 preceding and 2 following context lines:

```go
package main

import (
	"fmt"
	"os"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
	"github.com/MohamedElashri/agrep/scan"
)

func SearchFileWithContext(filePath string, query string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	m, err := match.NewLiteral(query, arabic.ProfileSearch)
	if err != nil {
		return err
	}

	opts := scan.Options{
		File:          filePath,
		BeforeContext: 2,
		AfterContext:  2,
		MapSpans:      true,
	}

	_, err = scan.Search(file, m, opts, func(mt scan.Match) error {
		prefix := ":"
		if mt.Context {
			prefix = "-"
		}
		if mt.GroupStart {
			fmt.Println("--")
		}
		fmt.Printf("%s%s%d%s%s\n", mt.File, prefix, mt.Line, prefix, mt.Text)
		return nil
	})

	return err
}
```

---

## 5. Performance Best Practices

1. **Precompile Matchers Once:** Creating a `match.Matcher` compiles the query, extracts invariant raw anchors, and prepares bit-masks. Create the matcher once and reuse it across millions of lines or concurrent goroutines.
2. **Reuse Buffer Memory:** In tight loops, avoid allocations by reusing slices with `p.NormalizeMappedInto(s, buffer)`.
3. **Use `OmitText = true` for Line Counting:** If you only need to count matches or check existence, set `opts.OmitText = true`. This activates the fast summary path, which checks blocks of lines against invariant raw byte filters without allocating string objects.
4. **Early Exit with Context:** Return a non-nil error from the `onMatch` callback to immediately stop the scanner when your limit is reached.
