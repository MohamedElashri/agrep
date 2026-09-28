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
| Exact count, repeated | 270,601 lines | 54.1 (53.4–60.6) | 3.5 | 26.0 | 2.08× | 295.9 |
| Literal miss, repeated | 0 lines | 63.6 (61.6–64.5) | 10.1 | 6.5 | 9.78× | 251.8 |
| Numbered line output | 270,601 lines | 427.7 (417.9–459.3) | 3.0 | 47.6 | 8.98× | 37.4 |
| 64 KiB logical lines | 257 lines | 36.5 (36.1–39.0) | 2.4 | 5.0 | 7.36× | 439.9 |
| Exact count, varied unique lines | 3,122 lines | 67.6 (65.8–70.5) | 2.5 | 5.6 | 12.00× | 236.8 |
| Literal miss, varied unique lines | 0 lines | 83.2 (81.5–86.7) | 8.1 | 5.7 | 14.59× | 192.4 |
| Recursive matching files | 32 files | 13.2 (12.7–13.8) | 3.8 | 7.5 | 1.76× | 1,214.4 |

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
| Alef-hamza fold, `اعلنت` | 270,601 lines | 1,172.3 (1,114.8–1,242.9) | 13.6 |
| Remove vowels, `قال` | 390,168 lines | 841.4 (833.7–1,067.6) | 19.0 |
| Normalized count, varied lines | 6,244 lines | 938.8 (917.2–994.5) | 17.0 |
| Mapped JSON spans, repeated | 270,601 records | 1,711.0 (1,696.6–1,755.9) | 9.4 |
| Mapped JSON spans, varied | 6,244 records | 1,167.6 (1,090.0–1,472.2) | 13.7 |
| One-edit fuzzy count | 1,677,722 lines | 688.6 (633.3–733.9) | 23.2 |
| CP1256 decode and count | 493,448 lines | 111.8 (108.5–119.1) | 143.1 |

The repeated mapped JSON case emits about 52 MiB of JSON from 16 MiB of
input. Output volume and span mapping are part of its measured cost.

## Allocations inside the scanner

These package benchmarks scan approximately 10 MiB per operation without CLI
startup or JSON serialization. They help locate allocation costs; they are
not directly comparable to the full CLI times above.

| Scan path | Bytes/op | Allocs/op |
| :--- | ---: | ---: |
| Dense mapped matches | 29,573,044 | 595,805 |
| Mapped miss | 4,128 | 2 |
| Sparse mapped matches | 19,592 | 604 |
| Plain count-style scan | 4,128 | 2 |
| Plain miss | 4,128 | 2 |

## Setup and reproduction

| Item | Value |
| :--- | :--- |
| agrep source revision | `f0bb812` |
| agrep binary SHA-256 | `4f479e325f359311aa13b3b91029115d36aa2a70e0cbab39999643e31bdbf02d` |
| Host | AMD Ryzen 7 7735H, Linux x86-64 |
| Toolchain | Go 1.27.1, GNU grep 3.11, ripgrep 15.2.0 |
| Locale | `C.UTF-8` |
| Method | One warm-up per command; five timed runs with rotated command order; output discarded during timing |

The [benchmark script](../scripts/benchmark.py) records individual timings,
commands, tool versions, selected counts, fixture hashes, and binary hash in
JSON. It regenerates fixtures from the licensed [corpus](../testdata/corpus/SOURCES.md).
The binary hash matches the preceding report: this branch revision changes the
benchmark inputs and documentation, not the compiled CLI. Differences between
the two reports reflect separate timing runs on the host.

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
