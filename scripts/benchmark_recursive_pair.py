#!/usr/bin/env python3
"""Compare recursive agrep searches on retained small- and large-file trees."""

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

from benchmark import CORPUS, ROOT, cpu_model, sha256


def invoke(binary, args, *, capture=False):
    env = os.environ.copy()
    env["LC_ALL"] = "C.UTF-8"
    return subprocess.run(
        [str(binary), *args], cwd=ROOT, env=env,
        stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
        stderr=subprocess.PIPE, check=False,
    )


def write_trees(fixture_dir, phase6_cases=False):
    small = fixture_dir / "many-small"
    large = fixture_dir / "few-large"
    errors = fixture_dir / "errors"
    for root in (small, large, errors):
        root.mkdir(parents=True, exist_ok=True)

    source = (CORPUS / "msa.txt").read_bytes()
    miss = "سطر عادي بلا تطابق.\n".encode()
    small_contents = source + miss * 7
    small_files = []
    small_other = []
    for group in range(10):
        directory = small / f"group-{group:02d}"
        directory.mkdir(exist_ok=True)
        for index in range(100):
            path = directory / f"part-{index:03d}.txt"
            path.write_bytes(small_contents)
            small_files.append(path)
        ignored = directory / "ignored.txt"
        skipped = directory / "skip.md"
        ignored.write_bytes(source)
        skipped.write_bytes(miss)
        small_other.extend((ignored, skipped))
    ignore_file = small / ".gitignore"
    ignore_file.write_text("ignored.txt\n", encoding="utf-8")

    large_contents = source + miss * ((4 << 20) // len(miss))
    large_files = []
    for index in range(4):
        path = large / f"part-{index:02d}.txt"
        path.write_bytes(large_contents)
        large_files.append(path)

    invalid = errors / "a-invalid.txt"
    valid = errors / "b-valid.txt"
    invalid.write_bytes(source + b"\xff\n")
    valid.write_bytes(source)
    for root, expected in (
        (small, {*small_files, *small_other, ignore_file}),
        (large, set(large_files)),
        (errors, {invalid, valid}),
    ):
        actual = {path for path in root.rglob("*") if path.is_file()}
        if actual != expected:
            raise RuntimeError(f"unexpected files under {root}; use a clean fixture directory")
    extra = []
    if phase6_cases:
        late = fixture_dir / "few-large-late"
        absent = fixture_dir / "few-large-absent"
        late.mkdir(parents=True, exist_ok=True)
        absent.mkdir(parents=True, exist_ok=True)
        for index in range(4):
            name = f"part-{index:02d}.txt"
            (late / name).write_bytes(miss * ((4 << 20) // len(miss)) + source)
            (absent / name).write_bytes(miss * ((4 << 20) // len(miss)))
        extra = [late, absent]
    return small, large, errors, small_files, large_files, extra


def tree_record(root):
    digest = hashlib.sha256()
    files = sorted(path for path in root.rglob("*") if path.is_file())
    for path in files:
        digest.update(str(path.relative_to(root)).encode())
        digest.update(bytes.fromhex(sha256(path)))
    return {"path": str(root), "files": len(files),
            "bytes": sum(path.stat().st_size for path in files),
            "combined_sha256": digest.hexdigest()}


def request_cold_data(files):
    # Advisory eviction of file contents only; directory metadata can remain
    # cached, and the kernel may decline to evict pages still in use.
    for path in files:
        fd = os.open(path, os.O_RDONLY)
        try:
            os.posix_fadvise(fd, 0, 0, os.POSIX_FADV_DONTNEED)
        finally:
            os.close(fd)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", type=Path, required=True)
    parser.add_argument("--after", type=Path, required=True)
    parser.add_argument("--before-revision", required=True)
    parser.add_argument("--after-revision", required=True)
    parser.add_argument("--before-patch", type=Path)
    parser.add_argument("--after-patch", type=Path)
    parser.add_argument("--fixture-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--runs", type=int, default=7)
    parser.add_argument("--phase6-cases", action="store_true",
                        help="also measure late hits and absent matches in large files")
    args = parser.parse_args()
    if args.runs < 1:
        parser.error("--runs must be positive")
    if not hasattr(os, "posix_fadvise") or not hasattr(os, "POSIX_FADV_DONTNEED"):
        parser.error("cold-data measurement needs os.posix_fadvise")

    binaries = {"before": args.before.resolve(), "after": args.after.resolve()}
    for binary in binaries.values():
        if not binary.is_file():
            parser.error(f"binary does not exist: {binary}")

    fixture_dir = args.fixture_dir.resolve()
    small, large, errors, small_files, large_files, extra_trees = write_trees(
        fixture_dir, args.phase6_cases)
    small_lines = [str(path) + "\n" for path in small_files]
    large_lines = [str(path) + "\n" for path in large_files]
    cases = [
        ("many_small_hit", small, ["-r", "--threads=4", "-l", "اعلنت", str(small)],
         0, "".join(small_lines).encode()),
        ("few_large_hit", large, ["-r", "--threads=4", "-l", "اعلنت", str(large)],
         0, "".join(large_lines).encode()),
        ("few_large_without_match", large, ["-r", "--threads=4", "-L", "اعلنت", str(large)],
         1, b""),
        ("many_small_miss", small, ["-r", "--threads=4", "-l", "غيرموجود", str(small)],
         1, b""),
        ("many_small_count", small,
         ["-r", "--threads=4", "--include=*.txt", "-c", "اعلنت", str(small)],
         0, "".join(str(path) + ":1\n" for path in small_files).encode()),
        ("many_small_filtered", small,
         ["-r", "--threads=4", "--include=*.txt", "--exclude=group-00/part-*.txt",
          "-l", "اعلنت", str(small)], 0, "".join(small_lines[100:]).encode()),
    ]
    if args.phase6_cases:
        for name, root, option in (
            ("few_large_late_hit", extra_trees[0], "-l"),
            ("few_large_absent", extra_trees[1], "-L"),
        ):
            expected = "".join(str(root / f"part-{index:02d}.txt") + "\n"
                               for index in range(4)).encode()
            cases.append((name, root,
                          ["-r", "--threads=4", option, "اعلنت", str(root)],
                          0, expected))

    # Compare thread counts, ignore behavior, and late errors before timing.
    checks = []
    for threads in (1, 4, 16):
        checks.append((f"ordering_threads_{threads}",
                       ["-r", f"--threads={threads}", "-l", "اعلنت", str(small)],
                       0, "".join(small_lines).encode(), False))
    ignored = [str(small / f"group-{group:02d}" / "ignored.txt") + "\n"
               for group in range(10)]
    checks.append(("no_ignore",
                   ["-r", "--no-ignore", "--threads=4", "-l", "اعلنت", str(small)],
                   0, "".join(sorted(small_lines + ignored)).encode(), False))
    checks.append(("late_invalid_utf8",
                   ["-r", "--threads=4", "-l", "اعلنت", str(errors)],
                   2, (str(errors / "b-valid.txt") + "\n").encode(), True))

    checked = []
    for name, command, expected_exit, expected_output, expects_error in [
            *[(name, command, code, output, False) for name, _, command, code, output in cases],
            *checks]:
        results = {label: invoke(binary, command, capture=True)
                   for label, binary in binaries.items()}
        for label, result in results.items():
            if result.returncode != expected_exit or result.stdout != expected_output or \
                    bool(result.stderr) != expects_error:
                raise RuntimeError(f"{name} {label}: unexpected exit/output/error: "
                                   f"{result.returncode}, {result.stdout[:200]!r}, "
                                   f"{result.stderr[:200]!r}")
        if results["before"].stderr != results["after"].stderr:
            raise RuntimeError(f"{name}: stderr differs")
        checked.append({"name": name, "exit": expected_exit,
                        "stdout_sha256": hashlib.sha256(expected_output).hexdigest(),
                        "stderr_sha256": hashlib.sha256(results["before"].stderr).hexdigest()})

    report = []
    for name, root, command, expected_exit, _ in cases:
        for cache_mode in ("warm", "cold_data_requested"):
            if cache_mode == "warm":
                for binary in binaries.values():
                    invoke(binary, command)
            timings = {label: [] for label in binaries}
            order_record = []
            for iteration in range(args.runs):
                order = ("before", "after") if iteration % 2 == 0 else ("after", "before")
                for label in order:
                    if cache_mode == "cold_data_requested":
                        request_cold_data([path for path in root.rglob("*") if path.is_file()])
                    start = time.perf_counter()
                    result = invoke(binaries[label], command)
                    elapsed = time.perf_counter() - start
                    if result.returncode != expected_exit or result.stderr:
                        raise RuntimeError(f"{name} {cache_mode} {label}: timed run failed")
                    timings[label].append(elapsed)
                    order_record.append(label)
            report.append({"name": name, "cache_mode": cache_mode,
                           "commands": {label: [str(binary), *command]
                                        for label, binary in binaries.items()},
                           "execution_order": order_record,
                           "timings": {label: {"seconds": values,
                                               "median_seconds": statistics.median(values)}
                                       for label, values in timings.items()}})

    metadata = {
        "date_utc": datetime.now(timezone.utc).isoformat(),
        "platform": platform.platform(), "processor": cpu_model(),
        "logical_cpus": os.cpu_count(),
        "go_version": invoke("go", ["version"], capture=True).stdout.decode().strip(),
        "locale": "C.UTF-8", "runs_per_binary": args.runs,
        "cold_data_method": "POSIX_FADV_DONTNEED requested before each timed run; file data only, advisory",
        "binaries": {label: {"path": str(binary), "revision": revision,
                             "sha256": sha256(binary)}
                     for (label, binary), revision in zip(
                         binaries.items(), (args.before_revision, args.after_revision))},
        "patches": {"before": {"path": str(args.before_patch.resolve()),
                               "sha256": sha256(args.before_patch.resolve())}
                    if args.before_patch else None,
                    "after": {"path": str(args.after_patch.resolve()),
                              "sha256": sha256(args.after_patch.resolve())}
                    if args.after_patch else None},
        "fixtures": {root.name: tree_record(root)
                     for root in (small, large, errors, *extra_trees)},
        "source_sha256": sha256(CORPUS / "msa.txt"),
        "checks": checked, "cases": report,
    }
    output = args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {output}")
    for case in report:
        before = case["timings"]["before"]["median_seconds"] * 1000
        after = case["timings"]["after"]["median_seconds"] * 1000
        print(f"{case['name']:24} {case['cache_mode']:19} "
              f"{before:8.1f} -> {after:8.1f} ms ({before / after:.2f}x)")


if __name__ == "__main__":
    main()
