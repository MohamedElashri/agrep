#!/usr/bin/env python3
"""Compare two agrep binaries on identical retained fixtures and outputs."""

import argparse
import hashlib
import json
import os
import platform
import statistics
import subprocess
import time
from datetime import datetime, timezone
from pathlib import Path

from benchmark import CORPUS, ROOT, Fixture, cpu_model, fixture, sha256


def invoke(command, *, capture=False):
    env = os.environ.copy()
    env["LC_ALL"] = "C.UTF-8"
    return subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        check=False,
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", type=Path, required=True)
    parser.add_argument("--after", type=Path, required=True)
    parser.add_argument("--before-revision", required=True)
    parser.add_argument("--after-revision", required=True)
    parser.add_argument("--patch", type=Path, help="patch defining the after revision")
    parser.add_argument("--fixture-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--size-mib", type=int, default=8)
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--phase2-cases", action="store_true",
                        help="also measure varied misses and long-line hits")
    parser.add_argument("--check-corpus", action="store_true",
                        help="compare full CLI output on every corpus case")
    args = parser.parse_args()
    if args.size_mib <= 0 or args.runs <= 0:
        parser.error("--size-mib and --runs must be positive")

    binaries = {"before": args.before.resolve(), "after": args.after.resolve()}
    for binary in binaries.values():
        if not binary.is_file():
            parser.error(f"binary does not exist: {binary}")

    corpus_check = None
    if args.check_corpus:
        corpus_path = CORPUS / "cases.json"
        corpus_cases = json.loads(corpus_path.read_text(encoding="utf-8"))
        output_digest = hashlib.sha256()
        comparisons = 0
        modes = (("--json",), ("-o",), ("-n",))
        for case in corpus_cases:
            for mode in modes:
                command_tail = [*mode, *case["args"], str(CORPUS / case["file"])]
                checked = {label: invoke([str(binary), *command_tail], capture=True)
                           for label, binary in binaries.items()}
                before, after = checked["before"], checked["after"]
                if (before.returncode, before.stdout, before.stderr) != (
                        after.returncode, after.stdout, after.stderr):
                    raise RuntimeError(f"corpus output differs: {case['name']} {mode}")
                output_digest.update(case["name"].encode())
                output_digest.update(" ".join(mode).encode())
                output_digest.update(bytes([before.returncode]))
                output_digest.update(before.stdout)
                output_digest.update(before.stderr)
                comparisons += 1
        corpus_check = {"source": str(corpus_path.relative_to(ROOT)),
                        "source_sha256": sha256(corpus_path),
                        "cases": len(corpus_cases),
                        "modes": [list(mode) for mode in modes],
                        "comparisons": comparisons,
                        "combined_output_sha256": output_digest.hexdigest()}

    fixture_dir = args.fixture_dir.resolve()
    fixture_dir.mkdir(parents=True, exist_ok=True)
    size = args.size_mib << 20
    msa = fixture(fixture_dir / "msa.txt", "repeated MSA",
                  (CORPUS / "msa.txt").read_bytes(), size)
    quran = fixture(fixture_dir / "quran.txt", "repeated voweled Arabic",
                    (CORPUS / "quran.txt").read_bytes(), size)

    cases = [
        ("exact_count", msa, ["-c", "مكتبة"], 0, msa.lines),
        ("literal_miss", msa, ["-c", "غيرموجود"], 1, None),
        ("hamza_fold", msa, ["-c", "اعلنت"], 0, msa.lines),
        ("voweled", quran, ["-c", "قال"], 0, quran.lines),
        ("json_spans", msa, ["--json", "مكتبه"], 0, msa.lines),
    ]
    extra_fixtures = []
    if args.phase2_cases:
        varied_sources = ("msa.txt", "quran.txt", "ocr.txt", "pdf.txt",
                          "social.txt", "languages.txt", "poetry.txt", "distinct.txt")
        line_pool = [line for name in varied_sources
                     for line in (CORPUS / name).read_bytes().splitlines() if line]
        varied_path = fixture_dir / "varied.txt"
        varied_size = 0
        varied_lines = 0
        with varied_path.open("wb") as output:
            while varied_size < size:
                line = line_pool[varied_lines % len(line_pool)] + b" #" + str(varied_lines).encode() + b"\n"
                output.write(line)
                varied_size += len(line)
                varied_lines += 1
        varied = Fixture("varied unique corpus lines", varied_path, varied_size, varied_lines)
        long_source = (CORPUS / "msa.txt").read_bytes().rstrip(b"\n")
        long_line = long_source * ((64 << 10) // len(long_source)) + b"\n"
        long_lines = fixture(fixture_dir / "long.txt", "64 KiB lines",
                             long_line, size)
        cases.extend((
            ("varied_miss", varied, ["-c", "غيرموجود"], 1, None),
            ("long_lines", long_lines, ["-c", "مكتبة"], 0, long_lines.lines),
        ))
        extra_fixtures = [(varied, varied_sources), (long_lines, ("msa.txt",))]
    report = []
    for name, source, options, expected_exit, expected_lines in cases:
        commands = {label: [str(binary), *options, str(source.path)]
                    for label, binary in binaries.items()}
        checked = {label: invoke(command, capture=True)
                   for label, command in commands.items()}
        for label, result in checked.items():
            if result.returncode != expected_exit:
                raise RuntimeError(f"{name} {label}: exit {result.returncode}: "
                                   f"{result.stderr.decode(errors='replace')}")
            if result.stderr:
                raise RuntimeError(f"{name} {label}: unexpected stderr: "
                                   f"{result.stderr.decode(errors='replace')}")
        if checked["before"].stdout != checked["after"].stdout:
            raise RuntimeError(f"{name}: before and after output differs")
        output = checked["before"].stdout
        if expected_lines is not None:
            if name == "json_spans":
                records = output.splitlines()
                if len(records) != expected_lines or not json.loads(records[0])["spans"]:
                    raise RuntimeError(f"{name}: unexpected JSON records or spans")
            elif int(output.strip()) != expected_lines:
                raise RuntimeError(f"{name}: selected-line count differs from fixture")
        elif output.strip() != b"0":
            raise RuntimeError(f"{name}: miss count is not zero")

        timings = {label: [] for label in binaries}
        execution_order = []
        for iteration in range(args.runs):
            order = ("before", "after") if iteration % 2 == 0 else ("after", "before")
            for label in order:
                start = time.perf_counter()
                result = invoke(commands[label])
                elapsed = time.perf_counter() - start
                if result.returncode != expected_exit or result.stderr:
                    raise RuntimeError(f"{name} {label}: timed run failed: "
                                       f"{result.stderr.decode(errors='replace')}")
                timings[label].append(elapsed)
                execution_order.append(label)

        report.append({
            "name": name,
            "fixture": source.name,
            "commands": commands,
            "expected_exit": expected_exit,
            "output_bytes": len(output),
            "output_sha256": hashlib.sha256(output).hexdigest(),
            "execution_order": execution_order,
            "timings": {
                label: {"seconds": values, "median_seconds": statistics.median(values)}
                for label, values in timings.items()
            },
        })

    metadata = {
        "date_utc": datetime.now(timezone.utc).isoformat(),
        "platform": platform.platform(),
        "processor": cpu_model(),
        "logical_cpus": os.cpu_count(),
        "go_version": invoke(["go", "version"], capture=True).stdout.decode().strip(),
        "locale": "C.UTF-8",
        "runs_per_binary": args.runs,
        "target_size_mib": args.size_mib,
        "corpus_output_check": corpus_check,
        "binaries": {
            label: {"path": str(binary), "revision": revision,
                    "sha256": sha256(binary)}
            for (label, binary), revision in zip(
                binaries.items(), (args.before_revision, args.after_revision))
        },
        "source_patch": ({"path": str(args.patch.resolve()),
                          "sha256": sha256(args.patch.resolve())}
                         if args.patch else None),
        "fixtures": [
            {"name": source.name, "path": str(source.path), "bytes": source.bytes,
             "lines": source.lines, "sha256": sha256(source.path),
             "sources": {name: sha256(CORPUS / name) for name in names},
             "generation": ("round-robin source lines with unique numeric suffix"
                            if args.phase2_cases and source is extra_fixtures[0][0]
                            else "64 KiB line repeated"
                            if args.phase2_cases and source is extra_fixtures[1][0]
                            else "repeat source bytes")}
            for source, names in [(msa, ("msa.txt",)), (quran, ("quran.txt",)),
                                  *extra_fixtures]
        ],
        "cases": report,
    }
    output_path = args.output.resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n",
                           encoding="utf-8")
    print(f"Wrote {output_path}")
    for case in report:
        before = case["timings"]["before"]["median_seconds"] * 1000
        after = case["timings"]["after"]["median_seconds"] * 1000
        print(f"{case['name']:14} {before:8.1f} ms -> {after:8.1f} ms "
              f"({before / after:.2f}x)")


if __name__ == "__main__":
    main()
