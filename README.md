# agrep

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
go install .
go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o agrep .
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

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
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
