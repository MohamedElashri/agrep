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

Both query and input lines are normalized with Unicode NFD, then:

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

## Library use

The CLI is a thin wrapper around three importable packages:

- [`arabic`](arabic) — `arabic.Normalize(s string) string`, the normalization
  described above, with no CLI or I/O dependency.
- [`match`](match) — `match.NewLiteral(query string) (Matcher, error)` builds
  a reusable matcher from a query once; `Matcher.FindAll(normalized string)
  []Span` finds every occurrence in already-normalized text.
- [`scan`](scan) — `scan.Search(r io.Reader, m match.Matcher, opts
  scan.Options, onMatch func(scan.Match) error) (bool, error)` streams
  arbitrary-length logical lines from a reader and reports the ones that
  match.

```go
import "github.com/MohamedElashri/agrep/arabic"

voweled := "مَدْرَسَةٌ"
unvoweled := "مدرسه"
arabic.Normalize(voweled) == arabic.Normalize(unvoweled) // true
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
