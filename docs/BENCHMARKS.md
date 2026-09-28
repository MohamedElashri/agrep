# Benchmarks

These are local measurements, not a claim about every Arabic corpus or machine.
The input is a 16 MiB file made by repeating the original synthetic line in
`testdata/corpus/msa.txt`. `scripts/benchmark.py` builds the current CLI,
warms the filesystem cache, then reports the median of five runs. All three
tools count the same 270,601 lines containing the exact literal `مكتبة`.
Output is discarded during timing.

| Tool | Median seconds | Throughput | Relative to ripgrep |
| --- | ---: | ---: | ---: |
| agrep | 1.691 | 9.5 MiB/s | 68.8x slower |
| GNU grep 3.11 | 0.00271 | 5,898 MiB/s | 9.1x faster |
| ripgrep 15.2.0 | 0.0246 | 651 MiB/s | baseline |

Run on 2026-09-28, Linux 7.0, AMD Ryzen 7 7735HS, Go 1.27.1. The repeated,
short line and `-c` mode make this a narrow throughput comparison. It exposes
a large performance gap worth profiling, especially relative to ripgrep. It
does not establish performance on mixed documents, long lines, recursive file
discovery, or cold storage.

This speed comparison uses a spelling all three programs can find. The script
also checks the feature difference separately: with input `المدينة` and query
`المدينه`, agrep exits 0 while literal `grep -F` and `rg -F` exit 1. In that
case agrep folds ta-marbuta to heh before matching, while the other
tools search the raw spelling.

Reproduce the comparison from the repository root:

```sh
python3 scripts/benchmark.py --size-mib 16 --runs 5 --output /tmp/agrep-benchmark.json
```

The feature microbenchmarks use Go's benchmark runner on a roughly 10 KiB
normalized Arabic line. On the same machine, positional fuzzy `FindAll` took
about 460 µs/op and 11 allocations; its endpoint-only boolean `Matches` path
took about 273 µs/op and zero allocations. On a short fully voweled line,
rasm normalization ran at about 7.9 MB/s directly and 7.7 MB/s with origin
mapping. These figures measure different input sizes and operations from the
CLI comparison above and should not be combined into a single ratio.

```sh
go test ./match ./arabic -run '^$' \
  -bench 'BenchmarkFuzzyMatcher|BenchmarkNormalize(Rasm|MappedRasm)$' \
  -benchtime=500ms -count=1
```
