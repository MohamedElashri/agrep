---
title: "Overview & Quickstart"
description: "Introduction to agrep, architectural overview, search mechanics, and getting started with Arabic-script search."
category: "Getting Started"
order: 1
---

# agrep Documentation

`agrep` is an ultrafast, Unicode-aware search tool tailored specifically for Arabic-script text. While traditional search tools like GNU grep and ripgrep treat text as raw byte sequences or standard Unicode codepoints, `agrep` understands the linguistic, orthographic, and typographical realities of Arabic-script languages.

Whether searching modern Modern Standard Arabic (MSA), Classical Arabic texts, Persian (*Farsi*), Urdu, Pashto, Sorani Kurdish, or Uyghur, `agrep` bridges the gap between how queries are typed and how text appears in digitized corpora.

---

## The Challenge of Arabic Search

Searching Arabic-script corpora with generic regex or byte-matching tools frequently fails to find relevant text:

1. **Diacritics (*Tashkil*):** Texts often include vowel marks (*fathah*, *kasrah*, *dammah*, *sukun*, *shaddah*). A search for `مدرسه` will miss `مَدْرَسَة` unless diacritics are stripped during comparison.
2. **Spelling Variants:** Different spelling habits interchange `أ`, `إ`, `آ`, and `ا`, or `ة` (*ta-marbuta*) and `ه` (*ha*), or `ى` (*alef maksura*) and `ي` (*yeh*).
3. **Presentation Forms & Ligatures:** Extracted text from PDFs often contains legacy Unicode presentation forms (e.g., `ﻻ` U+FEFB) rather than decomposed letter sequences (`ل` + `ا`).
4. **Multi-Language Orthographies:** Letters like `ک` (Persian/Urdu Keheh) and `ك` (Arabic Kaf), or `ی` (Farsi Yeh) and `ي` (Arabic Yeh) look similar but use different codepoints.
5. **Historical Manuscripts & OCR:** Early manuscripts lack consonant dots (*i'jam*), requiring consonant skeleton (*rasm*) matching. Scanned documents introduce OCR errors requiring fuzzy Levenshtein distance.

`agrep` solves all these problems while preserving original text formatting, line numbers, and exact UTF-8 byte offsets for match highlighting.

---

## Architectural Pipeline

Every query and input stream in `agrep` flows through a streaming, zero-allocation pipeline:

```text
Input Stream (File, stdin, or Archive)
                  │
                  ▼
         [ 1. Input Decoding ]  ──► CP1256, ISO-8859-6, UTF-16, or UTF-8
                  │
                  ▼
         [ 2. Transliteration ] ──► Buckwalter, ArabTeX, or ISO-233 query decoding
                  │
                  ▼
         [ 3. Normalization ]   ──► Presentation expansion, NFD, tashkil stripping,
                  │                 letter folding, and language adjustments
                  ▼
         [ 4. Match Engine ]    ──► Streaming Literal, Myers Fuzzy, or Regex
                  │
                  ▼
         [ 5. Span Mapping ]    ──► Re-maps normalized match bounds back to
                  │                 exact source-line byte offsets
                  ▼
         [ 6. Formatted Output ]──► Colored terminal (with RTL isolates) or JSON Lines
```

---

## Quickstart

### Installation

Install via the automated install script:

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | bash
```

Or install via Go:

```sh
go install github.com/MohamedElashri/agrep/cmd/agrep@latest
```

### Common Search Scenarios

#### 1. Diacritic-Insensitive Search

Matches across full tashkil, tatweel (*kashida*), and spelling variations:

```sh
# Matches 'مَدْرَسَة', 'مدرسه', 'مَـدْرَسَـة'
agrep 'مدرسه' documents/
```

#### 2. Cross-Language Search (Persian & Arabic)

Match both Arabic `كتاب` and Persian `کتاب` in mixed corpora:

```sh
agrep --lang=ar,fa 'كتاب' library/
```

#### 3. High-Recall Skeleton Search (*Rasm*)

Search early manuscripts or dotless text where `ب`, `ت`, `ث`, `ن`, and `ي` share the same skeleton (`ٮ`):

```sh
agrep --rasm 'مستشرق' manuscript.txt
```

#### 4. Typo and OCR-Tolerant Search (*Fuzzy*)

Allow edit distance up to $N$ codepoints using bit-parallel Myers algorithms:

```sh
agrep --fuzzy=1 'خوارزمي' scanned_books.txt
```

#### 5. Latin Keyboard Transliteration

Search Arabic text using Buckwalter or ArabTeX transliteration without switching keyboard layouts:

```sh
agrep --translit=buckwalter 'ktAb' corpus.txt
```

#### 6. Structured Output for Tooling and Agents

Stream JSON Lines with exact line numbers and byte spans:

```sh
agrep --json 'الذكاء الاصطناعي' articles/
```

---

## Documentation Guide

Explore the sections of the documentation to master `agrep`:

- [Command-Line Reference](CLI.md) — Comprehensive guide to flags, search modes, JSON schema, and updater options.
- [Go Package & API Guide](PACKAGE.md) — Embedding agrep as a Go library: linguistic normalization, matchers, streaming scanner, and span mapping.
- [Unicode Normalization](NORMALIZATION.md) — Normalization presets (`search`, `strict`, `loose`, `lucene`, `camel`), rule tables, and citations.
- [Arabic-Script Languages](LANGUAGES.md) — Alphabet rules, ZWNJ handling, and cross-language folds for Persian, Urdu, Pashto, Sorani Kurdish, and Uyghur.
- [Rasm & Fuzzy Matching](MATCHING.md) — Consonant skeletons, Levenshtein distance semantics, and Myers algorithm details.
- [Encodings & Transliteration](INPUT.md) — Automatic codepage detection (CP1256, ISO-8859-6, UTF-16) and transliteration schemes.
- [Terminal Color & BiDi](TERMINALS.md) — Directional isolates (RLI/PDI) and terminal compatibility.
- [Performance Benchmarks](BENCHMARKS.md) — Throughput measurements and comparison against ripgrep and GNU grep.
- [Normalization Playground](PLAYGROUND.md) — Interactive browser-based WebAssembly demo.
- [Development Guide](DEVELOPMENT.md) — Compiling, fuzzing, running test suites, and AI agent integration.
- [Distribution & Releases](DISTRIBUTION.md) — GoReleaser configuration, release verification, and package recipes.
