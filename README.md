# agrep

[![CI](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml/badge.svg)](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml)

`agrep` searches Arabic-script text across spelling and Unicode variants. It can
ignore diacritics, fold common letter forms, search transliterated queries, and
return original-text spans as JSON Lines.

## Install

Download the archive for your operating system from
[GitHub Releases](https://github.com/MohamedElashri/agrep/releases), verify its
SHA-256 hash against `checksums.txt`, extract it, and put `agrep` on your `PATH`.
Check the installation with `agrep --version`.

With Go installed, you can also install the latest published version:

```sh
go install github.com/MohamedElashri/agrep/cmd/agrep@latest
```

The standard binary searches text files. HTML and EPUB extraction requires a
[source build with `-tags formats`](docs/INPUT.md#optional-html-and-epub-extraction).

## Start searching

```sh
agrep 'مدرسه' book.txt                         # also finds مَدْرَسَة
agrep -r -n --include '*.txt' 'كتاب' books/    # recursive search
agrep --translit=buckwalter 'ktAb' corpus.txt  # Latin keyboard → كتاب
agrep --json 'احمد' people.txt                 # JSON Lines with byte spans
```

With no path, `agrep` reads standard input. Use `-` as a path to mix standard
input with files. Exit status is `0` for a selection, `1` for none, and `2` for
an error.

## Choose a search mode

| Need | Option | Detail |
| --- | --- | --- |
| Exact spelling distinctions | `--profile=strict` | Keeps hamza, ta-marbuta, and alef-maksura distinct. |
| More recall | `--profile=loose` or `--rasm` | Can produce extra matches. |
| Misspellings or OCR errors | `--fuzzy[=N]` | Edit distance over normalized Unicode codepoints. |
| Regular expressions | `--regex` | Pattern syntax is evaluated over normalized text. |
| Persian, Urdu, and other orthographies | `--lang=fa`, `--lang=ar,fa`, etc. | Selects language-specific equivalences. |
| Legacy files | `--encoding=auto` or an explicit encoding | Decodes before searching. |

The matching line is displayed with its original spelling. JSON `spans` and
`-o` refer to decoded UTF-8 byte offsets in that original line.

## Reference

- [All CLI options and output formats](docs/CLI.md)
- [Normalization rules and profiles](docs/NORMALIZATION.md)
- [Languages](docs/LANGUAGES.md), [rasm and fuzzy matching](docs/MATCHING.md),
  [encodings and transliteration](docs/INPUT.md)
- [Terminal color](docs/TERMINALS.md) and [normalization playground](docs/PLAYGROUND.md)
- [Benchmarks](docs/BENCHMARKS.md) and [development](docs/DEVELOPMENT.md)

## Go packages

`arabic` normalizes text, `match` builds reusable literal, regex, or fuzzy
matchers, and `scan` streams matching lines. The CLI uses the same packages.

```go
import "github.com/MohamedElashri/agrep/arabic"

same := arabic.Normalize("مَدْرَسَةٌ") == arabic.Normalize("مدرسه") // true
_ = same
```

See the package documentation for
[`arabic`](https://pkg.go.dev/github.com/MohamedElashri/agrep/arabic),
[`match`](https://pkg.go.dev/github.com/MohamedElashri/agrep/match), and
[`scan`](https://pkg.go.dev/github.com/MohamedElashri/agrep/scan).

## License

MIT. See [LICENSE](LICENSE).
