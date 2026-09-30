---
name: agrep-search
description: Search local Arabic-script text with agrep when spelling, diacritics, script variants, transliteration, or original-text match spans matter. Use for corpus discovery and evidence extraction; ordinary source-code searches do not need this skill.
---

# agrep: Arabic-Script Search & Evidence Extraction

`agrep` is a specialized search utility for Arabic-script text (Arabic, Persian, Urdu, Kurdish, Pashto, Uyghur). Standard tools (`grep`, `ripgrep`) fail on Arabic documents because diacritics (*tashkil*), spelling variations (alef, hamza, ta-marbuta, alef-maksura), ligatures, and encoding differences create mismatches. `agrep` normalizes queries and text on the fly while preserving original text lines, line numbers, and exact UTF-8 byte offsets.

---

## 1. Acquiring and Verifying the Binary

Ensure you are using the modern Arabic `agrep` binary (`v0.1.0+`), not the unrelated 1990s approximate grep utility (`agrep` by Wu & Manber).

### Verification
```sh
agrep --version
```
Expected output format: `agrep 0.1.0` (or `agrep dev`). If the command fails, is missing, or outputs legacy usage, acquire the official release binary.

### Installation via Pre-compiled Release Binaries

**Option A: Automated Install Script (macOS, Linux, BSD)**
```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | bash
```

**Option B: Direct Release Download (No Root Required)**
```sh
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] && ARCH="amd64"
[ "$ARCH" = "aarch64" ] && ARCH="arm64"
VERSION="0.1.0"

mkdir -p ~/.local/bin
curl -fsSL "https://github.com/MohamedElashri/agrep/releases/download/v${VERSION}/agrep_${VERSION}_${OS}_${ARCH}.tar.gz" | tar -xz -C ~/.local/bin agrep
export PATH="$HOME/.local/bin:$PATH"
```

**Option C: Go Toolchain or Local Build**
```sh
# If Go is installed:
go install github.com/MohamedElashri/agrep/cmd/agrep@v0.1.0

# Or inside the repository:
go build -o ./agrep ./cmd/agrep
```

---

## 2. Core Agent Recipes

Follow progressive filtering: check file presence and counts before requesting full text or JSON.

### A. Corpus Discovery (Find matching files)
Find which files mention a term without dumping entire contents:
```sh
agrep -r -l --include '*.txt' 'خوارزمي' corpus/
```

### B. Count Matches Across Files
```sh
agrep -r -c 'ابن خلدون' corpus/
```

### C. Human Inspection with Line Numbers & Context
Inspect matching lines with 2 lines of surrounding context:
```sh
agrep -n -C 2 'الجمهورية' documents/doc.txt
```

### D. Exact Machine-Readable Evidence (JSON Lines)
Produce line-by-line JSON with original text and match byte spans:
```sh
agrep --json 'أحمد' people.txt
```

### E. Transliterated Query from Latin Keyboards
Search Arabic texts using standard Latin transliteration (e.g. Buckwalter, ArabTeX, ISO 233):
```sh
agrep --translit=buckwalter 'ktAb' corpus.txt  # Matches كِتَاب, كتاب, etc.
```

### F. Fuzzy Search for OCR & Typographical Errors
Allow Levenshtein edit distance over normalized codepoints (default `N=1`):
```sh
agrep --fuzzy=1 'قسطنطينية' historical_corpus/
```

---

## 3. Search Profiles & Normalization Reference

| Profile | Command Flag | Rules & Behavior | Recommended Use Case |
| :--- | :--- | :--- | :--- |
| **Search** (Default) | `--profile=search` | Strips tashkil and tatweel; folds alef variants (`أ إ آ ٱ` → `ا`), ta-marbuta (`ة` → `ه`), and alef-maksura (`ى` → `ي`); expands presentation forms. | General search, broad information retrieval, search engines. |
| **Strict** | `--profile=strict` | Strips only tatweel and decorative diacritics; keeps `أ`/`إ`/`ا`, `ة`/`ه`, and `ى`/`ي` distinct. | Quranic and Classical texts, legal/official documents, exact names. |
| **Loose** | `--profile=loose` | All search folds plus consonant skeleton (*rasm*), digits, and Arabic punctuation. | High-recall searches, heavily damaged documents. |
| **Rasm** | `--rasm` | Folds consonants to dotless skeletons (`ب ت ث ن ي` → `ٮ`, `ج ح خ` → `ح`, `د ذ` → `د`, `ر ز` → `ر`, `س ش` → `س`, `ص ض` → `ص`, `ط ظ` → `ط`, `ع غ` → `ع`, `ف ق` → `ڡ`). | Early Islamic manuscripts, unpointed calligraphy, OCR errors. |
| **Lucene** | `--profile=lucene` | Emulates Apache Lucene's `ArabicNormalizer`. | Compatibility with Lucene/Elasticsearch index pipelines. |
| **CAMeL** | `--profile=camel` | Emulates CAMeL Tools' `normalize_alef_ar` and `dediac_ar`. | Compatibility with CAMeL NLP pipelines. |

### Fine-Tuning Flags (Apply over any profile)
- `--keep-hamza`: Do not fold hamza variants (`أ`, `إ`, `آ`, `ٱ`, `ؤ`, `ئ`).
- `--keep-tamarbuta`: Do not fold `ة` to `ه`.
- `--keep-tashkil`: Preserve all diacritics (*harakat*).
- `--fold-digits`: Convert Arabic-Indic digits (`٠-٩`) and Persian digits (`۰-۹`) to ASCII (`0-9`).
- `--fold-punctuation`: Convert Arabic comma (`،`), semicolon (`؛`), and question mark (`؟`) to ASCII.

---

## 4. Languages and Encodings

### Languages (`--lang`)
Selects language-specific letter mappings:
- `ar`: Standard Arabic (default).
- `fa`: Persian (folds `ک`/`گ`, `ی`/`ي`, preserves ZWNJ when configured).
- `ur`: Urdu (folds `ٹ`, `ڈ`, `ڑ`, `ہ`, `ۂ`, Urdu digits).
- `ku`: Kurdish (Central Kurdish / Sorani normalization).
- `ps`: Pashto.
- `ug`: Uyghur.
- Combinations: `--lang=ar,fa` applies both sets of rules.

### Legacy Encodings (`--encoding`)
If scanning older archives, legacy databases, or Windows files:
- `--encoding=auto`: Auto-detects UTF-8, Windows CP1256, ISO-8859-6, UTF-16LE, and UTF-16BE.
- Explicit: `--encoding=cp1256`, `--encoding=iso-8859-6`, `--encoding=utf16le`, `--encoding=utf16be`.

---

## 5. Machine-Readable Evidence Contract (JSON Lines)

When invoked with `--json` (or `-j`), `agrep` streams one JSON object per matched or context line:

```json
{"line":14,"text":"وَقَالَ الْفَارَابِيُّ فِي كِتَابِهِ","spans":[[7,25]]}
```
If multiple files are searched, a `"file"` key is included:
```json
{"file":"corpus/philosophy.txt","line":14,"text":"وَقَالَ الْفَارَابِيُّ فِي كِتَابِهِ","spans":[[7,25]]}
```
For context lines (`-A`, `-B`, `-C`), `"context": true` is present and `"spans"` is omitted.

### Important: Decoding Byte Spans
`spans` are half-open **UTF-8 byte offsets** `[start, end)` into the decoded original `text` string, **not character or rune indices**.

**In Python:**
```python
import json

line_data = json.loads(raw_line)
text_bytes = line_data["text"].encode("utf-8")
for start, end in line_data.get("spans", []):
    matched_term = text_bytes[start:end].decode("utf-8")
    print(f"Matched: {matched_term}")
```

**In Go:**
```go
matched := text[start:end] // Go string indexing is byte-based
```

---

## 6. Complete CLI Flags Reference

| Flag | Purpose | Notes |
| :--- | :--- | :--- |
| `-e PATTERN` | Query pattern | Repeatable; patterns are ORed together. |
| `-i`, `--ignore-case` | Case folding | Applied to Latin and dual-cased scripts. |
| `-v`, `--invert-match` | Invert match | Selects non-matching lines. |
| `-w`, `--word-regexp` | Word boundaries | Requires word boundaries in original text. |
| `-r`, `--recursive` | Directory recursion | Honors `.gitignore` unless `--no-ignore` is passed. |
| `-n`, `--line-number` | Line numbers | 1-based line numbers. |
| `-c`, `--count` | Count | Emits number of matched lines per file. |
| `-l`, `--files-with-matches` | File list | Emits paths of files with at least one match. |
| `-L`, `--files-without-match` | Non-matching files | Emits paths of files with zero matches. |
| `-j`, `--json` | JSON Lines output | Emits structured JSON with UTF-8 byte spans. |
| `-o`, `--only-matching` | Extract spans | Emits only matched text spans. |
| `-A N`, `-B N`, `-C N` | Context lines | After, Before, or surrounding context. |
| `--include GLOB` | Path filter | E.g. `--include '*.txt'` (repeatable). |
| `--exclude GLOB` | Path exclusion | Skip matching paths or directories (repeatable). |
| `--no-ignore` | Bypass ignore | Search files ignored by `.gitignore`. |
| `--regex` | Regex pattern | Evaluates regex over normalized text. |
| `--fuzzy[=N]` | Edit distance | Levenshtein edits over normalized codepoints. |
| `--translit NAME` | Latin query conversion | `buckwalter`, `arabtex`, or `iso233`. |
| `--encoding NAME` | Text decoding | `utf8`, `cp1256`, `iso-8859-6`, `auto`. |
| `--threads N` | Concurrency | Parallel file search workers (default: CPU cores). |

---

## 7. Agent Operational Rules & Guardrails

1. **Check Exit Codes**:
   - `0`: Selection found (matches present).
   - `1`: No match found (text did not match query).
   - `2`: Error (syntax error, unreadable path, invalid regex).
2. **Prevent Context Overflow**:
   - Never run unbounded `--json` on large directory trees without scoping via `--include` or checking matches first with `-l` or `-c`.
3. **Partial Output on Error**:
   - If exit code is `2`, any JSON lines emitted before the error are partial and must not be treated as comprehensive corpus evidence.
4. **Terminal RTL Safety**:
   - By default, human output isolates Arabic spans using Unicode First Strong Isolate (`\u2068`) and Pop Directional Isolate (`\u2069`) to prevent terminal display corruption. When piping or in scripts, pass `--no-bidi-isolate` if pure raw text is required.
