#!/usr/bin/env python3
"""Compare literal line counting on a repeated, synthetic corpus fixture."""

import argparse
import json
import platform
import shutil
import statistics
import subprocess
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "testdata" / "corpus" / "msa.txt"


def run(command, *, capture=False):
    result = subprocess.run(
        command,
        cwd=ROOT,
        stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"{command!r} exited {result.returncode}: "
            f"{result.stderr.decode(errors='replace')}"
        )
    return result.stdout.decode().strip() if capture else None


def measure(command, runs):
    run(command)  # warm the filesystem cache before timed runs
    samples = []
    for _ in range(runs):
        start = time.perf_counter()
        run(command)
        samples.append(time.perf_counter() - start)
    return samples


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--size-mib", type=int, default=16)
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--agrep", type=Path, help="use an existing agrep binary")
    parser.add_argument("--output", type=Path, help="write machine-readable results")
    args = parser.parse_args()
    if args.size_mib <= 0 or args.runs <= 0:
        parser.error("--size-mib and --runs must be positive")
    for program in ("grep", "rg"):
        if shutil.which(program) is None:
            parser.error(f"{program} is required")

    with tempfile.TemporaryDirectory(prefix="agrep-benchmark-") as temp:
        temp = Path(temp)
        binary = args.agrep.resolve() if args.agrep else temp / "agrep"
        if not args.agrep:
            run(["go", "build", "-trimpath", "-o", str(binary), "./cmd/agrep"])
        source = SOURCE.read_bytes()
        corpus = temp / "msa-repeated.txt"
        with corpus.open("wb") as output:
            for _ in range(max(1, (args.size_mib * 1024 * 1024 + len(source) - 1) // len(source))):
                output.write(source)

        # All tools count the same literal word on the same file. Output is
        # suppressed in timed runs, avoiding terminal throughput as a factor.
        commands = {
            "agrep": [str(binary), "-c", "مكتبة", str(corpus)],
            "grep": ["grep", "-Fc", "مكتبة", str(corpus)],
            "ripgrep": ["rg", "-Fc", "مكتبة", str(corpus)],
        }
        counts = {name: run(command, capture=True) for name, command in commands.items()}
        if len(set(counts.values())) != 1:
            raise RuntimeError(f"selected-line counts differ: {counts}")

        # The feature comparison is separate from the speed comparison:
        # grep and ripgrep cannot match this spelling without preprocessing.
        contrast = {}
        for name, command in {
            "agrep": [str(binary), "المدينه"],
            "grep": ["grep", "-F", "المدينه"],
            "ripgrep": ["rg", "-F", "المدينه"],
        }.items():
            contrast[name] = subprocess.run(
                command, input="المدينة\n".encode(), stdout=subprocess.DEVNULL,
                stderr=subprocess.PIPE, check=False,
            ).returncode
        if contrast != {"agrep": 0, "grep": 1, "ripgrep": 1}:
            raise RuntimeError(f"normalization contrast changed: {contrast}")

        timings = {name: measure(command, args.runs) for name, command in commands.items()}
        size = corpus.stat().st_size
        results = {
            "date_utc": datetime.now(timezone.utc).isoformat(),
            "platform": platform.platform(),
            "go_version": run(["go", "version"], capture=True),
            "grep_version": run(["grep", "--version"], capture=True).splitlines()[0],
            "ripgrep_version": run(["rg", "--version"], capture=True).splitlines()[0],
            "fixture": str(SOURCE.relative_to(ROOT)),
            "synthetic": True,
            "bytes": size,
            "selected_lines": int(next(iter(counts.values()))),
            "normalization_contrast_exit_codes": contrast,
            "runs": args.runs,
            "seconds": timings,
            "median_seconds": {name: statistics.median(values) for name, values in timings.items()},
            "mib_per_second": {
                name: size / (1024 * 1024) / statistics.median(values)
                for name, values in timings.items()
            },
        }
        report = json.dumps(results, ensure_ascii=False, indent=2) + "\n"
        if args.output:
            args.output.write_text(report, encoding="utf-8")
        print(report, end="")


if __name__ == "__main__":
    main()
