# Input encodings, transliteration, and document formats

## Legacy encodings

The default input encoding remains UTF-8. Use `--encoding` when searching older
Arabic text:

```sh
agrep --encoding=cp1256 'مدينه' legacy.txt
agrep --encoding=iso-8859-6 'مدينه' archive.txt
agrep --encoding=utf16le 'مدينه' export.txt
agrep --encoding=auto 'مدينه' unknown.txt
```

Supported values are `utf8`, `cp1256`, `iso-8859-6`, `utf16le`, `utf16be`, and
`auto`. UTF-16 modes accept an optional byte-order mark. Auto detection gives a
UTF-8 or UTF-16 BOM priority, then scores UTF-8, Windows-1256, ISO-8859-6, and
both UTF-16 byte orders by decoded Arabic-codepoint frequency. Windows-1256 wins
ties with ISO-8859-6 because the encodings overlap for many Arabic letters and
Windows-1256 is the more common source format.

Decoding happens before line scanning. `Match.Text`, `--max-line-bytes`, `-o`,
highlighting, and JSON `spans` therefore all refer to decoded UTF-8 text. They
never refer to byte positions in the encoded source file. With the default
`utf8` mode, malformed input remains an error and the diagnostic suggests
`--encoding=auto`.

## Transliteration

Latin-keyboard queries can be converted before normalization and matching:

```sh
agrep --translit=buckwalter 'ktAb' corpus.txt
agrep --translit=arabtex 'kitAb' corpus.txt
agrep --translit=iso233 'kitāb' corpus.txt
```

All three commands search for `كتاب`. Unrecognized query characters are copied
unchanged. `--translit` cannot be combined with `--regex`, because rewriting an
arbitrary regular expression as Arabic would require parsing and preserving its
operators and character classes.

Buckwalter uses the direct character mapping documented by CAMeL Tools. ArabTeX
and ISO 233 include contextual writing conventions in addition to character
values. Their implementation here intentionally covers the documented
character-level forms used for search; it does not reproduce ArabTeX's
typesetter or ISO 233's contextual vowel and diphthong rules.

`--translit-out` renders emitted text as Buckwalter for terminals without an
Arabic font:

```sh
printf '%s\n' 'هذا كتاب' | agrep --translit=buckwalter --translit-out 'ktAb'
```

The whole emitted line is rendered. Span boundaries are remapped into that
rendering, so `-o`, color, and JSON offsets continue to reference the adjacent
text rather than stale Arabic byte positions.

Primary mapping references:

- [CAMeL Tools Buckwalter table](https://camel-tools.readthedocs.io/en/stable/reference/encoding_schemes.html)
- [ArabTeX package documentation](https://mirrors.ctan.org/language/arabic/arabtex/arabtex-doc.pdf)
- [ISO 233:1984](https://www.iso.org/standard/4117.html)

## Optional HTML and EPUB extraction

Build with the `formats` tag to extract visible text from HTML and EPUB files:

```sh
go build -tags formats -o agrep ./cmd/agrep
agrep 'مدينه' page.html book.epub
```

HTML honors a declared character set, ignores script, style, template, SVG,
and noscript content, and converts block elements to logical text lines. EPUB
follows the package spine rather than ZIP entry order and extracts its HTML or
XHTML items. Extracted text is UTF-8 and is capped at 256 MiB per document. An
explicit legacy `--encoding` cannot be applied after document extraction; use
UTF-8 or auto.

PDF and DOCX extraction remain deferred. They need separate parsers and a clear
policy for layout order, embedded fonts, and dependency maintenance. The
default build remains a plain-text binary and does not link the HTML parser.
