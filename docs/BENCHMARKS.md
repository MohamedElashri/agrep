# Performance benchmarks

> **Reading the results:** Lower time is better. Each time is the median of five
> full CLI runs, in milliseconds. The `agrep / grep` and `agrep / rg` columns
> divide agrep's time by the other tool's time; `3.0×` means agrep took three
> times as long on that workload.

These measurements show both the cost of agrep's default search behavior and
what its Arabic-script features add. Comparisons with GNU grep and ripgrep are
shown only when the tools return the same matching lines or files on the
fixture. They do not imply identical search semantics on arbitrary input.

## Same-result workloads

| Workload | Matching result | agrep | GNU grep | ripgrep | agrep / grep | agrep / rg |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| Exact literal count | 270,601 lines | **76.4** | 2.5 | 25.2 | 30.1× | 3.0× |
| Literal miss | 0 lines | **1,940.8** | 10.7 | 7.3 | 180.8× | 265.3× |
| Numbered line output | 270,601 lines | **534.7** | 3.4 | 61.7 | 158.1× | 8.7× |
| Long lines (about 64 KiB each) | 257 lines | **57.3** | 3.1 | 6.7 | 18.4× | 8.6× |
| Recursive matching files | 32 files | **23.1** | 3.4 | 7.3 | 6.8× | 3.2× |

`-c` counts selected lines, not occurrences. Numbered line output goes to a null
sink; the tools select the same lines but format their output differently.
Recursive mode checks the matching file set. The literal miss is a full scan
with no matching line, and is the largest relative gap in this run.
The shared hit query is `مكتبة`; the miss query is `غيرموجود`. GNU grep and
ripgrep use fixed-string (`-F`) searches.

## When normalization changes the answer

Here, raw literal grep and ripgrep miss matches that agrep finds. Their times
would describe a different result, so the table shows **match counts**, not a
cross-tool speed ratio.

| Input and query | agrep matches | Raw grep | Raw rg | agrep time |
| :--- | ---: | ---: | ---: | ---: |
| `أعلنت` searched with `اعلنت` (alef-hamza fold) | 270,601 | 0 | 0 | 1,705.4 ms |
| `قَالَ` searched with `قال` (vowel removal) | 390,168 | 0 | 0 | 901.3 ms |

## Additional agrep workloads

| Operation | What it measures | Result | Median | Throughput |
| :--- | :--- | ---: | ---: | ---: |
| Mapped JSON spans | Normalize, map spans to original text, emit JSON | 270,601 records | 2,403.8 ms | 6.7 MiB/s |
| One-edit fuzzy count | Search OCR text with `--fuzzy=1` | 1,677,722 lines | 1,000.3 ms | 16.0 MiB/s |
| CP1256 decode and count | Decode legacy text, normalize, count | 493,448 lines | 167.9 ms | 95.3 MiB/s |

Throughput is input MiB divided by median elapsed time. It includes process
startup and output generation, and is not a pure matcher throughput figure.

## Measurement setup

| Item | Value |
| :--- | :--- |
| Source revision | `83b5e2c` |
| Host | AMD Ryzen 7 7735H, Linux x86-64 |
| Toolchain | Go 1.27.1, GNU grep 3.11, ripgrep 15.2.0 |
| Locale | `LC_ALL=C.UTF-8` |
| Input | About 16 MiB per fixture; recursive case uses 32 files totaling about 16 MiB |
| Timing | One warm-up per command, five timed runs, rotated command order, median wall time |

The [benchmark script](../scripts/benchmark.py) repeats the checked-in
[MSA](../testdata/corpus/msa.txt), [voweled](../testdata/corpus/quran.txt), and
[OCR](../testdata/corpus/ocr.txt) samples into temporary fixtures. It verifies
matching-line counts, output record counts, or matching file sets before
measuring. Output is discarded during timing. The script records individual
runs, commands, versions, input sizes, and hashes in JSON, then removes its
temporary files.

These are warm-cache, synthetic fixtures. Repeated text, machine load, storage,
and query mix can change the numbers. In particular, small grep runs are
sensitive to process startup noise. Treat the table as a measured profile of
these workloads, not a general speed ranking.

## Reproduce or compare a change

```sh
python3 scripts/benchmark.py --size-mib 16 --runs 5 --output results.json
```

Pass `--agrep /path/to/binary` to measure an existing build. To evaluate a
code change, run the same fixture size and run count for both binaries on the
same machine, keep both JSON reports, and compare the per-workload medians.
