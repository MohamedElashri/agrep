---
title: "Rasm & Fuzzy Matching"
description: "Dotless rasm consonant skeleton matching and bit-parallel Myers fuzzy Levenshtein distance."
category: "Core Concepts"
order: 3
---

# Rasm and fuzzy matching

`--rasm` broadens normalization by merging selected consonants; `--fuzzy[=N]`
allows edits between normalized words. They can be combined. Both increase
recall and may produce extra matches.

## Dotless rasm

| Letters | Shared key |
| --- | --- |
| ب ت ث ن ي | ٮ |
| ج ح خ | ح |
| د ذ | د |
| ر ز | ر |
| س ش | س |
| ص ض | ص |
| ط ظ | ط |
| ع غ | ع |
| ف ق | ٯ |

```sh
agrep --rasm -C 2 'مستشرق' manuscript.txt
```

`search` leaves rasm off; `loose` enables it. The fold runs after language
normalization. It leaves language-specific letters outside the table distinct.
With `--regex`, write the pattern for the **normalized skeleton**; agrep does
not rewrite regex syntax.

## Fuzzy matching

`--fuzzy` allows one insertion, deletion, or substitution. Set a non-negative
threshold with `--fuzzy=N`; `--fuzzy=0` requires an exact normalized match.
Distance counts Unicode codepoints, not UTF-8 bytes.

```sh
agrep --fuzzy=1 'كتاب' OCR.txt
agrep --rasm --fuzzy=2 -w 'المستشرقون' scans.txt
```

Profiles, language rules, transliteration, and case folding apply before fuzzy
comparison. `--regex` and `--fuzzy` cannot be combined. Longer queries and
context output help review broad matches.

For `-o`, color, and JSON, fuzzy matches map to original text. The match must
cover at least one text codepoint. Inserted text is included; a deleted query
codepoint contributes no source bytes. At one endpoint, the lowest edit
distance wins, then the longest text span. Overlapping candidates prefer lower
distance, earlier position, then longer span. Results are non-overlapping and
sorted in text order.

The detector uses [Myers' bit-vector algorithm](https://doi.org/10.1145/316542.316550)
for both short and long patterns; tests compare its endpoints with a dynamic
programming oracle.
