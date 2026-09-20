# agrep

[![CI](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml/badge.svg)](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml)

`agrep` (Arabic Grep) searches Arabic-script UTF-8 text while tolerating
tashkil, tatweel, canonical Unicode differences, and explicitly selected
orthographic variants. It is small enough for shell use and has a stable JSON
Lines mode for tool-calling agents.

Search Arabic from a Latin keyboard without an IME:

```sh
agrep --translit=buckwalter "ktAb" corpus.txt     # finds كتاب
```

## Install

Download the archive for your operating system and architecture from the
[GitHub Releases](https://github.com/MohamedElashri/agrep/releases) page, extract
it, and place `agrep` somewhere on your `PATH`.

From a checkout:

```sh
go install ./cmd/agrep
go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o agrep ./cmd/agrep
```

## Usage

```text
agrep [options] <query> [path ...]
agrep [options] -e <query>... [path ...]
```

Human-readable output is the default:

```sh
agrep -n "مدرسه" book.txt
printf '%s\n' "هذه مَدْرَسَة" | agrep "مدرسة"
```

For agents and scripts, use JSON Lines and check the exit status:

```sh
agrep --json "احمد" people.txt
```

```json
{"line":12,"text":"أحمد وصل مبكرا","spans":[[0,8]]}
```

For a file input, JSON adds the optional `file` field without changing the
existing `line` and `text` fields:

```json
{"file":"people.txt","line":12,"text":"أحمد وصل مبكرا","spans":[[0,8]]}
```

`spans` are half-open byte ranges in the original `text`, not in its normalized
comparison key. When context output is requested, neighboring records contain
`"context":true` and omit `spans`.

Recursive and multi-file search supports the familiar grep controls:

```sh
agrep -r -n --include='*.txt' "مدرسه" books/
agrep -i -e "أحمد" -e "STRASSE" corpus-a.txt corpus-b.txt
agrep -C 2 "كتاب" chapter.txt
agrep -l "المدينه" texts/*.txt
agrep -w --color=always "كتاب" chapter.txt
agrep --regex '^مدرس[هة]$' words.txt
agrep -o "مدينه" people.txt
agrep --encoding=auto "مدينه" legacy.txt
agrep --translit-out --translit=buckwalter "ktAb" corpus.txt
agrep --lang=ar,fa "كتاب" mixed-arabic-persian.txt
```

Recursive searches honor `.gitignore` files by default. Use `--no-ignore` to
search ignored files, `--exclude` to remove paths, and `--threads N` to control
the parallel file workers. Multi-file results are emitted in deterministic
input/walk order even when workers finish out of order. `-c`, `-l`, `-L`,
`-v`, `-A`, `-B`, `-C`, `-H`, and `-h` follow their grep meanings. Run
`agrep --help` for the full list.

`--regex` evaluates expressions over normalized text. Patterns themselves are
not normalized because doing so would corrupt regex syntax. For example, `ة` in
a pattern does not match under the default profile, which folds input `ة` to
`ه`; use `--profile=strict` when that distinction belongs in the expression.
With `--regex`, `-i` follows Go regexp's Unicode simple-fold semantics; literal
`-i` uses full default case folding and therefore also handles expansions such
as `ß` to `ss`.

`-w` checks word boundaries in the original text after mapping normalized
matches back. `-o` likewise prints the exact original spelling, including
tashkil and tatweel inside the mapped span.

`--lang=ar|fa|ur|ps|ku|ug` selects one or more Arabic-script orthographies;
the default is Arabic. A single language preserves its distinct alphabet,
while a mixed selection such as `--lang=ar,fa` folds shared Kaf and Yeh forms
for cross-language search. ZWNJ is preserved for Persian, Sorani Kurdish, and
Uyghur and remains word-internal for `-w`. See
[docs/LANGUAGES.md](docs/LANGUAGES.md) for the exact fold table and rationale.

Legacy CP1256, ISO-8859-6, and UTF-16 input can be decoded with `--encoding`.
`--translit=buckwalter|arabtex|iso233` converts a Latin query to Arabic before
matching, while `--translit-out` renders output as Buckwalter and remaps spans
to that rendered text. See [docs/INPUT.md](docs/INPUT.md) for detection rules,
mapping scope, and the optional HTML/EPUB build.

Color defaults to `auto`, honors `NO_COLOR` and `TERM=dumb`, and wraps colored
Arabic spans in Unicode RTL isolates. Use `--no-bidi-isolate` if a terminal
renders those controls visibly. See [docs/TERMINALS.md](docs/TERMINALS.md) for
the byte layout and verification matrix.

Exit status is `0` for one or more matches, `1` for no matches, and `2` for an
argument, input, or output error. Streaming output can be partial after an
output failure, so consumers must honor the final exit status.

## Normalization

Both query and input lines are normalized with Unicode NFD, then, under the
default profile (`search`) and language (`ar`):

- Arabic presentation forms (contextual letter shapes, ligatures like `ﻻ`
  and `ﷲ`) are expanded to plain letters, which is what lets `agrep` match
  Arabic extracted from a PDF, since PDFs usually encode text this way;
- all Unicode non-spacing marks (`Mn`) and Arabic tatweel are removed;
- `أ`, `إ`, `آ`, and `ٱ` become `ا`;
- `ؤ` becomes `و`, and `ئ` becomes `ي`;
- `ة` becomes `ه`;
- `ى` becomes `ي`;
- ZWNJ/ZWJ, bidi control characters, and Quranic annotation marks are
  removed.

The original, unnormalized matching line is emitted. Matching is literal and
substring-based after normalization. It is case-sensitive unless `-i` is used;
that option applies Unicode default case folding, including multi-codepoint
folds such as `ß` to `ss`. Because normalization is intentionally lossy, it can
produce false positives where distinct Arabic spellings collapse to the same
comparison key.

Queries must be valid UTF-8. Input defaults to UTF-8 and can instead be decoded
with `--encoding`. Logical lines have no built-in size ceiling; use
`--max-line-bytes N` when processing untrusted input. The limit is measured in
decoded UTF-8 bytes. A query that becomes empty after normalization is rejected.

### Profiles

Which rules apply is controlled by `--profile`:

```sh
agrep --profile=strict "مدرسه" book.txt   # keeps hamza/ta-marbuta/alef-maksura distinct
agrep --profile=lucene "مدرسه" book.txt   # matches Apache Lucene's ArabicNormalizer
agrep --profile=camel  "مدرسه" book.txt   # matches CAMeL Tools' normalize_alef_ar/dediac_ar family
```

| Profile | What it does |
| --- | --- |
| `search` (default) | agrep's original behavior, plus (as of the presentation-form/joiner/bidi/Quranic-mark rules above) fixes for text extracted from PDFs and copied from right-to-left web pages. Digit and punctuation folding stay off. |
| `strict` | Strips only cosmetic/encoding-level marks (tashkil, tatweel, presentation forms, joiners, bidi marks, Quranic marks); keeps every letter-level distinction (hamza, ta-marbuta, alef-maksura). |
| `loose` | Every fold this build defines, including digit (`١٢٣`→`123`) and punctuation (`،`→`,`) folding. |
| `lucene` | Matches Apache Lucene's `ArabicNormalizer` with `--lang=ar`, verified against its source. |
| `camel` | Matches the normalization CAMeL Tools users compose from `normalize_alef_ar`/`dediac_ar` and related functions with `--lang=ar`, verified against their source. |

Nine `--keep-*`/`--fold-*` flags apply on top of whichever `--profile` was
selected: `--keep-hamza`, `--keep-tamarbuta`, `--keep-tashkil`,
`--keep-presentation-forms`, `--keep-joiners`, `--keep-bidi-marks`,
`--keep-quranic-marks`, `--fold-digits`, `--fold-punctuation`. Run
`agrep --help` for what each one does. See
[docs/NORMALIZATION.md](docs/NORMALIZATION.md) for exactly what every rule
does, codepoint by codepoint, with citations to the Unicode Character
Database and to the Lucene and CAMeL Tools source each preset is checked
against.

Language selection composes with profiles. Selecting a non-Arabic language on
top of `lucene` or `camel` is an explicit agrep extension, not part of the
upstream compatibility claim.

## Library use

The CLI is a thin wrapper around three importable packages:

- [`arabic`](arabic): `arabic.Normalize(s string) string` (the default
  profile's normalization) and `arabic.Profile`, with `Profile.Normalize`
  for any of the five built-in presets (`ProfileSearch`, `ProfileStrict`,
  `ProfileLoose`, `ProfileLucene`, `ProfileCAMeL`) or a custom combination
  of rules. `Profile.Languages` and `arabic.ParseLanguages` select
  language-aware equivalences. `Profile.NormalizeMapped` returns the identical key plus a byte
  index back to the original string; `arabic.MapSpan` handles expansion
  boundaries correctly. No CLI or I/O dependency.
- [`match`](match): `match.NewLiteral(query string, p arabic.Profile)
  (Matcher, error)` builds a reusable matcher from a query and a profile
  once; `match.NewLiterals` adds repeatable patterns and Unicode case folding.
  `match.NewRegex` evaluates regular expressions over normalized text.
  `Matcher.FindAll(normalized string) []Span` finds every occurrence in text
  normalized under that same profile.
- [`scan`](scan): `scan.Search(r io.Reader, m match.Matcher, opts
  scan.Options, onMatch func(scan.Match) error) (bool, error)` streams
  arbitrary-length logical lines from a reader, normalizes each with the
  matcher's own profile, and reports the ones that match.

```go
import "github.com/MohamedElashri/agrep/arabic"

voweled := "مَدْرَسَةٌ"
unvoweled := "مدرسه"
arabic.Normalize(voweled) == arabic.Normalize(unvoweled) // true

arabic.ProfileStrict.Normalize(voweled) == arabic.ProfileStrict.Normalize(unvoweled) // false: ta-marbuta stays distinct from heh
```

```sh
go get github.com/MohamedElashri/agrep/arabic
```

## Development

Development and CI use Go 1.27.1. The only runtime module dependency is
`golang.org/x/text`, currently v0.42.0.

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
```

`arabic/tables.go` (the presentation-form fold table) is generated from a
vendored copy of the Unicode Character Database; regenerate it with:

```sh
go generate ./arabic/...
```

`go generate` should be a no-op unless `arabic/gen/UnicodeData.txt` or
`arabic/gen/main.go` changed; run it and check `git diff` after touching
either. See [arabic/gen/README.md](arabic/gen/README.md) for provenance and
how to pick up a newer Unicode version.

`arabic`, `match`, and `scan` also have fuzz targets that assert normalization
never panics, always produces valid UTF-8, and is idempotent (across the
*entire* `Profile` flag space, not just the five named presets: this is
what actually found the subtle NFD/canonical-ordering interactions
documented in `arabic/profile.go`'s `Normalize` doc comment), and that
matching and line-scanning never panic on arbitrary input:

```sh
go test -run '^$' -fuzz FuzzProfileNormalizeIdempotent -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzProfileNormalizeValidUTF8 -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzProfileNormalizeNoPanic -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzLanguageNormalizeProperties -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzNewLiteralNoPanic -fuzztime 30s ./match
go test -run '^$' -fuzz FuzzSearchNoPanic -fuzztime 30s ./scan
```

Benchmark baselines live alongside each package (`arabic/bench_test.go`,
`scan/bench_test.go`, `internal/walk/bench_test.go`). The walker benchmarks
include a same-fixture `rg --files` comparison when ripgrep is installed:

```sh
go test -run '^$' -bench . -benchmem ./...
```

## Releases

Releases are built and published by GitHub Actions from semantic version tags.
Each release contains `tar.gz` archives for Linux, macOS, FreeBSD, OpenBSD, and
NetBSD on amd64 and arm64, plus DragonFly BSD on amd64. The archives include the
binary, README, and license; `checksums.txt` contains their SHA-256 checksums.

To publish a release, first make sure the target commit is on the default branch
and its CI checks pass. Then create and push an annotated tag:

```sh
git tag -a v1.2.3 -m "agrep v1.2.3"
git push origin v1.2.3
```

Tags must follow SemVer, such as `v1.2.3` or `v1.2.3-rc.1`. A prerelease tag
creates a GitHub prerelease. GoReleaser generates release notes from commits
since the previous tag and embeds the version without the leading `v` in the
binary; verify it with `agrep --version`.

## LICENSE

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
