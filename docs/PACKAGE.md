---
title: "Go Package & API Guide"
description: "Using agrep from Go: normalization profiles, literal, fuzzy, and regex matchers, the streaming scanner, and span mapping."
category: "Tools & Ecosystem"
order: 2
---

# Go packages

The CLI is built on three importable packages in the
`github.com/MohamedElashri/agrep` module. `scan` uses `match`, and `match`
uses `arabic`:

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

| Package | Import path | What it does |
| :--- | :--- | :--- |
| `arabic` | `github.com/MohamedElashri/agrep/arabic` | Builds comparison keys: strips diacritics, expands presentation forms, applies language rules, and maps key offsets back to the original text. |
| `match` | `github.com/MohamedElashri/agrep/match` | The `Matcher` interface and its literal, Myers fuzzy, and regex implementations. |
| `scan` | `github.com/MohamedElashri/agrep/scan` | Reads lines from any `io.Reader`, runs a `Matcher`, and reports matches, context lines, and original-text spans. |

### Installation

```sh
go get github.com/MohamedElashri/agrep
```

The module requires the Go version in `go.mod` (currently 1.27.1). It is pure Go and does not use cgo.

## Package `arabic`

`arabic` turns text into a comparison key. The key is lossy on purpose, so a
query typed without vowels or with a different alef or ta-marbuta spelling
still compares equal to the corpus text.

### Normalize

`arabic.Normalize` uses `ProfileSearch`, the CLI default:

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

### Profiles

There are five predefined profiles. [NORMALIZATION.md](NORMALIZATION.md) lists
the exact codepoints each rule touches.

| Profile | Tashkil | Tatweel | Hamza / Wasla | Hamza Seat | Ta-Marbuta | Alef Maksura | Digits | Rasm |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `ProfileSearch` | Stripped | Stripped | Folded | Folded | Folded | Folded | Preserved | Off |
| `ProfileStrict` | Stripped | Stripped | Preserved | Preserved | Preserved | Preserved | Preserved | Off |
| `ProfileLoose` | Stripped | Stripped | Folded | Folded | Folded | Folded | Folded | On |
| `ProfileLucene` | 8 Harakat | Stripped | Folded (no wasla)| Preserved | Folded | Folded | Preserved | Off |
| `ProfileCAMeL` | 9 Marks | Preserved | Folded (with wasla)| Preserved | Folded | Folded | Preserved | Off |

#### Predefined profiles

```go
// Strict keeps letter distinctions such as ة vs ه and ى vs ي.
strictKey := arabic.ProfileStrict.Normalize("مكتبة") // "مكتبة"

// Loose also folds digits, punctuation, and consonant skeletons (rasm).
looseKey := arabic.ProfileLoose.Normalize("عام ١٩٨٤، صـ ٤٢") // "عام 1984, ص 42"

// Lucene matches Apache Lucene's ArabicNormalizer on Arabic text.
luceneKey := arabic.ProfileLucene.Normalize(text)
```

#### Custom profiles

Set the `Profile` fields directly and call `Validate` before use:

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

### Languages

Every predefined profile uses Arabic only. For Persian, Urdu, or a mixed
corpus, set `Languages`:

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

With `LanguagePersian`, `LanguageKurdish`, or `LanguageUyghur` selected,
`Languages.PreservesZWNJ()` returns `true` and ZWNJ (U+200C) stays in the key.
In those languages ZWNJ separates suffixes, as in Persian `خانه‌ها`. See
[LANGUAGES.md](LANGUAGES.md) for each language's rules.

### Rasm

Set `Rasm` to compare dotless consonant skeletons, for example in unpointed
manuscripts:

```go
rasmProfile := arabic.ProfileSearch
rasmProfile.Rasm = true

// Folds ب, ت, ث, ن, ي to dotless skeleton ٮ
// Folds ج, ح, خ to ح
// Folds ف, ق to ٯ
fmt.Println(rasmProfile.Normalize("بيت")) // "ٮٮٮ"
fmt.Println(rasmProfile.Normalize("بيت") == rasmProfile.Normalize("نبت")) // true
```

### Mapping offsets back to the original text

Normalization removes characters (tashkil, tatweel) and expands others (`ﻻ`
becomes `ل` + `ا`), so a byte offset in the key is not an offset in the
original text. `NormalizeMapped` returns an index alongside the key, and
`MapSpan` uses it to convert a key range into an original-text range:

```go
text := "هذِهِ مَدْرَسَةٌ مُبَارَكَةٌ"

normKey, idx := arabic.ProfileSearch.NormalizeMapped(text)
// normKey == "هذه مدرسه مباركه"

// Byte offsets of "مدرسه" in the key.
normStart := strings.Index(normKey, "مدرسه") // 7
normEnd := normStart + len("مدرسه")           // 17

origStart, origEnd := arabic.MapSpan(idx, normStart, normEnd)
matchedSlice := text[origStart:origEnd]

fmt.Printf("Original slice: %q\n", matchedSlice)
// Output: Original slice: "مَدْرَسَةٌ"
```

#### Reusing the index buffer

In a loop, `NormalizeMappedInto` reuses the index slice instead of allocating
a new one for every line:

```go
var idxBuf []int32

for _, line := range lines {
	normKey, idx := profile.NormalizeMappedInto(line, idxBuf)
	idxBuf = idx // keep the capacity for the next line
	// ... process spans ...
}
```

## Package `match`

`match` defines the `Matcher` interface and three implementations:

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

Build a `Matcher` once with a `Profile`, then call `FindAll` on text
normalized with that same profile. Matchers are safe for concurrent use.

### Literal (`NewLiteral`, `NewLiterals`)

Finds the query key as a substring of the normalized text. `NewLiterals`
takes several queries and an `ignoreCase` flag:

```go
import "github.com/MohamedElashri/agrep/match"

profile := arabic.ProfileSearch

// Single query
m, err := match.NewLiteral("مدرسه", profile)
if err != nil {
	log.Fatal(err)
}

// Several queries, no case folding
queries := []string{"جامعة", "معهد", "كلية"}
multiM, err := match.NewLiterals(queries, profile, false)

// Find all matches in normalized text
haystack := profile.Normalize("تخرج من جامعة القاهرة")
for _, span := range multiM.FindAll(haystack) {
	fmt.Printf("Match: %q [%d:%d]\n", haystack[span.Start:span.End], span.Start, span.End)
}
```

### Fuzzy (`NewFuzzy`)

Finds matches within a Levenshtein distance using Myers' bit-vector
algorithm. Distance counts codepoints, not UTF-8 bytes.
[MATCHING.md](MATCHING.md) describes how overlapping candidates are chosen.

```go
// At most one inserted, deleted, or substituted codepoint
fuzzyM, err := match.NewFuzzy([]string{"خوارزمي"}, arabic.ProfileSearch, false, 1)
if err != nil {
	log.Fatal(err)
}

// The text is missing ر.
input := arabic.ProfileSearch.Normalize("عالم الرياضيات الخوازمي الشهير")
spans := fuzzyM.FindAll(input)
for _, span := range spans {
	fmt.Printf("Fuzzy match: %q\n", input[span.Start:span.End]) // "خوازمي"
}
```

### Regex (`NewRegex`)

Runs Go `regexp` syntax over the normalized text. The pattern is not
normalized, so write it in key form: under `ProfileSearch`, `ة` has already
become `ه`. Go's `\b` and `\w` are ASCII-only and do not match Arabic letters.

```go
patterns := []string{`مدر[سص]ه`}
reM, err := match.NewRegex(patterns, arabic.ProfileSearch, false)
if err != nil {
	log.Fatal(err)
}

spans := reM.FindAll(arabic.ProfileSearch.Normalize("مدرسة جديدة")) // [{0 10}]
```

## Package `scan`

`scan.Search` reads lines from an `io.Reader`, normalizes each line with the
matcher's profile, and calls `onMatch` for every selected or context line.
Unlike `bufio.Scanner`, it has no 64 KiB token limit. If `onMatch` returns an
error, `Search` stops and returns that error.

```go
func Search(
	r io.Reader,
	m match.Matcher,
	opts Options,
	onMatch func(Match) error,
) (found bool, err error)
```

### Options

```go
type Options struct {
	MaxLineBytes  uint64 // Max line length after removing CR/LF; 0 means unlimited
	File          string // Copied to Match.File
	InvertMatch   bool   // Select non-matching lines (grep -v)
	BeforeContext int    // Preceding context lines (grep -B)
	AfterContext  int    // Following context lines (grep -A)
	MapSpans      bool   // Fill Match.Spans with original-text offsets; allocates
	WordRegexp    bool   // Keep only matches bounded by non-word runes
	OmitText      bool   // Leave Match.Text empty; not allowed with context or MapSpans
	ExistenceOnly bool   // Emit only the first selected line; still reads the rest of the input
}
```

### Match

```go
type Match struct {
	File       string    `json:"file,omitempty"`
	Line       int64     `json:"line"`
	Text       string    `json:"text"`              // Original line without CR/LF
	Context    bool      `json:"context,omitempty"` // True for context lines
	GroupStart bool      `json:"-"`                 // Marks discontinuity between match groups
	Spans      []Span    `json:"spans,omitempty"`   // Byte ranges in Text
}

type Span [2]int // Half-open [start, end); serialized as [start,end]
```

### Example

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

## Recipes

### Search documents held in memory

Normalize each document once when it is added, then run matchers against the
stored keys:

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

### Wrap matches in `<mark>`

Highlight matches in the original voweled text. The example writes the text as
is; escape it first if it comes from users.

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

### Persian and Arabic in one corpus

A Persian query also matches a voweled copy of the word:

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
		"دَانِشْگَاه في طهران",       // with vowel marks
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

### Search a file with context lines

Print each match with two lines before and after it, in grep's
`file:line:text` format:

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

## Performance notes

- Build a `Matcher` once and reuse it. Construction normalizes the queries,
  derives raw-byte filters for literal queries, and builds fuzzy bit masks.
- Reuse the index buffer with `NormalizeMappedInto` when mapping spans in a
  loop.
- Leave `MapSpans` off unless you need offsets. Mapping allocates.
- To count lines or test for a match, set `OmitText`. With a literal matcher
  and no `MaxLineBytes`, `InvertMatch`, `WordRegexp`, or context, `Search`
  checks 256 KiB blocks for the matcher's raw-byte anchors and skips blocks
  that contain none, without splitting them into lines.
- To stop early, return an error from `onMatch`.
