# Performance benchmarks

These are full command-line runs on synthetic, checked-in corpus text expanded
to approximately 16 MiB per fixture. Times include startup, decoding,
matching, and output. **Lower time is better.** Each time is the median of five
warm-cache runs; the parentheses show the observed minimum and maximum.
Throughput divides input MiB by median elapsed time.

## Same-result searches

GNU grep and ripgrep use fixed-string search (`-F`). These rows compare tools
only where they select the same lines or files. They do not promise equivalent
matching on other Arabic text.

| Workload | Selected | agrep ms (range) | grep ms | rg ms | agrep / rg | agrep MiB/s |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| Exact count, repeated | 270,601 lines | 50.9 (49.1–54.5) | 2.9 | 25.9 | 1.96× | 314.4 |
| Literal miss, repeated | 0 lines | 68.6 (62.6–69.5) | 10.8 | 7.3 | 9.38× | 233.3 |
| Numbered line output | 270,601 lines | 416.5 (393.6–423.8) | 3.1 | 49.2 | 8.47× | 38.4 |
| 64 KiB logical lines | 257 lines | 55.8 (54.5–66.4) | 3.2 | 6.9 | 8.10× | 287.8 |
| Exact count, varied unique lines | 3,122 lines | 121.2 (112.8–126.2) | 3.2 | 7.2 | 16.82× | 132.0 |
| Literal miss, varied unique lines | 0 lines | 152.3 (133.5–160.4) | 10.5 | 7.1 | 21.51× | 105.1 |
| Recursive matching files | 32 files | 13.1 (12.4–15.6) | 3.9 | 7.5 | 1.74× | 1,222.2 |

`-c` counts selected lines, not occurrences. The numbered-output commands
write to a null sink and use each tool's native formatting. Recursive results
are checked as a file set. The varied fixture cycles short, long, voweled,
presentation-form, and Persian lines with a unique suffix on every line.

## Arabic-aware and other agrep work

Raw grep and ripgrep find zero lines in the first three searches below, so
their timings would measure a different answer. JSON spans use decoded UTF-8
byte offsets in original text.

| Workload | Selected | agrep ms (range) | MiB/s |
| :--- | ---: | ---: | ---: |
| Alef-hamza fold, `اعلنت` | 270,601 lines | 1,149.5 (1,109.4–1,187.6) | 13.9 |
| Remove vowels, `قال` | 390,168 lines | 811.4 (796.5–830.1) | 19.7 |
| Normalized count, varied lines | 6,244 lines | 966.8 (883.9–1,068.2) | 16.6 |
| Mapped JSON spans, repeated | 270,601 records | 1,451.8 (1,431.9–2,194.6) | 11.0 |
| Mapped JSON spans, varied | 6,244 records | 1,189.6 (1,147.9–1,377.3) | 13.5 |
| One-edit fuzzy count | 1,677,722 lines | 929.9 (825.5–1,061.9) | 17.2 |
| CP1256 decode and count | 493,448 lines | 142.6 (132.5–146.3) | 112.2 |

The repeated mapped JSON case emits about 52 MiB of JSON from 16 MiB of
input. Output volume and span mapping are part of its measured cost.

## Allocations inside the scanner

These package benchmarks scan approximately 10 MiB per operation without CLI
startup or JSON serialization. They help locate allocation costs; they are
not directly comparable to the full CLI times above.

| Scan path | Bytes/op | Allocs/op |
| :--- | ---: | ---: |
| Dense mapped matches | 29,571,177 | 595,802 |
| Mapped miss | 4,128 | 2 |
| Sparse mapped matches | 19,592 | 604 |
| Plain count-style scan | 4,128 | 2 |
| Plain miss | 4,128 | 2 |

## Setup and reproduction

| Item | Value |
| :--- | :--- |
| agrep source revision | `ed2bfb1` |
| agrep binary SHA-256 | `4f479e325f359311aa13b3b91029115d36aa2a70e0cbab39999643e31bdbf02d` |
| Host | AMD Ryzen 7 7735H, Linux x86-64 |
| Toolchain | Go 1.27.1, GNU grep 3.11, ripgrep 15.2.0 |
| Locale | `C.UTF-8` |
| Method | One warm-up per command; five timed runs with rotated command order; output discarded during timing |

The [benchmark script](../scripts/benchmark.py) records individual timings,
commands, tool versions, selected counts, fixture hashes, and binary hash in
JSON. It regenerates fixtures from the licensed [corpus](../testdata/corpus/SOURCES.md).

```sh
go build -trimpath -buildvcs=false -o ./agrep ./cmd/agrep
python3 scripts/benchmark.py --agrep ./agrep --revision "$(git rev-parse --short HEAD)" \
  --size-mib 16 --runs 5 --output results.json
```

To compare revisions, build both binaries and run the same command for each
on one host. Keep the JSON reports and compare ranges as well as medians.
Repeated synthetic text and warm caches do not represent every document,
storage device, or query mix. Small grep and ripgrep times are especially
sensitive to host load and process startup.
