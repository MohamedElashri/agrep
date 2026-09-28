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

from benchmark import CORPUS, ROOT, cpu_model, fixture, sha256


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
    args = parser.parse_args()
    if args.size_mib <= 0 or args.runs <= 0:
        parser.error("--size-mib and --runs must be positive")

    binaries = {"before": args.before.resolve(), "after": args.after.resolve()}
    for binary in binaries.values():
        if not binary.is_file():
            parser.error(f"binary does not exist: {binary}")

    fixture_dir = args.fixture_dir.resolve()
    fixture_dir.mkdir(parents=True, exist_ok=True)
    size = args.size_mib << 20
    msa = fixture(fixture_dir / "msa.txt", "repeated MSA",
                  (CORPUS / "msa.txt").read_bytes(), size)
    quran = fixture(fixture_dir / "quran.txt", "repeated voweled Arabic",
                    (CORPUS / "quran.txt").read_bytes(), size)

    cases = (
        ("exact_count", msa, ["-c", "مكتبة"], 0, msa.lines),
        ("literal_miss", msa, ["-c", "غيرموجود"], 1, None),
        ("hamza_fold", msa, ["-c", "اعلنت"], 0, msa.lines),
        ("voweled", quran, ["-c", "قال"], 0, quran.lines),
        ("json_spans", msa, ["--json", "مكتبه"], 0, msa.lines),
    )
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
             "source": str((CORPUS / source_name).relative_to(ROOT)),
             "source_sha256": sha256(CORPUS / source_name)}
            for source, source_name in ((msa, "msa.txt"), (quran, "quran.txt"))
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
