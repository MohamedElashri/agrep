---
title: "Command-Line Reference"
description: "Complete reference for agrep CLI flags, options, exit codes, and JSON Lines output format."
category: "Getting Started"
order: 2
---

# Command-line reference

```text
agrep [options] <query> [path ...]
agrep [options] -e <query>... [path ...]
```

Searches standard input when no path is given. `-` reads standard input among
other paths. Matching is literal after normalization unless a mode says
otherwise. Repeated `-e` patterns are ORed.

## Search and file selection

| Option | Effect |
| --- | --- |
| `-e PATTERN` | Add a query; repeat for alternatives. |
| `-i`, `--ignore-case` | Unicode case folding. Literal search handles expansions such as `ß` → `ss`; regex uses Go's simple-fold rules. |
| `-v`, `--invert-match` | Select nonmatching lines. |
| `-w`, `--word-regexp` | Require word boundaries in the original text. |
| `--regex` | Evaluate a regular expression over normalized text; the pattern itself is not normalized. |
| `--fuzzy[=N]` | Allow at most `N` insertions, deletions, or substitutions; default `N=1`. |
| `-r`, `--recursive` | Walk directories, or the current directory if no path is supplied. |
| `--include GLOB` | Include matching paths; repeatable. |
| `--exclude GLOB` | Exclude matching paths or directories; repeatable. |
| `--no-ignore` | Search files normally excluded by `.gitignore`. |
| `--threads N` | Parallel file workers; default is available CPUs. Output order is deterministic. |
| `--encoding NAME` | Input: `utf8` (default), `cp1256`, `iso-8859-6`, `utf16le`, `utf16be`, or `auto`. |
| `--translit NAME` | Convert Latin query input using `buckwalter`, `arabtex`, or `iso233`. |

`--regex` cannot be combined with `--fuzzy` or `--translit`. See
[matching](MATCHING.md), [languages](LANGUAGES.md), and [input](INPUT.md) for
semantics that affect results.

## Output

| Option | Effect |
| --- | --- |
| `-j`, `--json` | One JSON object per selected or context line. |
| `-n`, `--line-number` | Prefix human output with 1-based line numbers. |
| `-c`, `--count` | Print selected-line counts. |
| `-l`, `--files-with-matches` | Print names of files with selected lines. |
| `-L`, `--files-without-match` | Print names of files without selected lines. |
| `-A N`, `--after-context N` | Include `N` following lines. |
| `-B N`, `--before-context N` | Include `N` preceding lines. |
| `-C N`, `--context N` | Include `N` lines on both sides. |
| `-H`, `--with-filename` | Always prefix human output with file names. |
| `-h`, `--no-filename` | Never prefix human output with file names. |
| `-o`, `--only-matching` | Emit only matching original-text spans. |
| `--color WHEN` | `auto` (default), `always`, or `never`. |
| `--no-bidi-isolate` | Omit RTL isolates around colored Arabic text. |
| `--translit-out` | Render emitted text as Buckwalter. |

`-c`, `-l`, and `-L` are mutually exclusive and cannot be combined with JSON.
`-o` cannot be combined with inverted, summary, or context output. Color in
`auto` mode follows the terminal and honors `NO_COLOR` and `TERM=dumb`.

JSON Lines fields are `line` (1-based), `text` (original decoded line), optional
`file`, optional `spans`, and `context: true` for neighboring lines. Each span
is a half-open byte range `[start,end]` in `text`. Context lines omit spans.
Streaming output can be partial if an input or output error occurs; check the
exit status.

## Normalization

| Option | Effect |
| --- | --- |
| `--profile NAME` | `search` (default), `strict`, `loose`, `lucene`, or `camel`. |
| `--lang LIST` | Comma-separated `ar`, `fa`, `ur`, `ps`, `ku`, `ug`; default `ar`. |
| `--rasm` | Fold selected Arabic consonants to dotless skeletons. |
| `--keep-hamza` | Preserve hamza and madda spelling variants. |
| `--keep-tamarbuta` | Preserve `ة` versus `ه`. |
| `--keep-tashkil` | Preserve diacritics. |
| `--keep-presentation-forms` | Preserve Arabic ligatures and contextual letter forms. |
| `--keep-joiners` | Preserve ZWNJ and ZWJ. |
| `--keep-bidi-marks` | Preserve bidi controls. |
| `--keep-quranic-marks` | Preserve Quranic annotation marks. |
| `--fold-digits` | Fold Arabic-Indic and Extended Arabic-Indic digits to ASCII. |
| `--fold-punctuation` | Fold supported Arabic punctuation to ASCII. |
| `--max-line-bytes N` | Reject decoded logical lines longer than `N` bytes; `0` means unlimited. |

Overrides apply on top of the selected profile. See the
[normalization reference](NORMALIZATION.md) for exact codepoints and the
[language reference](LANGUAGES.md) for cross-language behavior.

## Updates

| Option | Effect |
| --- | --- |
| `--check-update` | Check GitHub for the latest release and report whether an update is available. |
| `--update` | Self-update agrep in place to the latest release. |
| `--update=TAG` | Update or switch agrep in place to a specific release tag (e.g. `v0.2.0`). |

The updater securely checks and verifies the SHA-256 checksum against official release `checksums.txt` before applying the update. Executable replacement is atomic. If `GITHUB_TOKEN` or `GH_TOKEN` is present in the environment, it is sent with requests to avoid rate limits, with automatic fallback to release redirect resolution.

`--help` prints the built-in help; `--version` prints the build version. Exit
status is `0` when something is selected, `1` when nothing is selected, and `2`
for an argument, input, or output error.
