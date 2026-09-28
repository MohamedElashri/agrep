# Encodings and transliteration

## Input encoding

Input defaults to UTF-8. Use `--encoding` for older files:

| Value | Input |
| --- | --- |
| `utf8` (default) | UTF-8; malformed sequences are errors. |
| `cp1256` | Windows-1256. |
| `iso-8859-6` | ISO-8859-6. |
| `utf16le`, `utf16be` | UTF-16 in the stated byte order; optional BOM. |
| `auto` | BOM first; otherwise UTF-8 or the best-scoring legacy candidate. |

```sh
agrep --encoding=cp1256 'مدينه' legacy.txt
agrep --encoding=auto 'مدينه' unknown.txt
```

Auto detection samples up to 64 KiB and favors Arabic text. Ambiguous
Windows-1256/ISO-8859-6 samples favor Windows-1256. When it selects UTF-8,
malformed bytes after the sample are replaced with U+FFFD, so the decoded
stream stays valid UTF-8. Use an explicit encoding when the source is known.

Decoding precedes line scanning. `--max-line-bytes`, JSON spans, `-o`, and
highlighting refer to **decoded UTF-8 bytes**, not source-file byte offsets.

## Latin-keyboard queries

`--translit` converts the query before normalization:

```sh
agrep --translit=buckwalter 'ktAb' corpus.txt
agrep --translit=arabtex 'kitAb' corpus.txt
agrep --translit=iso233 'kitāb' corpus.txt
```

Each example searches for `كتاب`. Unmapped query characters pass through.
`--translit` cannot be combined with `--regex`. ArabTeX and ISO 233 support
the documented character forms used for search, not full typesetting or all
contextual spelling rules.

`--translit-out` renders output as Buckwalter and remaps `-o`, color, and JSON
spans to the rendered text:

```sh
printf '%s\n' 'هذا كتاب' | agrep --translit=buckwalter --translit-out 'ktAb'
```

Mapping references: [Buckwalter](https://camel-tools.readthedocs.io/en/stable/reference/encoding_schemes.html),
[ArabTeX](https://mirrors.ctan.org/language/arabic/arabtex/arabtex-doc.pdf),
[ISO 233](https://www.iso.org/standard/4117.html).

## Optional HTML and EPUB extraction

Build with `-tags formats` to extract visible HTML and EPUB text:

```sh
go build -tags formats -o agrep ./cmd/agrep
agrep 'مدينه' page.html book.epub
```

HTML ignores script, style, template, SVG, and noscript content. EPUB follows
the package spine. Extracted text is UTF-8, capped at 256 MiB per document;
use `--encoding=utf8` or `auto`. Release archives contain the standard text
binary unless a separate formats build is published.
