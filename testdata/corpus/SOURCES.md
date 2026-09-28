# Conformance corpus provenance

All lines in this directory were composed for agrep's tests in September 2026.
They are synthetic examples, not excerpts from the Quran, published poetry,
news, social media posts, PDFs, or OCR documents. The category names describe
the input feature being modeled. The source lines and expected results are
part of the project's MIT-licensed test suite.

| File | Modeled input and purpose |
| --- | --- |
| `quran.txt` | Fully voweled Arabic with wasla and a Quranic pause mark; the sentence is original test text. |
| `poetry.txt` | Poetic-style vowel marks and tatweel; original test text. |
| `msa.txt` | Modern Standard Arabic prose; original test text. |
| `pdf.txt` | Presentation-form glyphs used by some extracted text layers; typed for this suite. |
| `ocr.txt` | An inserted letter and a dot-sensitive variant for fuzzy and rasm search; typed for this suite. |
| `social.txt` | Mixed Latin, emoji, Arabic, and Eastern Arabic digits; original test text. |
| `languages.txt` | Persian/Arabic mixed orthography, Urdu and Uyghur distinctions, and Persian ZWNJ; original test text. |
| `distinct.txt` | Minimal Urdu, Pashto, Sorani, and Uyghur letters that must remain distinct; constructed test tokens. |
| `spans.txt` | Tatweel/mark, full case-fold expansion, and zero-length regex boundary cases; constructed test tokens. |
| `varied.txt` | Original mixed Arabic and Persian lines with sparse and repeated hits, vowel marks, presentation forms, digits, an empty line, and a line longer than 4 KiB. The long line repeats `مدخل قصير ` 260 times. |
| `cp1256.bin`, `iso88596.bin`, `utf16le.bin`, `utf16be.bin` | Encoded forms of original short Arabic test lines. |
| `varied-cp1256.bin`, `varied-utf16le.bin` | CP1256 form of four short lines from `varied.txt`; BOM-marked UTF-16LE form of the complete varied file. |

`cases.json` is the hand-reviewed oracle: each case records the command options,
selected line numbers, emitted text, and expected half-open UTF-8 byte ranges.
Legacy-encoded input uses decoded UTF-8 coordinates. The corpus is intentionally
small and synthetic; it checks conformance, not linguistic accuracy or speed on
real-world distributions.
