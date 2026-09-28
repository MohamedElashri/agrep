# Benchmarks

The [benchmark script](../scripts/benchmark.py) builds `agrep` and creates
temporary fixtures from the checked-in [MSA](../testdata/corpus/msa.txt),
[voweled](../testdata/corpus/quran.txt), and [OCR](../testdata/corpus/ocr.txt)
samples. It checks selected-line counts or output records before timing.
Generated fixtures and binaries are removed afterward.

## Workloads

| Workload | Measured operation | Comparison |
| --- | --- | --- |
| Exact count | `-c` on lines containing `مكتبة` | Same selected lines for all three tools. |
| Literal miss | `-c` with no matching lines | Same result; measures the full scan. |
| Line output | Numbered matching lines sent to a null sink | Same selected lines, different output formats. |
| Long lines | `-c` on 64 KiB logical lines | Same selected lines. |
| Recursive files | `-r -l` on a 32-file tree | Same matching file set. |
| Hamza fold | `-c اعلنت` on `أعلنت` | agrep selects lines; raw grep/ripgrep select none. |
| Voweled text | `-c قال` on `قَالَ` | agrep selects lines; raw grep/ripgrep select none. |
| JSON spans | `--json مكتبه` with mapped original-text spans | agrep-only output format. |
| Fuzzy | `--fuzzy=1 -c كتاب` on OCR lines | agrep-only edit-distance mode. |
| Legacy decode | `--encoding=cp1256 -c مكتبه` | agrep-only decode and normalize pipeline. |

`grep` and `ripgrep` timings appear only where the tools select the same lines
or files. Their raw counts for normalization workloads are recorded in JSON as
a correctness contrast, not a speed comparison.

## Method

```sh
python3 scripts/benchmark.py --size-mib 16 --runs 5 --output results.json
```

`--agrep /path/to/binary` uses an existing build. The script writes fixtures
near the requested size, warms each command once, rotates command order, and
reports individual runs, median, minimum, maximum, and MiB/s. It records
versions, CPU, fixture source hashes, the agrep binary hash, input size, and
commands. Timing includes process startup and output generation; output goes
to a null sink. Fixtures are synthetic repetitions with a warm filesystem
cache. These numbers do not predict cold I/O, diverse documents, or every
query mix. Small grep timings are especially sensitive to startup noise.

For a change comparison, run the **same script, fixture size, and run count**
for the base and candidate binaries on the same machine. Keep both JSON files.

## Example local run

Linux 7.0, AMD Ryzen 7 7735HS, Go 1.27.1, GNU grep 3.11, ripgrep 15.2.0;
8 MiB per fixture, three timed runs, 2026-09-28. Times below are medians in
seconds. Short runs are illustrative, not a regression threshold.

| Same-result workload | agrep | grep | ripgrep |
| --- | ---: | ---: | ---: |
| Exact count | 0.0365 | 0.0026 | 0.0144 |
| Literal miss | 1.031 | 0.0076 | 0.0060 |
| Numbered line output | 0.243 | 0.0028 | 0.0291 |
| 64 KiB lines | 0.0351 | 0.0026 | 0.0054 |
| Recursive matching files | 0.0163 | 0.0036 | 0.0075 |

| agrep-specific workload | Median seconds | Selected lines |
| --- | ---: | ---: |
| Alef-hamza fold | 0.995 | 135,301 |
| Voweled text | 0.531 | 195,084 |
| Mapped JSON spans | 1.480 | 135,301 |
| One-edit fuzzy count | 0.553 | 838,862 |
| CP1256 decode and count | 0.113 | 246,724 |

The exact-hit shortcut helps when a raw occurrence proves a normalized literal
match. Misses, voweled input, and mapped output still require substantially
more work. The benchmark suite keeps those costs visible rather than collapsing
them into one throughput claim.
