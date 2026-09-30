# agrep

[![Release](https://img.shields.io/github/v/release/MohamedElashri/agrep?color=0284c7&label=release)](https://github.com/MohamedElashri/agrep/releases)
[![CI](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml/badge.svg)](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/MohamedElashri/agrep.svg)](https://pkg.go.dev/github.com/MohamedElashri/agrep)
[![Playground](https://img.shields.io/badge/Playground-Live%20Demo-10b981?logo=webassembly&logoColor=white)](https://mohamedelashri.github.io/agrep/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`agrep` is an ultrafast, Unicode-aware search tool tailored for Arabic-script text. It matches across spelling variants, diacritics (*tashkil*), letter forms, consonant skeletons (*rasm*), and transliterated queries while preserving original source lines and exact byte offsets.

```sh
# Matches across diacritics, alef forms, and ta-marbuta:
agrep 'مدرسه' book.txt                         # matches مَدْرَسَة and مدرسة
agrep -r -n --include '*.txt' 'كتاب' books/    # recursive search with line numbers
agrep --translit=buckwalter 'ktAb' corpus.txt  # Latin keyboard query → كتاب
agrep --json 'احمد' people.txt                 # JSON Lines with exact byte spans
```

With no path, `agrep` reads standard input. Pass `-` as a path argument to mix standard input with files. Exit status follows standard grep conventions: `0` for match, `1` for no match, and `2` for an error.

## Install

### Shell Script (macOS, Linux, BSD)

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | bash
```

### Homebrew (macOS & Linux)

```sh
brew install MohamedElashri/tap/agrep
```

### Arch Linux (AUR)

```sh
yay -S agrep-bin
```

### Go Install

```sh
go install github.com/MohamedElashri/agrep/cmd/agrep@v0.1.0
```

### Pre-compiled Binaries

Download standalone archives for Linux, macOS, FreeBSD, OpenBSD, and NetBSD (`amd64`, `arm64`) from [GitHub Releases](https://github.com/MohamedElashri/agrep/releases/tag/v0.1.0), and verify hashes against `checksums.txt`.

```sh
# GitHub CLI
gh release download v0.1.0 -R MohamedElashri/agrep
```

### Build from Source

```sh
git clone https://github.com/MohamedElashri/agrep.git
cd agrep && go build -ldflags="-s -w" ./cmd/agrep
```

HTML and EPUB extraction can be included by adding `-tags formats` during compilation. See [INPUT.md](docs/INPUT.md#optional-html-and-epub-extraction).

### Updating

Check for new releases or self-update in place:

```sh
agrep --check-update    # check if a newer release is available
agrep --update          # download, verify, and update in place
agrep --update=v0.2.0   # update or switch to a specific version
```

## Search modes

| Need | Option | Detail |
| :--- | :--- | :--- |
| Exact spelling distinctions | `--profile=strict` | Keeps hamza, ta-marbuta, and alef-maksura distinct |
| High recall / skeleton | `--profile=loose` or `--rasm` | Folds consonants to dotless skeletons |
| Typographical / OCR errors | `--fuzzy[=N]` | Levenshtein edit distance over normalized codepoints |
| Regular expressions | `--regex` | Pattern syntax evaluated over normalized text |
| Non-Arabic orthographies | `--lang=fa`, `--lang=ur`, etc. | Language-specific equivalences (Persian, Urdu, Kurdish) |
| Legacy encodings | `--encoding=auto` | Automatic CP1256, ISO-8859-6, and UTF-16 decoding |

The matching line is always printed with its original source spelling. Emitted JSON `spans` and `-o` refer to decoded UTF-8 byte offsets in that original line.

## Documentation

| Topic | Document | Description |
| :--- | :--- | :--- |
| CLI Reference | [CLI.md](docs/CLI.md) | Full list of flags, options, and JSON Lines format |
| Normalization | [NORMALIZATION.md](docs/NORMALIZATION.md) | Unicode rules and profiles (`search`, `strict`, `loose`, `lucene`, `camel`) |
| Matching Engines | [MATCHING.md](docs/MATCHING.md) | Dotless rasm, Myers fuzzy Levenshtein distance, regex |
| Languages & Encodings | [LANGUAGES.md](docs/LANGUAGES.md), [INPUT.md](docs/INPUT.md) | Persian, Urdu, Kurdish orthographies and transliteration |
| Web Playground | [PLAYGROUND.md](docs/PLAYGROUND.md) | [Live interactive demo](https://mohamedelashri.github.io/agrep/) running in WebAssembly |
| Benchmarks | [BENCHMARKS.md](docs/BENCHMARKS.md) | Performance metrics and comparison suites |
| Agent Integration | [Agent Skill](.agents/skills/agrep-search/SKILL.md) | Ready-to-use search skill for Codex and Claude Code |

## Go packages

`agrep` is also available as modular Go packages: `arabic` for Unicode normalization, `match` for literal, regex, and fuzzy engines, and `scan` for line-by-line streaming.

```go
import "github.com/MohamedElashri/agrep/arabic"

// Normalized comparison treats diacritics and letter variants as equivalent:
same := arabic.Normalize("مَدْرَسَةٌ") == arabic.Normalize("مدرسه") // true
```

Documentation: [`arabic`](https://pkg.go.dev/github.com/MohamedElashri/agrep/arabic) · [`match`](https://pkg.go.dev/github.com/MohamedElashri/agrep/match) · [`scan`](https://pkg.go.dev/github.com/MohamedElashri/agrep/scan).

## License

MIT License. See [LICENSE](LICENSE).
