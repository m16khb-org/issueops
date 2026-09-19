#!/usr/bin/env python3

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


SCHEMA_VERSION = 1
ENVIRONMENT_KEYS = (
    "CGO_ENABLED",
    "GOARCH",
    "GOFLAGS",
    "GOMAXPROCS",
    "GOOS",
    "GOTOOLCHAIN",
)
ERROR_KEYS = {"error", "errors"}
WARNING_KEYS = {"warning", "warnings"}
SENSITIVE_KEY = re.compile(
    r"(?:^|_)(?:access_token|api_key|authorization|credential|password|secret|token)(?:$|_)",
    re.IGNORECASE,
)
REDACTION_MARKER = "<redacted>"


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def relative_path(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root.resolve()).as_posix()
    except ValueError as error:
        raise ValueError(f"path is outside workspace root: {path}") from error


def file_metadata(path: Path, root: Path) -> dict[str, Any]:
    content = path.read_bytes()
    return {
        "path": relative_path(path, root),
        "bytes": len(content),
        "sha256": sha256_bytes(content),
    }


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")


def json_pointer(parts: tuple[str, ...]) -> str:
    if not parts:
        return ""
    escaped = [part.replace("~", "~0").replace("/", "~1") for part in parts]
    return "/" + "/".join(escaped)


def observation(path: str, value: Any) -> dict[str, Any]:
    count = len(value) if isinstance(value, (dict, list)) else 1
    return {"path": path, "count": count, "sha256": sha256_bytes(canonical_json(value))}


def redaction_marker_count(value: Any) -> int:
    if isinstance(value, str):
        return value.count(REDACTION_MARKER)
    if isinstance(value, dict):
        return sum(redaction_marker_count(child) for child in value.values())
    if isinstance(value, list):
        return sum(redaction_marker_count(child) for child in value)
    return 0


def contract_projection(value: Any) -> dict[str, Any]:
    field_paths: list[str] = []
    errors: list[dict[str, Any]] = []
    warnings: list[dict[str, Any]] = []
    redactions: dict[str, dict[str, Any]] = {}

    def walk(child: Any, parts: tuple[str, ...]) -> None:
        if isinstance(child, dict):
            for key in sorted(child):
                value_at_key = child[key]
                child_parts = (*parts, str(key))
                path = json_pointer(child_parts)
                field_paths.append(path)
                lowered = str(key).lower()
                if lowered in ERROR_KEYS or lowered.endswith("_error") or lowered.endswith("_errors"):
                    errors.append(observation(path, value_at_key))
                if lowered in WARNING_KEYS or lowered.endswith("_warning") or lowered.endswith("_warnings"):
                    warnings.append(observation(path, value_at_key))
                marker_count = redaction_marker_count(value_at_key)
                if SENSITIVE_KEY.search(str(key)) or marker_count:
                    redactions[path] = {
                        "path": path,
                        "masked": marker_count > 0,
                        "marker_count": marker_count,
                    }
                walk(value_at_key, child_parts)
        elif isinstance(child, list):
            for index, item in enumerate(child):
                walk(item, (*parts, str(index)))
        elif isinstance(child, str) and REDACTION_MARKER in child:
            path = json_pointer(parts)
            redactions.setdefault(
                path,
                {"path": path, "masked": True, "marker_count": child.count(REDACTION_MARKER)},
            )

    walk(value, ())
    return {
        "sha256": sha256_bytes(canonical_json(value)),
        "field_paths": sorted(field_paths),
        "errors": sorted(errors, key=lambda item: item["path"]),
        "warnings": sorted(warnings, key=lambda item: item["path"]),
        "redactions": [redactions[path] for path in sorted(redactions)],
    }


def summarize_go_test_json(raw: bytes) -> dict[str, Any]:
    package_elapsed: dict[str, float] = {}
    counts = {
        "packages_started": 0,
        "packages_finished": 0,
        "tests_started": 0,
        "tests_finished": 0,
        "output_events": 0,
    }
    for line_number, raw_line in enumerate(raw.splitlines(), start=1):
        if not raw_line.strip():
            continue
        try:
            event = json.loads(raw_line)
        except json.JSONDecodeError as error:
            raise ValueError(f"invalid go test JSON at line {line_number}: {error.msg}") from error
        if not isinstance(event, dict):
            raise ValueError(f"invalid go test JSON event at line {line_number}: expected object")
        action = event.get("Action")
        package = event.get("Package")
        test = event.get("Test")
        if action == "start" and package and not test:
            counts["packages_started"] += 1
        if action in {"pass", "fail", "skip"} and package and not test:
            counts["packages_finished"] += 1
            elapsed = event.get("Elapsed")
            if isinstance(elapsed, (int, float)):
                package_elapsed[str(package)] = float(elapsed)
        if action == "run" and package and test:
            counts["tests_started"] += 1
        if action in {"pass", "fail", "skip"} and package and test:
            counts["tests_finished"] += 1
        if action == "output":
            counts["output_events"] += 1
    return {
        "package_elapsed_seconds": dict(sorted(package_elapsed.items())),
        "package_elapsed_sum_seconds": round(sum(package_elapsed.values()), 9),
        "package_elapsed_semantics": "overlapping package elapsed sum; never substitute for whole wall time",
        "event_counts": counts,
    }


def environment_metadata() -> dict[str, Any]:
    return {
        "system": platform.system(),
        "release": platform.release(),
        "machine": platform.machine(),
        "cpu_count": os.cpu_count(),
        "python_version": platform.python_version(),
        "selected_env": {key: os.environ[key] for key in ENVIRONMENT_KEYS if key in os.environ},
    }


def run_record(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace_root).resolve()
    output_dir = Path(args.output_dir).resolve()
    efficiency_root = workspace_root / ".issueops-runtime" / "efficiency"
    relative_path(output_dir, efficiency_root)
    if output_dir.exists():
        raise ValueError(f"output directory already exists: {output_dir}")

    contract_path = Path(args.contract_file).resolve()
    relative_path(contract_path, workspace_root)
    contract_bytes = contract_path.read_bytes()
    contract_value = json.loads(contract_bytes)
    projection = contract_projection(contract_value)
    input_paths = [Path(value).resolve() for value in args.input_file]
    inputs = [file_metadata(path, workspace_root) for path in input_paths]

    command = list(args.command)
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        raise ValueError("record requires a command after --")

    output_dir.mkdir(parents=True)
    started = time.perf_counter()
    timed_out = False
    try:
        completed = subprocess.run(
            command,
            cwd=workspace_root,
            capture_output=True,
            timeout=args.timeout_seconds,
            check=False,
        )
        stdout = completed.stdout
        stderr = completed.stderr
        exit_code = completed.returncode
    except subprocess.TimeoutExpired as error:
        timed_out = True
        stdout = error.stdout or b""
        stderr = error.stderr or b""
        exit_code = 124
    wall_time_seconds = time.perf_counter() - started

    stdout_path = output_dir / "stdout.log"
    stderr_path = output_dir / "stderr.log"
    contract_artifact_path = output_dir / "contract.json"
    stdout_path.write_bytes(stdout)
    stderr_path.write_bytes(stderr)
    shutil.copyfile(contract_path, contract_artifact_path)

    go_test = summarize_go_test_json(stdout) if args.parser == "go-test-json" else None
    parser_counts = go_test["event_counts"] if go_test else {
        "packages_started": 0,
        "packages_finished": 0,
        "tests_started": 0,
        "tests_finished": 0,
        "output_events": 0,
    }
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "measurement": {
            "label": args.label,
            "series_id": args.series_id,
            "variant": args.variant,
            "sequence": args.sequence,
            "revision": args.revision,
            "recorded_at": datetime.now(timezone.utc).isoformat(),
        },
        "tool": {"name": args.tool_name, "version": args.tool_version},
        "environment": environment_metadata(),
        "inputs": inputs,
        "contract": {
            "bytes": len(contract_bytes),
            "raw_sha256": sha256_bytes(contract_bytes),
            **projection,
        },
        "command": {
            "argv": command,
            "cwd": ".",
            "parser": args.parser,
            "timeout_seconds": args.timeout_seconds,
        },
        "counts": {
            "runs": 1,
            "inputs": len(inputs),
            "stdout_bytes": len(stdout),
            "stderr_bytes": len(stderr),
            "contract_field_paths": len(projection["field_paths"]),
            "error_observations": len(projection["errors"]),
            "warning_observations": len(projection["warnings"]),
            "redaction_observations": len(projection["redactions"]),
            **parser_counts,
        },
        "execution": {
            "exit_code": exit_code,
            "timed_out": timed_out,
            "wall_time_seconds": wall_time_seconds,
        },
        "go_test": go_test,
        "artifacts": {
            "stdout": stdout_path.name,
            "stderr": stderr_path.name,
            "contract": contract_artifact_path.name,
        },
    }
    manifest_path = output_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"ok": exit_code == 0, "manifest": str(manifest_path)}, sort_keys=True))
    return exit_code


def read_manifest(path: Path) -> tuple[dict[str, Any], Any]:
    manifest_path = path.resolve()
    manifest = json.loads(manifest_path.read_bytes())
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ValueError(f"unsupported measurement schema in {manifest_path}")
    artifact_name = manifest.get("artifacts", {}).get("contract")
    if not isinstance(artifact_name, str) or not artifact_name:
        raise ValueError(f"contract artifact is missing in {manifest_path}")
    contract_path = (manifest_path.parent / artifact_name).resolve()
    relative_path(contract_path, manifest_path.parent)
    contract_bytes = contract_path.read_bytes()
    contract_value = json.loads(contract_bytes)
    projection = contract_projection(contract_value)
    recorded = manifest.get("contract", {})
    if recorded.get("raw_sha256") != sha256_bytes(contract_bytes) or recorded.get("sha256") != projection["sha256"]:
        raise ValueError(f"contract artifact digest does not match manifest: {manifest_path}")
    return manifest, contract_value


def comparison_condition_drifts(baseline: dict[str, Any], candidate: dict[str, Any]) -> list[dict[str, Any]]:
    drifts: list[dict[str, Any]] = []

    def require_equal(name: str, baseline_value: Any, candidate_value: Any) -> None:
        if baseline_value != candidate_value:
            drifts.append({"kind": "measurement", "condition": name})

    baseline_measurement = baseline.get("measurement", {})
    candidate_measurement = candidate.get("measurement", {})
    require_equal("series_id", baseline_measurement.get("series_id"), candidate_measurement.get("series_id"))
    if baseline_measurement.get("variant") != "baseline":
        drifts.append({"kind": "measurement", "condition": "baseline_variant"})
    if candidate_measurement.get("variant") != "candidate":
        drifts.append({"kind": "measurement", "condition": "candidate_variant"})
    baseline_sequence = baseline_measurement.get("sequence")
    candidate_sequence = candidate_measurement.get("sequence")
    if not isinstance(baseline_sequence, int) or candidate_sequence != baseline_sequence + 1:
        drifts.append({"kind": "measurement", "condition": "alternating_sequence"})
    require_equal("tool", baseline.get("tool"), candidate.get("tool"))
    require_equal("environment", baseline.get("environment"), candidate.get("environment"))
    require_equal("fixed_inputs", baseline.get("inputs"), candidate.get("inputs"))
    require_equal("command", baseline.get("command"), candidate.get("command"))
    for name, manifest in (("baseline", baseline), ("candidate", candidate)):
        execution = manifest.get("execution", {})
        if execution.get("exit_code") != 0 or execution.get("timed_out") is not False:
            drifts.append({"kind": "measurement", "condition": f"{name}_execution"})
    return drifts


def contract_drifts(baseline_value: Any, candidate_value: Any) -> list[dict[str, Any]]:
    baseline = contract_projection(baseline_value)
    candidate = contract_projection(candidate_value)
    drifts: list[dict[str, Any]] = []
    if baseline["field_paths"] != candidate["field_paths"]:
        baseline_fields = set(baseline["field_paths"])
        candidate_fields = set(candidate["field_paths"])
        drifts.append(
            {
                "kind": "field",
                "removed": sorted(baseline_fields - candidate_fields),
                "added": sorted(candidate_fields - baseline_fields),
            }
        )
    for kind, key in (("error", "errors"), ("warning", "warnings"), ("redaction", "redactions")):
        if baseline[key] != candidate[key]:
            drifts.append(
                {
                    "kind": kind,
                    "baseline_sha256": sha256_bytes(canonical_json(baseline[key])),
                    "candidate_sha256": sha256_bytes(canonical_json(candidate[key])),
                }
            )
    if baseline["sha256"] != candidate["sha256"]:
        drifts.append(
            {
                "kind": "output",
                "baseline_sha256": baseline["sha256"],
                "candidate_sha256": candidate["sha256"],
            }
        )
    return drifts


def timing_projection(manifest: dict[str, Any]) -> dict[str, Any]:
    go_test = manifest.get("go_test")
    return {
        "wall_time_seconds": manifest.get("execution", {}).get("wall_time_seconds"),
        "package_elapsed_sum_seconds": (
            go_test.get("package_elapsed_sum_seconds") if isinstance(go_test, dict) else None
        ),
    }


def run_compare(args: argparse.Namespace) -> int:
    baseline, baseline_contract = read_manifest(Path(args.baseline))
    candidate, candidate_contract = read_manifest(Path(args.candidate))
    condition_drifts = comparison_condition_drifts(baseline, candidate)
    output_drifts = contract_drifts(baseline_contract, candidate_contract)
    comparable = not condition_drifts
    contract_equal = not output_drifts
    drifts = [*condition_drifts, *output_drifts]
    result = {
        "ok": comparable and contract_equal,
        "comparable": comparable,
        "contract_equal": contract_equal,
        "baseline_revision": baseline.get("measurement", {}).get("revision"),
        "candidate_revision": candidate.get("measurement", {}).get("revision"),
        "drifts": drifts,
        "timing": {
            "baseline": timing_projection(baseline),
            "candidate": timing_projection(candidate),
        },
    }
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0 if result["ok"] else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Record and compare fixed-input efficiency evidence.")
    subparsers = parser.add_subparsers(dest="subcommand", required=True)
    record = subparsers.add_parser("record", help="run one command and record raw measurement evidence")
    record.add_argument("--workspace-root", required=True)
    record.add_argument("--output-dir", required=True)
    record.add_argument("--label", required=True)
    record.add_argument("--series-id", required=True)
    record.add_argument("--variant", choices=("baseline", "candidate"), required=True)
    record.add_argument("--sequence", type=int, required=True)
    record.add_argument("--revision", required=True)
    record.add_argument("--tool-name", required=True)
    record.add_argument("--tool-version", required=True)
    record.add_argument("--parser", choices=("none", "go-test-json"), default="none")
    record.add_argument("--contract-file", required=True)
    record.add_argument("--input-file", action="append", default=[])
    record.add_argument("--timeout-seconds", type=float, default=1800)
    record.add_argument("command", nargs=argparse.REMAINDER)
    compare = subparsers.add_parser("compare", help="compare fixed-input contract and timing evidence")
    compare.add_argument("--baseline", required=True)
    compare.add_argument("--candidate", required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        if args.subcommand == "record":
            return run_record(args)
        if args.subcommand == "compare":
            return run_compare(args)
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"measure-efficiency: {error}", file=sys.stderr)
        return 1
    parser.error(f"unsupported subcommand: {args.subcommand}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
