---
name: agrep-search
description: Search local Arabic-script text with agrep when spelling, diacritics, script variants, transliteration, or original-text match spans matter. Use for corpus discovery and evidence extraction; ordinary source-code searches do not need this skill.
---

# Search Arabic-script text with agrep

Use this repository's `agrep` CLI. If it is not installed, build it once from the repository root with `go build -o ./agrep ./cmd/agrep` and reuse `./agrep`; do not recompile for every query. The root binary is gitignored. Check `agrep --version` if an unrelated system command may share the name. In the examples below, replace `agrep` with `./agrep` when using a local build.

Choose the smallest useful output:

- Discover files with `agrep -r -l --include '*.txt' 'مدرسه' corpus/`.
- Count selected lines with `agrep -c 'مدرسه' file.txt`.
- Inspect matching lines with `agrep -n 'مدرسه' file.txt`.
- Request machine-readable evidence with `agrep --json 'مدرسه' file.txt`. Parse one JSON object per line. `text` preserves the decoded original; `spans` are half-open UTF-8 byte offsets within `text`, not character positions. Context records can omit spans.

Start with the default `--profile=search --lang=ar`. Use `--lang=fa` or `--lang=ar,fa` when the corpus calls for Persian equivalences; choose other language codes from [docs/LANGUAGES.md](../../../docs/LANGUAGES.md). Use `--profile=strict` when spelling distinctions matter. `--profile=loose`, `--rasm`, and `--fuzzy=1` increase recall and can increase work and false positives. Literal matching is the default; `--regex` evaluates pattern syntax over normalized text and does not normalize the pattern itself. For legacy files, select `--encoding=auto` or an explicit encoding.

Keep searches scoped with paths and `--include`/`--exclude` before requesting full JSON output. Recursive search honors ignore files unless `--no-ignore` is set. Treat exit code 0 as a selection, 1 as no selection, and 2 as an error. JSON output can be partial on an error, so check the exit code before treating it as complete evidence.

For the complete option and output contract, read [docs/CLI.md](../../../docs/CLI.md). If your agent harness does not discover repository skills automatically, point it at this `SKILL.md` rather than copying these instructions into every prompt.
