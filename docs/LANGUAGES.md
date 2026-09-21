# Arabic-script languages

`--lang` selects which Arabic-script orthographies participate in
normalization. It accepts a comma-separated list of `ar`, `fa`, `ur`, `ps`,
`ku`, and `ug`; the default is `ar`.

```sh
agrep --lang=fa "کتاب" persian.txt
agrep --lang=ar,fa "كتاب" mixed-arabic-persian.txt
agrep --lang=ur "اردو" urdu.txt
```

Language selection is independent of `--profile`. The profile controls general
normalization such as tashkil and hamza handling, while `--lang` controls only
language-aware Arabic-script equivalences and joiner semantics. Every built-in
profile defaults to Arabic when it is used through the Go API.

## Folding policy

A single selected language preserves the distinctions in its alphabet. agrep
does not turn Urdu retroflexes into Arabic letters, collapse the Pashto yeh
set, or erase Kurdish and Uyghur vowel distinctions merely to increase recall.
Those transformations would conflate different letters.

| Selection | Language-aware behavior |
| --- | --- |
| `ar` | No additional language fold. This preserves behavior from before Phase 8. |
| `fa` | Preserves Persian ک and ی as distinct from Arabic ك and ي. Normalizes the discouraged precomposed ezafe form `ۀ` (U+06C0) to `ه`. Preserves ZWNJ. |
| `ur` | Preserves ہ, ھ, ے, ں, ٹ, ڈ, and ڑ as distinct Urdu letters. |
| `ps` | Preserves ښ, ږ, ځ, څ, ډ, ړ, ټ, ڼ, and the Pashto yeh forms ی, ۍ, and ئ. |
| `ku` | Preserves Sorani Kurdish ڵ, ڕ, ۆ, and ێ. Preserves ZWNJ. |
| `ug` | Preserves Uyghur ې, ۈ, ۆ, and ۇ. Preserves ZWNJ. |
| `ar` plus another language | Cross-folds ک (U+06A9) to ك (U+0643) and ی (U+06CC) to ي (U+064A), allowing shared-letter searches across Arabic and the selected orthography. |
| `ar,ur` | Also folds Urdu ہ (U+06C1) to Arabic ه. The aspiration letter ھ remains distinct. |

The important Persian distinction is therefore explicit:

```text
--lang=fa       ک and ك remain different; ی and ي remain different
--lang=ar,fa    ک matches ك; ی matches ي
```

Unicode documents U+06CC as FARSI YEH and records its use across several of
these orthographies. CLDR's Persian guidance requires U+06A9 rather than Arabic
U+0643 and U+06CC rather than Arabic U+064A, which is why Persian-only matching
does not silently merge them. CLDR also recommends spelling ezafe as ه followed
by U+0654 instead of the deprecated U+06C0 form. See the
[Unicode Arabic chapter](https://www.unicode.org/versions/Unicode18.0.0/core-spec/chapter-9/),
[Arabic names list](https://www.unicode.org/Public/18.0.0/charts/nameslist/0600/),
and [CLDR Persian guidance](https://cldr.unicode.org/translation/language-specific/persian).

## ZWNJ and word boundaries

ZWNJ (U+200C) is orthographic content in Persian, Sorani Kurdish, and Uyghur,
not merely copied-page formatting. When any of `fa`, `ku`, or `ug` is selected,
agrep preserves ZWNJ even if the profile normally strips joiners. It also treats
ZWNJ as word-internal for `-w`, so a query for `می` does not count as a complete
word at the start of `می‌روم`. ZWJ remains governed by `StripJoiners`.

This follows Unicode's Persian joining-control example in
[UAX #31](https://www.unicode.org/reports/tr31/tr31-29.html) and the Unicode
discussion of ZWNJ in
[Sorani and Uyghur](https://www.unicode.org/L2/L2014/14136-hehs-sorani-uighur.pdf).
`--keep-joiners` still preserves both ZWNJ and ZWJ for callers that need the raw
controls in any language.

## Profiles and transliteration

`--profile=lucene` and `--profile=camel` reproduce their documented Arabic
normalizers only with the default `--lang=ar`. Adding another language is an
explicit agrep extension layered on the selected profile, not a claim that the
upstream normalizer implements those language rules.

The Phase 7 transliteration schemes remain separate from language
normalization. Their documented Arabic characters are transliterated;
language-specific letters without a mapping pass through unchanged. agrep does
not invent Latin spellings for Urdu, Pashto, Kurdish, or Uyghur letters.

Phase 9 rasm follows the same boundary: only its documented Arabic consonant
groups lose i'jam distinctions. The language-specific letters in this document
remain distinct. See [MATCHING.md](MATCHING.md).

Pashto yeh forms can encode positional and grammatical distinctions, as noted
in the [Unicode Afghanistan locale requirements](https://www.unicode.org/L2/L2003/03148-af-locales.pdf).
That is one reason they are tested for distinctness instead of being collapsed.
