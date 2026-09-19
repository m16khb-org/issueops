#!/usr/bin/env python3

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import stat
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


SCHEMA_VERSION = 2
RELEVANT_ENVIRONMENT_KEYS = (
    "PATH_SHA256",
    "GOFLAGS",
    "GOWORK",
    "GOENV",
    "CGO_ENABLED",
    "CC",
    "CXX",
    "LANG",
    "LC_ALL",
    "LC_CTYPE",
)
ERROR_KEYS = {"error", "errors"}
WARNING_KEYS = {"warning", "warnings"}
REDACTION_MARKER = "<redacted>"
SENSITIVE_KEY = re.compile(
    r"(?:^|_)(?:access_key|access_token|api_key|authorization|client_secret|credential|passwd|password|secret|token)(?:$|_)",
    re.IGNORECASE,
)
SECRET_ASSIGNMENT = re.compile(
    r'''(?ix)["']?\b(?:[a-z0-9]+[_-])*(?:access[_-]?key|access[_-]?token|api[_-]?key|authorization|client[_-]?secret|credential|passwd|password|secret|token)\b["']?\s*[:=]\s*(?P<value>"[^"\r\n]*"|'[^'\r\n]*'|[^,;\r\n}]+)'''
)
SECRET_TOKEN = re.compile(
    r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|\bAKIA[0-9A-Z]{16}\b|\bgh[pousr]_[A-Za-z0-9]{20,}\b|\bsk-[A-Za-z0-9]{20,}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}\b"
)
SAFE_SECRET_VALUES = {
    "<redacted>",
    "Bearer <redacted>",
}
COUNT_KEYS = (
    "command_invocations",
    "samples",
    "inputs",
    "stdout_bytes",
    "stderr_bytes",
    "contract_fields",
    "contract_errors",
    "contract_warnings",
    "contract_redactions",
    "packages_started",
    "packages_finished",
    "tests_started",
    "tests_finished",
    "output_events",
)
EVENT_COUNT_KEYS = (
    "packages_started",
    "packages_finished",
    "tests_started",
    "tests_finished",
    "output_events",
)


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()


def inside(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


def relative_argument(raw: str, label: str) -> Path:
    path = Path(raw)
    if path.is_absolute():
        raise ValueError(f"{label} must be workspace-relative: {raw}")
    return path


def regular_file(root: Path, raw: str, label: str) -> Path:
    relative = relative_argument(raw, label)
    root = root.resolve()
    candidate = (root / relative).resolve(strict=False)
    if not inside(candidate, root):
        raise ValueError(f"{label} is outside workspace: {raw}")
    try:
        resolved = candidate.resolve(strict=True)
    except FileNotFoundError as error:
        raise ValueError(f"{label} is not a regular file: {raw}") from error
    if not inside(resolved, root):
        raise ValueError(f"{label} is outside workspace: {raw}")
    if not stat.S_ISREG(resolved.stat().st_mode):
        raise ValueError(f"{label} is not a regular file: {raw}")
    return resolved


def read_regular(root: Path, raw: str, label: str) -> tuple[Path, bytes]:
    path = regular_file(root, raw, label)
    return path, path.read_bytes()


def output_directory(root: Path, raw: str) -> Path:
    relative = relative_argument(raw, "output directory")
    root = root.resolve()
    efficiency_root = (root / ".issueops-runtime" / "efficiency").resolve(strict=False)
    if not inside(efficiency_root, root):
        raise ValueError("efficiency output root is outside workspace")
    target = (root / relative).resolve(strict=False)
    if target == efficiency_root or not inside(target, efficiency_root):
        raise ValueError(f"output directory is outside workspace efficiency root: {raw}")
    if target.exists():
        raise ValueError(f"output directory already exists: {raw}")
    return target


def parse_json(raw: bytes, label: str) -> Any:
    try:
        return json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError(f"invalid {label} JSON: {error}") from error


def require_object(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError(f"invalid {label}: expected object")
    return value


def required(value: dict[str, Any], key: str, kind: type, label: str) -> Any:
    if key not in value:
        raise ValueError(f"invalid {label}: missing {key}")
    child = value[key]
    if kind is int:
        valid = isinstance(child, int) and not isinstance(child, bool)
    else:
        valid = isinstance(child, kind)
    if not valid:
        raise ValueError(f"invalid {label}: {key}")
    if kind is str and not child:
        raise ValueError(f"invalid {label}: {key}")
    return child


def nonnegative_integer(value: dict[str, Any], key: str, label: str) -> int:
    child = required(value, key, int, label)
    if child < 0:
        raise ValueError(f"invalid {label}: {key}")
    return child


def positive_integer(value: dict[str, Any], key: str, label: str) -> int:
    child = required(value, key, int, label)
    if child < 1:
        raise ValueError(f"invalid {label}: {key}")
    return child


def finite_number(value: dict[str, Any], key: str, label: str) -> float:
    child = value.get(key)
    if isinstance(child, bool) or not isinstance(child, (int, float)) or not math.isfinite(child) or child < 0:
        raise ValueError(f"invalid {label}: {key}")
    return float(child)


def validate_environment(value: Any, label: str) -> dict[str, Any]:
    environment = require_object(value, label)
    for key in ("system", "release", "machine", "processor"):
        required(environment, key, str, label)
    positive_integer(environment, "cpu_count", label)
    variables = required(environment, "variables", dict, label)
    if set(variables) != set(RELEVANT_ENVIRONMENT_KEYS):
        raise ValueError(f"invalid {label}: environment variables must use the fixed relevant key set")
    if any(not isinstance(item, str) or not item for item in variables.values()):
        raise ValueError(f"invalid {label}: environment variables require values or <unset>")
    path_digest = variables["PATH_SHA256"]
    if path_digest != "<unset>" and not re.fullmatch(r"[0-9a-f]{64}", path_digest):
        raise ValueError(f"invalid {label}: environment variables require a PATH_SHA256 digest")
    return environment


def validate_command(value: Any, root: Path, label: str, *, parser: bool) -> dict[str, Any]:
    command = require_object(value, label)
    argv = required(command, "argv", list, label)
    if not argv or any(not isinstance(item, str) or not item for item in argv):
        raise ValueError(f"invalid {label}: argv")
    cwd = required(command, "cwd", str, label)
    relative_argument(cwd, f"{label} cwd")
    resolved_cwd = (root / cwd).resolve(strict=False)
    if not inside(resolved_cwd, root.resolve()) or not resolved_cwd.is_dir():
        raise ValueError(f"invalid {label}: cwd")
    if parser and required(command, "parser", str, label) not in {"none", "go-test-json"}:
        raise ValueError(f"invalid {label}: parser")
    return command


def go_test_count(argv: list[str]) -> int | None:
    if len(argv) < 2 or Path(argv[0]).name != "go" or argv[1] != "test":
        return None
    result = 1
    index = 2
    while index < len(argv):
        argument = argv[index]
        raw: str | None = None
        if argument.startswith("-count="):
            raw = argument.split("=", 1)[1]
        elif argument == "-count":
            index += 1
            if index >= len(argv):
                raise ValueError("invalid execution metadata: go test -count has no value")
            raw = argv[index]
        if raw is not None:
            try:
                result = int(raw)
            except ValueError as error:
                raise ValueError("invalid execution metadata: go test -count") from error
            if result < 1:
                raise ValueError("invalid execution metadata: go test -count")
        index += 1
    return result


def validate_execution(value: Any, root: Path, label: str = "execution metadata") -> dict[str, Any]:
    metadata = require_object(value, label)
    revision = required(metadata, "revision", str, label)
    if not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", revision):
        raise ValueError(f"invalid {label}: revision")
    tool = require_object(required(metadata, "tool", dict, label), f"{label} tool")
    required(tool, "name", str, f"{label} tool")
    required(tool, "version", str, f"{label} tool")
    environment = validate_environment(metadata.get("environment"), f"{label} environment")
    for key, item in environment["variables"].items():
        if SENSITIVE_KEY.search(key.replace("-", "_")) and not safe_secret_value(item):
            raise ValueError(f"unredacted secret-like material in {label}")
    command = validate_command(metadata.get("command"), root, f"{label} command", parser=False)
    for index, argument in enumerate(command["argv"]):
        option, separator, item = argument.partition("=")
        if option.startswith("-") and SENSITIVE_KEY.search(option.lstrip("-").replace("-", "_")):
            item = item if separator else command["argv"][index + 1] if index + 1 < len(command["argv"]) else ""
            if not safe_secret_value(item):
                raise ValueError(f"unredacted secret-like material in {label}")
    execution = require_object(required(metadata, "execution", dict, label), f"{label} execution")
    required(execution, "exit_code", int, f"{label} execution")
    required(execution, "timed_out", bool, f"{label} execution")
    finite_number(execution, "wall_time_seconds", f"{label} execution")
    positive_integer(execution, "command_invocations", f"{label} execution")
    samples = positive_integer(execution, "sample_count", f"{label} execution")
    count = go_test_count(command["argv"])
    if count is not None and count != samples:
        raise ValueError("sample_count does not match go test -count")
    return metadata


def safe_secret_value(value: str) -> bool:
    candidate = value.strip()
    if len(candidate) >= 2 and candidate[0] == candidate[-1] and candidate[0] in {'"', "'"}:
        candidate = candidate[1:-1]
    return candidate in SAFE_SECRET_VALUES


def secret_findings(raw: bytes, label: str) -> list[str]:
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as error:
        raise ValueError(f"{label} is not UTF-8 text") from error
    findings = ["token pattern"] if SECRET_TOKEN.search(text) else []
    for match in SECRET_ASSIGNMENT.finditer(text):
        if safe_secret_value(match.group("value")):
            continue
        findings.append("secret assignment")
    return findings


def scan_evidence(files: list[tuple[str, bytes]]) -> None:
    for label, raw in files:
        if secret_findings(raw, label):
            raise ValueError(f"unredacted secret-like material in {label}")


def pointer(parts: tuple[str, ...]) -> str:
    return "/" + "/".join(part.replace("~", "~0").replace("/", "~1") for part in parts) if parts else ""


def observation(path: str, value: Any) -> dict[str, Any]:
    count = len(value) if isinstance(value, (dict, list)) else 1
    return {"path": path, "count": count, "sha256": sha256_bytes(canonical_json(value))}


def marker_count(value: Any) -> int:
    if isinstance(value, str):
        return value.count(REDACTION_MARKER)
    if isinstance(value, dict):
        return sum(marker_count(child) for child in value.values())
    if isinstance(value, list):
        return sum(marker_count(child) for child in value)
    return 0


def contract_projection(value: Any) -> dict[str, Any]:
    fields: list[str] = []
    errors: list[dict[str, Any]] = []
    warnings: list[dict[str, Any]] = []
    redactions: dict[str, dict[str, Any]] = {}

    def walk(child: Any, parts: tuple[str, ...]) -> None:
        if isinstance(child, dict):
            severity = child.get("severity")
            if isinstance(severity, str):
                item = observation(pointer(parts), child)
                if severity.lower() == "error":
                    errors.append(item)
                elif severity.lower() in {"warn", "warning"}:
                    warnings.append(item)
            for key in sorted(child):
                item = child[key]
                item_parts = (*parts, str(key))
                path = pointer(item_parts)
                fields.append(path)
                lowered = str(key).lower()
                if lowered in ERROR_KEYS or lowered.endswith(("_error", "_errors")):
                    errors.append(observation(path, item))
                if lowered in WARNING_KEYS or lowered.endswith(("_warning", "_warnings")):
                    warnings.append(observation(path, item))
                markers = marker_count(item)
                if SENSITIVE_KEY.search(str(key)) or markers:
                    redactions[path] = {"path": path, "masked": markers > 0, "marker_count": markers}
                walk(item, item_parts)
        elif isinstance(child, list):
            for index, item in enumerate(child):
                walk(item, (*parts, str(index)))
        elif isinstance(child, str) and REDACTION_MARKER in child:
            path = pointer(parts)
            redactions.setdefault(
                path,
                {"path": path, "masked": True, "marker_count": child.count(REDACTION_MARKER)},
            )

    walk(value, ())
    return {
        "sha256": sha256_bytes(canonical_json(value)),
        "field_paths": sorted(fields),
        "errors": sorted(errors, key=lambda item: (item["path"], item["sha256"])),
        "warnings": sorted(warnings, key=lambda item: (item["path"], item["sha256"])),
        "redactions": [redactions[path] for path in sorted(redactions)],
    }


def summarize_go_test(raw: bytes) -> dict[str, Any]:
    elapsed: dict[str, float] = {}
    counts = {key: 0 for key in EVENT_COUNT_KEYS}
    for line_number, line in enumerate(raw.splitlines(), 1):
        if not line.strip():
            continue
        event = parse_json(line, f"go test line {line_number}")
        if not isinstance(event, dict):
            raise ValueError(f"invalid go test JSON at line {line_number}: expected object")
        action, package, test = event.get("Action"), event.get("Package"), event.get("Test")
        if action == "start" and package and not test:
            counts["packages_started"] += 1
        if action in {"pass", "fail", "skip"} and package and not test:
            counts["packages_finished"] += 1
            package_elapsed = event.get("Elapsed")
            if isinstance(package_elapsed, (int, float)) and not isinstance(package_elapsed, bool):
                package_elapsed = float(package_elapsed)
                if not math.isfinite(package_elapsed) or package_elapsed < 0:
                    raise ValueError(f"invalid go test JSON at line {line_number}: Elapsed")
                elapsed[str(package)] = package_elapsed
        if action == "run" and package and test:
            counts["tests_started"] += 1
        if action in {"pass", "fail", "skip"} and package and test:
            counts["tests_finished"] += 1
        if action == "output":
            counts["output_events"] += 1
    return {
        "package_elapsed_seconds": dict(sorted(elapsed.items())),
        "package_elapsed_sum_seconds": round(sum(elapsed.values()), 9),
        "package_elapsed_semantics": "overlapping package elapsed sum; never substitute for whole wall time",
        "event_counts": counts,
    }


def artifact_metadata(path: str, raw: bytes) -> dict[str, Any]:
    return {"path": path, "bytes": len(raw), "sha256": sha256_bytes(raw)}


def count_values(metadata: dict[str, Any], inputs: list[dict[str, Any]], stdout: bytes, stderr: bytes, projection: dict[str, Any], go_test: dict[str, Any] | None) -> dict[str, int]:
    events = go_test["event_counts"] if go_test else {key: 0 for key in EVENT_COUNT_KEYS}
    return {
        "command_invocations": metadata["execution"]["command_invocations"],
        "samples": metadata["execution"]["sample_count"],
        "inputs": len(inputs),
        "stdout_bytes": len(stdout),
        "stderr_bytes": len(stderr),
        "contract_fields": len(projection["field_paths"]),
        "contract_errors": len(projection["errors"]),
        "contract_warnings": len(projection["warnings"]),
        "contract_redactions": len(projection["redactions"]),
        **events,
    }


def run_record(args: argparse.Namespace) -> int:
    root = Path(args.workspace_root).resolve(strict=True)
    if not root.is_dir():
        raise ValueError("workspace root is not a directory")
    if not args.label.strip() or not args.series_id.strip() or args.sequence < 1:
        raise ValueError("label, series-id, and positive sequence are required")
    target = output_directory(root, args.output_dir)
    execution_path, execution_raw = read_regular(root, args.execution_file, "execution file")
    contract_path, contract_raw = read_regular(root, args.contract_file, "contract file")
    stdout_path, stdout = read_regular(root, args.stdout_file, "stdout file")
    stderr_path, stderr = read_regular(root, args.stderr_file, "stderr file")
    input_sources = [read_regular(root, value, "input file") for value in args.input_file]
    evidence = [
        ("execution file", execution_raw),
        ("contract file", contract_raw),
        ("stdout file", stdout),
        ("stderr file", stderr),
        *[("input file", raw) for _, raw in input_sources],
    ]
    scan_evidence(evidence)
    metadata = validate_execution(parse_json(execution_raw, "execution metadata"), root)
    contract = parse_json(contract_raw, "contract")
    projection = contract_projection(contract)
    go_test = summarize_go_test(stdout) if args.parser == "go-test-json" else None

    copied_inputs: list[dict[str, Any]] = []
    writes = {
        "execution.json": execution_raw,
        "contract.json": contract_raw,
        "stdout.log": stdout,
        "stderr.log": stderr,
    }
    for index, (source, raw) in enumerate(input_sources):
        artifact_path = f"inputs/{index:03d}-{source.name}"
        writes[artifact_path] = raw
        copied_inputs.append(
            {
                "source": source.relative_to(root).as_posix(),
                "artifact": artifact_metadata(artifact_path, raw),
            }
        )
    command = {**metadata["command"], "parser": args.parser}
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "measurement": {
            "label": args.label,
            "series_id": args.series_id,
            "variant": args.variant,
            "sequence": args.sequence,
            "revision": metadata["revision"],
            "recorded_at": datetime.now(timezone.utc).isoformat(),
        },
        "tool": metadata["tool"],
        "environment": metadata["environment"],
        "inputs": copied_inputs,
        "command": command,
        "counts": count_values(metadata, copied_inputs, stdout, stderr, projection, go_test),
        "execution": metadata["execution"],
        "go_test": go_test,
        "artifacts": {
            "execution": artifact_metadata("execution.json", execution_raw),
            "contract": artifact_metadata("contract.json", contract_raw),
            "stdout": artifact_metadata("stdout.log", stdout),
            "stderr": artifact_metadata("stderr.log", stderr),
        },
        "redaction_scan": {"scanner": "measure-efficiency-v1", "findings": 0, "files_checked": len(evidence)},
    }
    target.mkdir(parents=True)
    for relative, raw in writes.items():
        destination = target / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(raw)
    (target / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"ok": True, "manifest": (target / "manifest.json").relative_to(root).as_posix()}))
    return 0


def validate_artifact(metadata: Any, directory: Path, root: Path, label: str) -> bytes:
    artifact = require_object(metadata, f"invalid manifest {label} artifact")
    path = required(artifact, "path", str, f"manifest {label} artifact")
    size = nonnegative_integer(artifact, "bytes", f"manifest {label} artifact")
    digest = required(artifact, "sha256", str, f"manifest {label} artifact")
    if not re.fullmatch(r"[0-9a-f]{64}", digest):
        raise ValueError(f"invalid manifest: {label} artifact sha256")
    resolved, raw = read_regular(directory, path, f"{label} artifact")
    if not inside(resolved, root):
        raise ValueError(f"invalid manifest: {label} artifact outside workspace")
    if len(raw) != size:
        raise ValueError(f"{label} artifact size mismatch")
    if sha256_bytes(raw) != digest:
        raise ValueError(f"{label} artifact digest mismatch")
    return raw


def validate_go_test(value: Any, parser: str) -> dict[str, Any] | None:
    if parser == "none":
        if value is not None:
            raise ValueError("invalid manifest: go_test")
        return None
    summary = require_object(value, "manifest go_test")
    elapsed = required(summary, "package_elapsed_seconds", dict, "manifest go_test")
    if any(not isinstance(key, str) or isinstance(item, bool) or not isinstance(item, (int, float)) or not math.isfinite(item) or item < 0 for key, item in elapsed.items()):
        raise ValueError("invalid manifest: go_test package_elapsed_seconds")
    finite_number(summary, "package_elapsed_sum_seconds", "manifest go_test")
    required(summary, "package_elapsed_semantics", str, "manifest go_test")
    event_counts = require_object(required(summary, "event_counts", dict, "manifest go_test"), "manifest go_test event_counts")
    for key in EVENT_COUNT_KEYS:
        nonnegative_integer(event_counts, key, "manifest go_test event_counts")
    return summary


def read_manifest(root: Path, raw_path: str) -> dict[str, Any]:
    efficiency_root = (root / ".issueops-runtime" / "efficiency").resolve(strict=False)
    relative = relative_argument(raw_path, "manifest")
    candidate = (root / relative).resolve(strict=False)
    if not inside(efficiency_root, root) or not inside(candidate, efficiency_root):
        raise ValueError("invalid manifest: outside efficiency output root")
    manifest_path, manifest_raw = read_regular(root, raw_path, "manifest")
    manifest = require_object(parse_json(manifest_raw, "manifest"), "manifest")
    if required(manifest, "schema_version", int, "manifest") != SCHEMA_VERSION:
        raise ValueError("invalid manifest: schema_version")
    measurement = require_object(required(manifest, "measurement", dict, "manifest"), "manifest measurement")
    for key in ("label", "series_id", "variant", "revision", "recorded_at"):
        required(measurement, key, str, "manifest measurement")
    if measurement["variant"] not in {"baseline", "candidate"}:
        raise ValueError("invalid manifest: variant")
    positive_integer(measurement, "sequence", "manifest measurement")
    if not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", measurement["revision"]):
        raise ValueError("invalid manifest: revision")
    try:
        recorded = datetime.fromisoformat(measurement["recorded_at"])
    except ValueError as error:
        raise ValueError("invalid manifest: recorded_at") from error
    if recorded.tzinfo is None:
        raise ValueError("invalid manifest: recorded_at")

    tool = require_object(required(manifest, "tool", dict, "manifest"), "manifest tool")
    required(tool, "name", str, "manifest tool")
    required(tool, "version", str, "manifest tool")
    environment = validate_environment(manifest.get("environment"), "manifest environment")
    command = validate_command(manifest.get("command"), root, "manifest command", parser=True)
    execution = require_object(required(manifest, "execution", dict, "manifest"), "manifest execution")
    required(execution, "exit_code", int, "manifest execution")
    required(execution, "timed_out", bool, "manifest execution")
    finite_number(execution, "wall_time_seconds", "manifest execution")
    positive_integer(execution, "command_invocations", "manifest execution")
    positive_integer(execution, "sample_count", "manifest execution")
    counts = require_object(required(manifest, "counts", dict, "manifest"), "manifest counts")
    for key in COUNT_KEYS:
        nonnegative_integer(counts, key, "manifest counts")
    inputs = required(manifest, "inputs", list, "manifest")
    if not inputs:
        raise ValueError("invalid manifest: inputs")
    artifacts = require_object(required(manifest, "artifacts", dict, "manifest"), "manifest artifacts")
    directory = manifest_path.parent
    copied: dict[str, bytes] = {}
    for name in ("execution", "contract", "stdout", "stderr"):
        if name not in artifacts:
            raise ValueError(f"invalid manifest: missing {name} artifact")
        copied[name] = validate_artifact(artifacts[name], directory, root, name)
    input_bytes: list[bytes] = []
    for index, item in enumerate(inputs):
        entry = require_object(item, f"manifest input {index}")
        source = required(entry, "source", str, f"manifest input {index}")
        relative_argument(source, f"manifest input {index} source")
        if not inside((root / source).resolve(strict=False), root):
            raise ValueError(f"invalid manifest: input {index} source")
        if "artifact" not in entry:
            raise ValueError(f"invalid manifest: input {index} artifact")
        input_bytes.append(validate_artifact(entry["artifact"], directory, root, f"input {index}"))

    source_metadata = validate_execution(parse_json(copied["execution"], "execution artifact"), root, "execution artifact")
    expected_command = {**source_metadata["command"], "parser": command["parser"]}
    if measurement["revision"] != source_metadata["revision"] or tool != source_metadata["tool"] or environment != source_metadata["environment"] or command != expected_command or execution != source_metadata["execution"]:
        raise ValueError("invalid manifest: execution metadata does not match artifact")
    projection = contract_projection(parse_json(copied["contract"], "contract artifact"))
    parsed_go_test = summarize_go_test(copied["stdout"]) if command["parser"] == "go-test-json" else None
    stored_go_test = validate_go_test(manifest.get("go_test"), command["parser"])
    if stored_go_test != parsed_go_test:
        raise ValueError("invalid manifest: go_test does not match stdout artifact")
    expected_counts = count_values(source_metadata, inputs, copied["stdout"], copied["stderr"], projection, parsed_go_test)
    if counts != expected_counts:
        raise ValueError("invalid manifest: counts do not match artifacts")
    redaction = require_object(required(manifest, "redaction_scan", dict, "manifest"), "manifest redaction_scan")
    if required(redaction, "scanner", str, "manifest redaction_scan") != "measure-efficiency-v1" or nonnegative_integer(redaction, "findings", "manifest redaction_scan") != 0 or nonnegative_integer(redaction, "files_checked", "manifest redaction_scan") != 4 + len(inputs):
        raise ValueError("invalid manifest: redaction_scan")
    scan_evidence([(name, value) for name, value in copied.items()] + [("input file", value) for value in input_bytes])
    manifest["_projection"] = projection
    return manifest


def drift(kind: str, condition: str | None = None) -> dict[str, str]:
    result = {"kind": kind}
    if condition:
        result["condition"] = condition
    return result


def run_compare(args: argparse.Namespace) -> int:
    root = Path(args.workspace_root).resolve(strict=True)
    if not root.is_dir():
        raise ValueError("workspace root is not a directory")
    baseline = read_manifest(root, args.baseline)
    candidate = read_manifest(root, args.candidate)
    b_measurement, c_measurement = baseline["measurement"], candidate["measurement"]
    measurement_checks = {
        "series": b_measurement["series_id"] == c_measurement["series_id"],
        "tool": baseline["tool"] == candidate["tool"],
        "environment": baseline["environment"] == candidate["environment"],
        "fixed_inputs": baseline["inputs"] == candidate["inputs"],
        "command": baseline["command"] == candidate["command"],
        "command_invocations": baseline["counts"]["command_invocations"] == candidate["counts"]["command_invocations"],
        "samples": baseline["counts"]["samples"] == candidate["counts"]["samples"],
        "packages_started": baseline["counts"]["packages_started"] == candidate["counts"]["packages_started"],
        "packages_finished": baseline["counts"]["packages_finished"] == candidate["counts"]["packages_finished"],
        "tests_started": baseline["counts"]["tests_started"] == candidate["counts"]["tests_started"],
        "tests_finished": baseline["counts"]["tests_finished"] == candidate["counts"]["tests_finished"],
        "alternating_sequence": b_measurement["variant"] == "baseline" and c_measurement["variant"] == "candidate" and c_measurement["sequence"] == b_measurement["sequence"] + 1,
        "successful_execution": all(item["execution"]["exit_code"] == 0 and not item["execution"]["timed_out"] for item in (baseline, candidate)),
    }
    measurement_drifts = [drift("measurement", name) for name, passed in measurement_checks.items() if not passed]
    left, right = baseline["_projection"], candidate["_projection"]
    contract_checks = {
        "output": left["sha256"] == right["sha256"],
        "field": left["field_paths"] == right["field_paths"],
        "error": left["errors"] == right["errors"],
        "warning": left["warnings"] == right["warnings"],
        "redaction": left["redactions"] == right["redactions"],
    }
    contract_drifts = [drift(name) for name, passed in contract_checks.items() if not passed]
    comparable = not measurement_drifts
    contract_equal = not contract_drifts
    drifts = measurement_drifts + contract_drifts
    result = {
        "ok": comparable and contract_equal,
        "comparable": comparable,
        "contract_equal": contract_equal,
        "drifts": drifts,
        "timing": {
            "baseline": {
                "wall_time_seconds": baseline["execution"]["wall_time_seconds"],
                "package_elapsed_sum_seconds": baseline["go_test"]["package_elapsed_sum_seconds"] if baseline["go_test"] else None,
            },
            "candidate": {
                "wall_time_seconds": candidate["execution"]["wall_time_seconds"],
                "package_elapsed_sum_seconds": candidate["go_test"]["package_elapsed_sum_seconds"] if candidate["go_test"] else None,
            },
        },
    }
    print(json.dumps(result, sort_keys=True))
    return 0 if result["ok"] else 1


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description="Record and compare fixed-input efficiency evidence")
    commands = root.add_subparsers(dest="action", required=True)
    record = commands.add_parser("record")
    record.add_argument("--workspace-root", required=True)
    record.add_argument("--output-dir", required=True)
    record.add_argument("--label", required=True)
    record.add_argument("--series-id", required=True)
    record.add_argument("--variant", choices=("baseline", "candidate"), required=True)
    record.add_argument("--sequence", type=int, required=True)
    record.add_argument("--execution-file", required=True)
    record.add_argument("--parser", choices=("none", "go-test-json"), required=True)
    record.add_argument("--contract-file", required=True)
    record.add_argument("--input-file", action="append", required=True)
    record.add_argument("--stdout-file", required=True)
    record.add_argument("--stderr-file", required=True)
    compare = commands.add_parser("compare")
    compare.add_argument("--workspace-root", required=True)
    compare.add_argument("--baseline", required=True)
    compare.add_argument("--candidate", required=True)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        return run_record(args) if args.action == "record" else run_compare(args)
    except (OSError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
