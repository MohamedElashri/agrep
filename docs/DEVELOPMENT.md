# Development

## Current priorities

- Profile literal misses, voweled text, and mapped output with the
  [benchmark suite](BENCHMARKS.md) before changing search hot paths.
- Run the graphical terminal checks in [TERMINALS.md](TERMINALS.md) on real
  xterm, kitty, Alacritty, and tmux sessions.
- Extend the licensed [conformance corpus](../testdata/corpus/SOURCES.md)
  with representative documents and record provenance for every addition.
- Evaluate root and stem search against a labeled dataset before deciding
  whether to expose it as a CLI mode. Positional output would need mapping
  back to original text.

## Agent usage

The repository includes an `agrep-search` skill for
[Codex](../.agents/skills/agrep-search/SKILL.md) and
[Claude Code](../.claude/skills/agrep-search/SKILL.md). Other harnesses can load
either `SKILL.md` directly. Future work: a bounded-result CLI option that never
truncates JSON records, a stable JSON schema, and task checks for file
discovery, cross-spelling matches, no-match exits, and span extraction.

## Build and test

Use the Go version in `go.mod`. The default build searches text files; the
`formats` tag adds HTML and EPUB extraction.

```sh
go test ./...
go test -race ./...
go vet ./...
go test -tags formats ./...
go generate ./arabic/...
git diff --exit-code -- arabic/tables.go
```

`arabic/tables.go` is generated from the vendored Unicode data described in
[arabic/gen/README.md](../arabic/gen/README.md). Keep generator changes and
generated output together. CI also runs short fuzz jobs; for a focused local
run:

```sh
go test -run '^$' -fuzz '^FuzzAutoDecodeValidUTF8$' -fuzztime 20s ./internal/decode
```

## Benchmark and release workflow

Run the [benchmark suite](BENCHMARKS.md) before reporting performance changes.
Use the same machine, fixture size, and run count for comparisons; keep the
JSON result and disclose modes that do different work. The suite generates
fixtures in a temporary directory.

The [release checklist](DISTRIBUTION.md) covers tags, archives, checksums, and
package recipes. The [playground guide](PLAYGROUND.md) covers local preview and
Pages deployment.
