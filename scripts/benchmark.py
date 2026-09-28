#!/usr/bin/env python3
"""Reproducible CLI workloads; compare tools only when their results agree."""

import argparse
import hashlib
import json
import os
import platform
import shutil
import statistics
import subprocess
import tempfile
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CORPUS = ROOT / "testdata" / "corpus"


@dataclass
class Fixture:
    name: str
    path: Path
    bytes: int
    lines: int


def run(command, expected=(0,), capture=False):
    env = os.environ.copy()
    env["LC_ALL"] = "C.UTF-8"
    result = subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode not in expected:
        raise RuntimeError(
            f"{command!r} exited {result.returncode}: "
            f"{result.stderr.decode(errors='replace')}"
        )
    return result.stdout if capture else None


def count(command):
    return int(run(command, expected=(0, 1), capture=True).strip() or b"0")


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def cpu_model():
    cpuinfo = Path("/proc/cpuinfo")
    if cpuinfo.exists():
        for line in cpuinfo.read_text().splitlines():
            if line.startswith("model name"):
                return line.partition(":")[2].strip()
    return platform.processor() or "unknown"


def fixture(path, name, source, target_bytes):
    repeats = max(1, (target_bytes + len(source) - 1) // len(source))
    with path.open("wb") as output:
        for _ in range(repeats):
            output.write(source)
    return Fixture(name, path, path.stat().st_size, source.count(b"\n") * repeats)


def varied_fixture(path, target_bytes):
    """Cycle original lines with unique suffixes, including a >4 KiB line."""
    lines = [line for line in (CORPUS / "varied.txt").read_bytes().splitlines() if line]
    size = count = 0
    with path.open("wb") as output:
        while size < target_bytes:
            line = lines[count % len(lines)] + b" #" + str(count).encode() + b"\n"
            output.write(line)
            size += len(line)
            count += 1
    return Fixture("varied unique Arabic/Persian lines", path, size, count)


def measure(commands, runs, expected=(0,)):
    for command in commands.values():
        run(command, expected=expected)
    samples = {name: [] for name in commands}
    names = list(commands)
    for iteration in range(runs):
        for name in names[iteration % len(names):] + names[:iteration % len(names)]:
            start = time.perf_counter()
            run(commands[name], expected=expected)
            samples[name].append(time.perf_counter() - start)
    return {
        name: {
            "seconds": values,
            "median_seconds": statistics.median(values),
            "min_seconds": min(values),
            "max_seconds": max(values),
        }
        for name, values in samples.items()
    }


def add_case(report, name, description, input_fixture, commands, runs,
             expected_lines=None, same_result=False, raw_reference=None,
             count_commands=None, output_lines=None, expected_exit=(0,)):
    selected = {}
    if count_commands:
        selected = {tool: count(command) for tool, command in count_commands.items()}
        if expected_lines is not None and selected["agrep"] != expected_lines:
            raise RuntimeError(f"{name}: expected {expected_lines} lines, got {selected}")
        if same_result and len(set(selected.values())) != 1:
            raise RuntimeError(f"{name}: selected-line counts differ: {selected}")
    if output_lines is not None:
        for tool, command in commands.items():
            actual = run(command, capture=True).count(b"\n")
            if actual != output_lines:
                raise RuntimeError(f"{name}: {tool} emitted {actual} lines; want {output_lines}")
    timings = measure(commands, runs, expected_exit)
    for result in timings.values():
        result["mib_per_second"] = input_fixture.bytes / (1 << 20) / result["median_seconds"]
    report.append({
        "name": name,
        "description": description,
        "fixture": input_fixture.name,
        "fixture_sha256": sha256(input_fixture.path) if input_fixture.path.is_file() else None,
        "bytes": input_fixture.bytes,
        "lines": input_fixture.lines,
        "same_result_across_tools": same_result,
        "selected_lines": selected,
        "output_lines": output_lines,
        "raw_reference_counts": raw_reference,
        "commands": commands,
        "timings": timings,
    })


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--size-mib", type=int, default=16)
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--agrep", type=Path, help="use an existing agrep binary")
    parser.add_argument("--revision", help="source revision of the measured agrep binary")
    parser.add_argument("--output", type=Path, help="write JSON results")
    args = parser.parse_args()
    if args.size_mib <= 0 or args.runs <= 0:
        parser.error("--size-mib and --runs must be positive")
    for program in ("grep", "rg"):
        if shutil.which(program) is None:
            parser.error(f"{program} is required")

    with tempfile.TemporaryDirectory(prefix="agrep-benchmark-") as temporary:
        temp = Path(temporary)
        binary = args.agrep.resolve() if args.agrep else temp / "agrep"
        if not args.agrep:
            run(["go", "build", "-trimpath", "-o", str(binary), "./cmd/agrep"])
        target = args.size_mib << 20
        msa = fixture(temp / "msa.txt", "repeated MSA", (CORPUS / "msa.txt").read_bytes(), target)
        quran = fixture(temp / "quran.txt", "repeated voweled Arabic", (CORPUS / "quran.txt").read_bytes(), target)
        ocr = fixture(temp / "ocr.txt", "repeated OCR", (CORPUS / "ocr.txt").read_bytes(), target)
        encoded = fixture(temp / "cp1256.txt", "repeated CP1256 MSA", (CORPUS / "msa.txt").read_text().encode("cp1256"), target)
        long_source = (CORPUS / "msa.txt").read_bytes().rstrip(b"\n")
        long_line = long_source * ((64 << 10) // len(long_source)) + b"\n"
        long_lines = fixture(temp / "long.txt", "64 KiB lines", long_line, target)
        varied = varied_fixture(temp / "varied.txt", target)
        tree = temp / "tree"
        tree.mkdir()
        tree_size = max(1, target // 32)
        tree_line = (CORPUS / "msa.txt").read_bytes()
        for index in range(32):
            fixture(tree / f"part-{index:02d}.txt", "recursive shard", tree_line, tree_size)
        tree_fixture = Fixture("32-file tree", tree, sum(p.stat().st_size for p in tree.iterdir()), 32)

        report = []
        exact_counts = {
            "agrep": [str(binary), "-c", "مكتبة", str(msa.path)],
            "grep": ["grep", "-Fc", "مكتبة", str(msa.path)],
            "ripgrep": ["rg", "-Fc", "مكتبة", str(msa.path)],
        }
        add_case(report, "exact_count", "Exact hit, selected-line count", msa,
                 exact_counts, args.runs, msa.lines, True, count_commands=exact_counts)

        miss_counts = {
            "agrep": [str(binary), "-c", "غيرموجود", str(msa.path)],
            "grep": ["grep", "-Fc", "غيرموجود", str(msa.path)],
            "ripgrep": ["rg", "-Fc", "غيرموجود", str(msa.path)],
        }
        add_case(report, "literal_miss", "No selected lines; full scan", msa,
                 miss_counts, args.runs, 0, True, count_commands=miss_counts,
                 expected_exit=(0, 1))

        line_commands = {
            "agrep": [str(binary), "-n", "مكتبة", str(msa.path)],
            "grep": ["grep", "-nF", "مكتبة", str(msa.path)],
            "ripgrep": ["rg", "-nF", "مكتبة", str(msa.path)],
        }
        add_case(report, "line_output", "Emit numbered matching lines to /dev/null", msa,
                 line_commands, args.runs, same_result=True, output_lines=msa.lines)

        for name, source, query, expected, description in (
            ("hamza_fold", msa, "اعلنت", msa.lines, "Alef-hamza fold required"),
            ("voweled", quran, "قال", quran.lines, "Tashkil removal required"),
        ):
            raw = {
                "grep": count(["grep", "-Fc", query, str(source.path)]),
                "ripgrep": count(["rg", "-Fc", query, str(source.path)]),
            }
            command = [str(binary), "-c", query, str(source.path)]
            add_case(report, name, description, source, {"agrep": command},
                     args.runs, expected, raw_reference=raw,
                     count_commands={"agrep": command})

        json_command = [str(binary), "--json", "مكتبه", str(msa.path)]
        json_lines = run(json_command, capture=True)
        if json_lines.count(b"\n") != msa.lines or not json.loads(json_lines.splitlines()[0])["spans"]:
            raise RuntimeError("JSON span output differs from expected selected lines")
        add_case(report, "json_spans", "Emit mapped JSON spans to /dev/null", msa,
                 {"agrep": json_command}, args.runs, output_lines=msa.lines)

        fuzzy_command = [str(binary), "--fuzzy=1", "-c", "كتاب", str(ocr.path)]
        add_case(report, "fuzzy", "One-edit fuzzy count on OCR lines", ocr,
                 {"agrep": fuzzy_command}, args.runs, ocr.lines,
                 count_commands={"agrep": fuzzy_command})

        encoded_command = [str(binary), "--encoding=cp1256", "-c", "مكتبه", str(encoded.path)]
        add_case(report, "legacy_decode", "CP1256 decode and normalized count", encoded,
                 {"agrep": encoded_command}, args.runs, encoded.lines,
                 count_commands={"agrep": encoded_command})

        long_counts = {
            "agrep": [str(binary), "-c", "مكتبة", str(long_lines.path)],
            "grep": ["grep", "-Fc", "مكتبة", str(long_lines.path)],
            "ripgrep": ["rg", "-Fc", "مكتبة", str(long_lines.path)],
        }
        add_case(report, "long_lines", "64 KiB logical lines", long_lines,
                 long_counts, args.runs, long_lines.lines, True, count_commands=long_counts)

        varied_exact = {
            "agrep": [str(binary), "-c", "فارسی", str(varied.path)],
            "grep": ["grep", "-Fc", "فارسی", str(varied.path)],
            "ripgrep": ["rg", "-Fc", "فارسی", str(varied.path)],
        }
        add_case(report, "varied_exact_count", "Exact sparse hit on unique mixed lines",
                 varied, varied_exact, args.runs, same_result=True,
                 count_commands=varied_exact)

        varied_miss = {
            "agrep": [str(binary), "-c", "غيرموجود", str(varied.path)],
            "grep": ["grep", "-Fc", "غيرموجود", str(varied.path)],
            "ripgrep": ["rg", "-Fc", "غيرموجود", str(varied.path)],
        }
        add_case(report, "varied_literal_miss", "No hit on unique mixed lines",
                 varied, varied_miss, args.runs, 0, True,
                 count_commands=varied_miss, expected_exit=(0, 1))

        varied_count = [str(binary), "-c", "مكتبه", str(varied.path)]
        varied_selected = count(varied_count)
        raw_varied = {
            "grep": count(["grep", "-Fc", "مكتبه", str(varied.path)]),
            "ripgrep": count(["rg", "-Fc", "مكتبه", str(varied.path)]),
        }
        add_case(report, "varied_normalized_count", "Sparse normalized hits on mixed lines",
                 varied, {"agrep": varied_count}, args.runs, varied_selected,
                 raw_reference=raw_varied, count_commands={"agrep": varied_count})
        varied_json = [str(binary), "--json", "مكتبه", str(varied.path)]
        add_case(report, "varied_json_spans", "Mapped sparse spans on mixed lines",
                 varied, {"agrep": varied_json}, args.runs,
                 output_lines=varied_selected)

        tree_commands = {
            "agrep": [str(binary), "-r", "-l", "مكتبة", str(tree)],
            "grep": ["grep", "-rlF", "مكتبة", str(tree)],
            "ripgrep": ["rg", "-lF", "مكتبة", str(tree)],
        }
        for tool, command in tree_commands.items():
            found = {Path(path).name for path in run(command, capture=True).decode().splitlines()}
            if found != {f"part-{index:02d}.txt" for index in range(32)}:
                raise RuntimeError(f"recursive results differ for {tool}: {found}")
        add_case(report, "recursive_files", "Find matching files in a 32-file tree", tree_fixture,
                 tree_commands, args.runs, same_result=True)

        results = {
            "date_utc": datetime.now(timezone.utc).isoformat(),
            "platform": platform.platform(),
            "processor": cpu_model(),
            "logical_cpus": os.cpu_count(),
            "go_version": run(["go", "version"], capture=True).decode().strip(),
            "grep_version": run(["grep", "--version"], capture=True).decode().splitlines()[0],
            "ripgrep_version": run(["rg", "--version"], capture=True).decode().splitlines()[0],
            "runs": args.runs,
            "locale": "C.UTF-8",
            "agrep_revision": args.revision or run(["git", "rev-parse", "HEAD"], capture=True).decode().strip(),
            "target_size_mib": args.size_mib,
            "fixture_sources": [str((CORPUS / name).relative_to(ROOT)) for name in ("msa.txt", "quran.txt", "ocr.txt", "varied.txt")],
            "fixture_source_sha256": {name: sha256(CORPUS / name) for name in ("msa.txt", "quran.txt", "ocr.txt", "varied.txt")},
            "agrep_binary_sha256": sha256(binary),
            "cases": report,
        }
        output = json.dumps(results, ensure_ascii=False, indent=2) + "\n"
        if args.output:
            args.output.write_text(output, encoding="utf-8")
        print(output, end="")


if __name__ == "__main__":
    main()
