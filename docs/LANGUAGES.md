---
title: "Arabic-Script Languages"
description: "Language-specific orthographies, alphabet rules, and cross-language shared folding for Persian, Urdu, Pashto, Kurdish, and Uyghur."
category: "Core Concepts"
order: 2
---

# Arabic-script languages

`--lang` takes a comma-separated list of `ar`, `fa`, `ur`, `ps`, `ku`, and
`ug`. The default is `ar`. A single language keeps its distinct alphabet;
selecting Arabic with another language enables only the shared-letter folds
listed below. Language selection works with every profile.

```sh
agrep --lang=fa 'کتاب' persian.txt
agrep --lang=ar,fa 'كتاب' mixed.txt
```

| Selection | Additional behavior |
| --- | --- |
| `ar` | No extra language fold. |
| `fa` | Preserves Persian ک and ی; folds precomposed ezafe `ۀ` to `ه`; preserves ZWNJ. |
| `ur` | Preserves Urdu ہ, ھ, ے, ں, ٹ, ڈ, ڑ. |
| `ps` | Preserves Pashto ښ, ږ, ځ, څ, ډ, ړ, ټ, ڼ and its yeh forms. |
| `ku` | Preserves Sorani Kurdish ڵ, ڕ, ۆ, ێ and ZWNJ. |
| `ug` | Preserves Uyghur ې, ۈ, ۆ, ۇ and ZWNJ. |
| `ar` with another language | Folds ک → ك and ی → ي for shared-letter search. |
| `ar,ur` | Also folds Urdu ہ → Arabic ه; keeps aspiration letter ھ distinct. |

Under `fa`, `ک` and `ك` remain different; under `ar,fa`, they match. The same
applies to `ی` and `ي`. ZWNJ (U+200C) stays in Persian, Sorani Kurdish, and
Uyghur and counts as word-internal for `-w`; ZWJ follows the selected profile.
`--keep-joiners` preserves both.

`lucene` and `camel` represent their upstream Arabic normalizers only under
`--lang=ar`. Rasm likewise folds only its documented Arabic consonants; see
[MATCHING.md](MATCHING.md). Transliteration passes through language-specific
letters without a mapping.

Sources: [Unicode Arabic chapter](https://www.unicode.org/versions/Unicode18.0.0/core-spec/chapter-9/),
[CLDR Persian guidance](https://cldr.unicode.org/translation/language-specific/persian),
[Unicode joining controls](https://www.unicode.org/reports/tr31/tr31-29.html),
[Afghanistan locale requirements](https://www.unicode.org/L2/L2003/03148-af-locales.pdf).
