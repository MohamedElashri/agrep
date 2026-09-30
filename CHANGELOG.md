# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2026-09-30

### Added

- **CLI Self-Update & Version Check**:
  - Added `--check-update` flag to check GitHub for newer releases and compare with the current version without modifying files.
  - Added `--update` flag (and `--update=TAG`) to download, verify, and atomically self-update the running `agrep` binary in place.
  - Added `agrep self-update` (and `agrep self-update check`) command alias.
- **Cryptographic & Integrity Verification**:
  - Cryptographic verification of downloaded release archives against official `checksums.txt` SHA-256 hashes prior to extraction.
  - Path traversal (Zip Slip) protection and decompression size limits (`io.LimitReader`) during archive extraction.
- **Atomic Binary Replacement**:
  - Atomic executable replacement on POSIX systems via same-directory staging and `rename(2)` with `0755` permissions.
  - Running executable replacement on Windows via `.old` file rotation.
  - Safeguard preventing self-update when running from temporary build directories (e.g. `go run`).
  - Environment detection providing upgrade guidance for package managers (Homebrew, `go install`).
- **GitHub API Rate-Limit Resilience**:
  - Authenticated GitHub API requests when `GITHUB_TOKEN` or `GH_TOKEN` is present in the environment.
  - Automatic fallback to HTML release redirect resolution (`/releases/latest` 302 redirect) to bypass API rate limits completely.
- **Windows Platform Support**:
  - Added support for Windows (`windows/amd64` and `windows/arm64`) release builds and distribution archives (`.zip`).
  - Added support for extracting and updating `agrep.exe` from `.zip` archives.
- **Documentation**:
  - Added update options to built-in `--help`, [`README.md`](README.md), and [`docs/CLI.md`](docs/CLI.md).

## [0.1.0] - 2026-09-29

### Added

- **Core Search Engine**:
  - Ultrafast, Unicode-aware search tool tailored for Arabic-script text.
  - Support for reading from files, directories, or standard input (`-`).
- **Unicode Normalization Profiles**:
  - `search` (default): folds diacritics (*tashkil*), tatweel, canonical forms, and common letter variations (Alef, Hamza, Ta-Marbuta, Alef-Maksura).
  - `strict`: preserves letter-level distinctions while stripping cosmetic marks.
  - `loose`: aggressive folding including consonant skeleton (*rasm*) and digit/punctuation folding.
  - `lucene`: Apache Lucene `ArabicNormalizer` compatibility profile.
  - `camel`: CAMeL Tools normalization compatibility profile.
- **Fine-Grained Orthographic Flags**:
  - Letter and mark controls: `--keep-hamza`, `--keep-tamarbuta`, `--keep-tashkil`, `--keep-presentation-forms`, `--keep-joiners`, `--keep-bidi-marks`, `--keep-quranic-marks`.
  - Normalization transforms: `--fold-digits`, `--fold-punctuation`, and dotless skeleton search (`--rasm`).
- **Multi-Language Support**:
  - Multi-orthography support for Arabic-script languages via `--lang`: Arabic (`ar`), Persian (`fa`), Urdu (`ur`), Pashto (`ps`), Kurdish (`ku`), and Uyghur (`ug`).
- **Search Modes**:
  - High-performance literal matcher with substring shortcuts.
  - Regular expressions evaluated over normalized text (`--regex`).
  - Myers fuzzy Levenshtein edit distance search (`--fuzzy[=N]`).
- **Query Transliteration**:
  - Input query transliteration schemes via `--translit`: Buckwalter, ArabTeX, and ISO 233.
- **Output & Formatting**:
  - Line numbers (`-n`), match count only (`-c`), files with matches (`-l`), files without matches (`-L`).
  - Context lines (`-A`, `-B`, `-C`).
  - Matched spans only (`-o`).
  - Colored matching output with directional RTL isolates (`--color`, `--no-bidi-isolate`).
  - Transliterated output rendering in Buckwalter (`--translit-out`).
  - Structured, machine-readable JSON Lines streaming output (`--json`).
- **File System & Encoding**:
  - Recursive directory search with multi-threaded file workers (`-r`, `--threads`).
  - `.gitignore` awareness with override flag (`--no-ignore`).
  - Path filtering with `--include` and `--exclude` glob patterns.
  - Automatic encoding detection and legacy decoding: UTF-8, CP1256, ISO-8859-6, UTF-16LE, and UTF-16BE (`--encoding`).
  - Optional document extraction for HTML and EPUB (`-tags formats`).
- **Modular Go Packages**:
  - Public packages: `arabic` (normalization), `match` (match engines), `scan` (line scanner and span mapping).
  - Internal packages: `internal/decode`, `internal/inputformat`, `internal/playground`, `internal/translit`, and `internal/walk`.

[0.1.1]: https://github.com/MohamedElashri/agrep/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/MohamedElashri/agrep/releases/tag/v0.1.0
