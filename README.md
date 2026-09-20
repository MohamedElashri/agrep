# agrep

[![CI](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml/badge.svg)](https://github.com/MohamedElashri/agrep/actions/workflows/ci.yml)

`agrep` (Arabic Grep) searches UTF-8 text while tolerating Arabic tashkil,
tatweel, canonical Unicode differences, and common orthographic variants. It is
small enough for shell use and has a stable JSON Lines mode for tool-calling
agents.

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
agrep [options] <query> [file]
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
{"line":12,"text":"أحمد وصل مبكرا"}
```

Exit status is `0` for one or more matches, `1` for no matches, and `2` for an
argument, input, or output error. Streaming output can be partial after an
output failure, so consumers must honor the final exit status.

## Normalization

Both query and input lines are normalized with Unicode NFD, then, under the
default profile (`search`):

- all Unicode non-spacing marks (`Mn`) and Arabic tatweel are removed;
- `أ`, `إ`, `آ`, and `ٱ` become `ا`;
- `ؤ` becomes `و`, and `ئ` becomes `ي`;
- `ة` becomes `ه`;
- `ى` becomes `ي`.

The original, unnormalized matching line is emitted. Matching is literal,
case-sensitive, and substring-based after normalization. Because normalization
is intentionally lossy, it can produce false positives where distinct Arabic
spellings collapse to the same comparison key.

Input and query must be valid UTF-8. Logical lines have no built-in size ceiling;
use `--max-line-bytes N` when processing untrusted input. A query that becomes
empty after normalization is rejected.

### Profiles

Which rules apply is controlled by `--profile`:

```sh
agrep --profile=strict "مدرسه" book.txt   # keeps hamza/ta-marbuta/alef-maksura distinct
agrep --profile=lucene "مدرسه" book.txt   # matches Apache Lucene's ArabicNormalizer
agrep --profile=camel  "مدرسه" book.txt   # matches CAMeL Tools' normalize_alef_ar/dediac_ar family
```

| Profile | What it does |
| --- | --- |
| `search` (default) | agrep's original, fixed behavior — every fold above, on every Unicode diacritic. |
| `strict` | Strips only tashkil/tatweel; keeps every letter-level distinction (hamza, ta-marbuta, alef-maksura). |
| `loose` | Every fold this build defines — currently identical to `search`. |
| `lucene` | Matches Apache Lucene's `ArabicNormalizer`, verified against its source. |
| `camel` | Matches the normalization CAMeL Tools users compose from `normalize_alef_ar`/`dediac_ar` and related functions, verified against their source. |

`--keep-hamza`, `--keep-tamarbuta`, and `--keep-tashkil` each clear the
corresponding rule on top of whichever `--profile` was selected. See
[docs/NORMALIZATION.md](docs/NORMALIZATION.md) for exactly what every rule
does, codepoint by codepoint, with citations to the Lucene and CAMeL Tools
source each preset is checked against.

## Library use

The CLI is a thin wrapper around three importable packages:

- [`arabic`](arabic) — `arabic.Normalize(s string) string` (the default
  profile's normalization) and `arabic.Profile`, with `Profile.Normalize`
  for any of the five built-in presets (`ProfileSearch`, `ProfileStrict`,
  `ProfileLoose`, `ProfileLucene`, `ProfileCAMeL`) or a custom combination
  of rules. No CLI or I/O dependency.
- [`match`](match) — `match.NewLiteral(query string, p arabic.Profile)
  (Matcher, error)` builds a reusable matcher from a query and a profile
  once; `Matcher.FindAll(normalized string) []Span` finds every occurrence
  in text normalized under that same profile.
- [`scan`](scan) — `scan.Search(r io.Reader, m match.Matcher, opts
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

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
```

`arabic`, `match`, and `scan` also have fuzz targets that assert normalization
never panics, always produces valid UTF-8, and is idempotent, and that matching
and line-scanning never panic on arbitrary input:

```sh
go test -run '^$' -fuzz FuzzNormalizeIdempotent -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzNormalizeValidUTF8 -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzNormalizeNoPanic -fuzztime 30s ./arabic
go test -run '^$' -fuzz FuzzNewLiteralNoPanic -fuzztime 30s ./match
go test -run '^$' -fuzz FuzzSearchNoPanic -fuzztime 30s ./scan
```

Benchmark baselines live alongside each package (`arabic/bench_test.go`,
`scan/bench_test.go`):

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
