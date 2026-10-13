---
title: "Overview & Quickstart"
description: "What agrep does, how a search runs, and how to install and start using it."
category: "Getting Started"
order: 1
---

# agrep documentation

`agrep` searches Arabic-script text. GNU grep and ripgrep compare bytes, so
`مدرسه` does not find `مَدْرَسَة`. agrep normalizes both the query and each
line before comparing them, then prints the original line and the byte offsets
of the match in it. It supports Arabic (Modern Standard and Classical),
Persian, Urdu, Pashto, Sorani Kurdish, and Uyghur.

## Why byte search misses Arabic text

1. **Diacritics (*tashkil*).** Voweled text carries *fathah*, *kasrah*,
   *dammah*, *sukun*, and *shaddah*. A search for `مدرسه` misses `مَدْرَسَة`
   unless the marks are stripped before comparison.
2. **Spelling variants.** Writers interchange `أ`, `إ`, `آ`, and `ا`; `ة`
   (*ta-marbuta*) and `ه`; and `ى` (*alef maksura*) and `ي`.
3. **Presentation forms.** Text extracted from PDFs often uses legacy
   presentation forms such as `ﻻ` (U+FEFB) instead of `ل` + `ا`.
4. **Other orthographies.** Persian/Urdu `ک` and Arabic `ك`, or Persian `ی`
   and Arabic `ي`, look alike but are different codepoints.
5. **Manuscripts and OCR.** Early manuscripts omit consonant dots (*i'jam*),
   so only the skeleton (*rasm*) can be compared. Scanned text adds OCR
   errors, which need edit-distance matching.

## Pipeline

Every input line goes through these stages:

```text
Input (file, stdin, or HTML/EPUB with -tags formats)
                  │
                  ▼
         [ 1. Input decoding ]  ──► UTF-8, CP1256, ISO-8859-6, or UTF-16
                  │
                  ▼
         [ 2. Transliteration ] ──► Buckwalter, ArabTeX, or ISO 233 (query only)
                  │
                  ▼
         [ 3. Normalization ]   ──► Presentation expansion, NFD, tashkil stripping,
                  │                 letter folding, and language rules
                  ▼
         [ 4. Matching ]        ──► Literal, Myers fuzzy, or regex
                  │
                  ▼
         [ 5. Span mapping ]    ──► Maps normalized match bounds back to
                  │                 byte offsets in the original line
                  ▼
         [ 6. Output ]          ──► Colored terminal text (with RTL isolates) or JSON Lines
```

## Quickstart

### Installation

With the install script:

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | bash
```

Or with Go:

```sh
go install github.com/MohamedElashri/agrep/cmd/agrep@latest
```

On Windows, download the zip from
[GitHub Releases](https://github.com/MohamedElashri/agrep/releases/latest).
Windows builds are not tested in CI.

### Examples

#### Ignore diacritics and spelling variants

The default profile ignores tashkil, tatweel (*kashida*), and common spelling variants:

```sh
# Matches 'مَدْرَسَة', 'مدرسه', 'مَـدْرَسَـة'
agrep 'مدرسه' documents/
```

#### Search Arabic and Persian together

Find both Arabic `كتاب` and Persian `کتاب` in a mixed corpus:

```sh
agrep --lang=ar,fa 'كتاب' library/
```

#### Match the consonant skeleton (*rasm*)

For dotless manuscripts, `--rasm` gives `ب`, `ت`, `ث`, `ن`, and `ي` the same key (`ٮ`):

```sh
agrep --rasm 'مستشرق' manuscript.txt
```

#### Tolerate typos and OCR errors

Allow up to `N` inserted, deleted, or substituted codepoints:

```sh
agrep --fuzzy=1 'خوارزمي' scanned_books.txt
```

#### Type the query on a Latin keyboard

Write the query in Buckwalter, ArabTeX, or ISO 233 transliteration:

```sh
agrep --translit=buckwalter 'ktAb' corpus.txt
```

#### JSON output for scripts and agents

One JSON object per line, with line numbers and byte spans:

```sh
agrep --json 'الذكاء الاصطناعي' articles/
```

## Pages

- [Command-Line Reference](CLI.md) — Flags, search modes, JSON output, and the updater.
- [Go Package & API Guide](PACKAGE.md) — Using the `arabic`, `match`, and `scan` packages from Go.
- [Unicode Normalization](NORMALIZATION.md) — Profiles (`search`, `strict`, `loose`, `lucene`, `camel`) and the codepoints each rule touches.
- [Arabic-Script Languages](LANGUAGES.md) — Persian, Urdu, Pashto, Sorani Kurdish, and Uyghur letters, ZWNJ, and cross-language folds.
- [Rasm & Fuzzy Matching](MATCHING.md) — Dotless skeletons and edit-distance matching.
- [Encodings & Transliteration](INPUT.md) — Legacy encodings (CP1256, ISO-8859-6, UTF-16), transliterated queries, and HTML/EPUB input.
- [Terminal Color & BiDi](TERMINALS.md) — How colored Arabic matches stay in place in a terminal.
- [Performance Benchmarks](BENCHMARKS.md) — Timings against GNU grep and ripgrep, and how to reproduce them.
- [Normalization Playground](PLAYGROUND.md) — Try normalization in the browser.
- [Development Guide](DEVELOPMENT.md) — Building, testing, fuzzing, and the agent skill.
- [Distribution & Releases](DISTRIBUTION.md) — Release checklist and package recipes.
