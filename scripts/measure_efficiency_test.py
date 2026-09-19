#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("measure_efficiency.py")


class MeasureEfficiencyTest(unittest.TestCase):
    def write_json(self, path: Path, value: object) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value) + "\n", encoding="utf-8")

    def base_contract(self) -> dict[str, object]:
        return {
            "ok": True,
            "errors": [],
            "warnings": [],
            "access_token": "<redacted>",
            "diagnostics": [{"severity": "info", "message": "fixture diagnostic"}],
        }

    def execution_metadata(
        self,
        *,
        sample_count: int = 20,
        wall_time_seconds: float = 0.001,
        machine: str = "fixture-arm64",
        revision: str = "b" * 40,
    ) -> dict[str, object]:
        return {
            "revision": revision,
            "tool": {"name": "fixture-go", "version": "go version go1.fixture test/arch"},
            "environment": {
                "system": "FixtureOS",
                "release": "1.0",
                "machine": machine,
                "processor": "fixture-cpu",
                "cpu_count": 8,
                "variables": {
                    "PATH_SHA256": hashlib.sha256(b"/fixture/bin").hexdigest(),
                    "GOFLAGS": "<unset>",
                    "GOWORK": "<unset>",
                    "GOENV": "<unset>",
                    "CGO_ENABLED": "0",
                    "GOMAXPROCS": "<unset>",
                    "GOGC": "<unset>",
                    "GOMEMLIMIT": "<unset>",
                    "GOEXPERIMENT": "<unset>",
                    "GODEBUG": "<unset>",
                    "CC": "<unset>",
                    "CXX": "<unset>",
                    "LANG": "C.UTF-8",
                    "LC_ALL": "<unset>",
                    "LC_CTYPE": "<unset>",
                },
            },
            "command": {
                "argv": ["/definitely/not/go", "test", "-json", f"-count={sample_count}", "./..."],
                "cwd": ".",
            },
            "execution": {
                "exit_code": 0,
                "timed_out": False,
                "wall_time_seconds": wall_time_seconds,
                "command_invocations": 1,
                "sample_count": sample_count,
            },
        }

    def go_test_output(self, elapsed: float = 1_000_000.0) -> str:
        events = [
            {"Action": "start", "Package": "example/one"},
            {"Action": "start", "Package": "example/two"},
            {"Action": "run", "Package": "example/one", "Test": "TestOne"},
            {"Action": "pass", "Package": "example/one", "Test": "TestOne", "Elapsed": 0.01},
            {"Action": "pass", "Package": "example/one", "Elapsed": elapsed},
            {"Action": "pass", "Package": "example/two", "Elapsed": elapsed},
        ]
        return "".join(json.dumps(event) + "\n" for event in events)

    def prepare_record(
        self,
        root: Path,
        output_name: str,
        variant: str,
        sequence: int,
        *,
        contract: object | None = None,
        execution: object | None = None,
        stdout: str | None = None,
        stderr: str = "Authorization: Bearer <redacted>\n",
    ) -> list[str]:
        fixtures = root / "fixtures"
        fixtures.mkdir(exist_ok=True)
        fixed_input = fixtures / "fixed-input.json"
        if not fixed_input.exists():
            fixed_input.write_text('{"cycles":100}\n', encoding="utf-8")
        contract_path = fixtures / f"{output_name}-contract.json"
        execution_path = fixtures / f"{output_name}-execution.json"
        stdout_path = fixtures / f"{output_name}-stdout.jsonl"
        stderr_path = fixtures / f"{output_name}-stderr.log"
        self.write_json(contract_path, self.base_contract() if contract is None else contract)
        self.write_json(
            execution_path,
            self.execution_metadata() if execution is None else execution,
        )
        stdout_path.write_text(self.go_test_output() if stdout is None else stdout, encoding="utf-8")
        stderr_path.write_text(stderr, encoding="utf-8")
        return [
            sys.executable,
            str(SCRIPT),
            "record",
            "--workspace-root",
            str(root),
            "--output-dir",
            f".issueops-runtime/efficiency/{output_name}",
            "--label",
            output_name,
            "--series-id",
            "comparison-series",
            "--variant",
            variant,
            "--sequence",
            str(sequence),
            "--execution-file",
            execution_path.relative_to(root).as_posix(),
            "--parser",
            "go-test-json",
            "--contract-file",
            contract_path.relative_to(root).as_posix(),
            "--input-file",
            fixed_input.relative_to(root).as_posix(),
            "--stdout-file",
            stdout_path.relative_to(root).as_posix(),
            "--stderr-file",
            stderr_path.relative_to(root).as_posix(),
        ]

    def record_fixture(
        self,
        root: Path,
        output_name: str,
        variant: str,
        sequence: int,
        **kwargs: object,
    ) -> Path:
        result = subprocess.run(
            self.prepare_record(root, output_name, variant, sequence, **kwargs),
            cwd=root,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        status = json.loads(result.stdout)
        self.assertTrue(status["ok"])
        manifest = Path(status["manifest"])
        self.assertFalse(manifest.is_absolute())
        return manifest

    def compare(self, root: Path, baseline: Path, candidate: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "compare",
                "--workspace-root",
                str(root),
                "--baseline",
                baseline.as_posix(),
                "--candidate",
                candidate.as_posix(),
            ],
            cwd=root,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_record_copies_validated_evidence_without_executing_ambient_command(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            manifest_path = self.record_fixture(root, "run-001-baseline", "baseline", 1)

            manifest = json.loads((root / manifest_path).read_text(encoding="utf-8"))
            self.assertEqual(manifest["schema_version"], 2)
            self.assertEqual(manifest["measurement"]["revision"], "b" * 40)
            self.assertEqual(manifest["tool"]["version"], "go version go1.fixture test/arch")
            self.assertEqual(manifest["command"]["argv"][0], "/definitely/not/go")
            self.assertEqual(manifest["counts"]["command_invocations"], 1)
            self.assertEqual(manifest["counts"]["samples"], 20)
            self.assertEqual(manifest["counts"]["packages_started"], 2)
            self.assertEqual(manifest["counts"]["packages_finished"], 2)
            self.assertEqual(manifest["counts"]["tests_started"], 1)
            self.assertEqual(manifest["counts"]["tests_finished"], 1)
            self.assertEqual(manifest["go_test"]["package_elapsed_sum_seconds"], 2_000_000.0)
            self.assertEqual(manifest["execution"]["wall_time_seconds"], 0.001)
            self.assertNotEqual(
                manifest["go_test"]["package_elapsed_sum_seconds"],
                manifest["execution"]["wall_time_seconds"],
            )
            for name in ("execution", "stdout", "stderr", "contract"):
                artifact = manifest["artifacts"][name]
                content = (root / manifest_path.parent / artifact["path"]).read_bytes()
                self.assertEqual(artifact["bytes"], len(content))
                self.assertEqual(artifact["sha256"], hashlib.sha256(content).hexdigest())

    def test_compare_accepts_cross_revision_contract_despite_duration_noise(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
            candidate = self.record_fixture(
                root,
                "run-002-candidate",
                "candidate",
                2,
                execution=self.execution_metadata(wall_time_seconds=9.5, revision="c" * 40),
            )

            result = self.compare(root, baseline, candidate)

            self.assertEqual(result.returncode, 0, result.stderr)
            comparison = json.loads(result.stdout)
            self.assertTrue(comparison["ok"])
            self.assertTrue(comparison["comparable"])
            self.assertTrue(comparison["contract_equal"])
            self.assertEqual(comparison["drifts"], [])

    def test_compare_accepts_same_revision_contract_despite_duration_noise(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
            candidate = self.record_fixture(
                root,
                "run-002-candidate",
                "candidate",
                2,
                execution=self.execution_metadata(wall_time_seconds=9.5),
            )

            result = self.compare(root, baseline, candidate)

            self.assertEqual(result.returncode, 0, result.stderr)
            comparison = json.loads(result.stdout)
            self.assertTrue(comparison["ok"])
            self.assertTrue(comparison["comparable"])
            self.assertTrue(comparison["contract_equal"])
            self.assertEqual(comparison["drifts"], [])

    def test_compare_rejects_field_error_warning_and_redaction_drift(self) -> None:
        baseline_contract = self.base_contract()
        cases = {
            "field": {**baseline_contract, "new_field": 1},
            "error": {
                **baseline_contract,
                "diagnostics": [{"severity": "error", "message": "fixture diagnostic"}],
            },
            "warning": {**baseline_contract, "warnings": ["fixture_warning"]},
            "redaction": {key: value for key, value in baseline_contract.items() if key != "access_token"},
        }
        for expected_kind, candidate_contract in cases.items():
            with self.subTest(kind=expected_kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(
                    root,
                    "run-001-baseline",
                    "baseline",
                    1,
                    contract=baseline_contract,
                )
                candidate = self.record_fixture(
                    root,
                    "run-002-candidate",
                    "candidate",
                    2,
                    contract=candidate_contract,
                )

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1, result.stdout)
                comparison = json.loads(result.stdout)
                self.assertTrue(comparison["comparable"])
                kinds = {drift["kind"] for drift in comparison["drifts"]}
                self.assertIn("output", kinds)
                self.assertIn(expected_kind, kinds)

    def test_compare_rejects_missing_or_malformed_required_fields_on_both_sides(self) -> None:
        mutations = {
            "measurement.revision": lambda value: value["measurement"].pop("revision"),
            "tool.version": lambda value: value["tool"].pop("version"),
            "environment.machine": lambda value: value["environment"].pop("machine"),
            "inputs": lambda value: value.pop("inputs"),
            "command.argv": lambda value: value["command"].pop("argv"),
            "counts.samples": lambda value: value["counts"].pop("samples"),
            "artifacts.stdout.sha256": lambda value: value["artifacts"]["stdout"].pop("sha256"),
            "sequence_type": lambda value: value["measurement"].update({"sequence": "1"}),
        }
        for field, mutate in mutations.items():
            with self.subTest(field=field), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
                candidate = self.record_fixture(root, "run-002-candidate", "candidate", 2)
                for original in (baseline, candidate):
                    value = json.loads((root / original).read_text(encoding="utf-8"))
                    mutate(value)
                    alternate = original.parent / f"malformed-{original.name}"
                    self.write_json(root / alternate, value)
                    if original == baseline:
                        bad_baseline = alternate
                    else:
                        bad_candidate = alternate

                result = self.compare(root, bad_baseline, bad_candidate)

                self.assertEqual(result.returncode, 1)
                self.assertIn("invalid manifest", result.stderr)

    def test_compare_validates_stdout_and_stderr_artifact_integrity(self) -> None:
        cases = ((name, action) for name in ("stdout", "stderr") for action in ("tampered", "missing"))
        for name, action in cases:
            with self.subTest(artifact=name, action=action), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
                candidate = self.record_fixture(root, "run-002-candidate", "candidate", 2)
                manifest = json.loads((root / candidate).read_text(encoding="utf-8"))
                artifact = root / candidate.parent / manifest["artifacts"][name]["path"]
                if action == "tampered":
                    artifact.write_text("changed\n", encoding="utf-8")
                else:
                    artifact.unlink()

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1)
                self.assertIn(f"{name} artifact", result.stderr)

    def test_record_rejects_outside_fifo_and_symlink_escape_sources(self) -> None:
        with tempfile.TemporaryDirectory() as temp, tempfile.TemporaryDirectory() as outside_temp:
            root = Path(temp)
            outside = Path(outside_temp) / "outside.json"
            outside.write_text("{}\n", encoding="utf-8")
            base_args = self.prepare_record(root, "run-001-baseline", "baseline", 1)
            contract_index = base_args.index("--contract-file") + 1
            stdout_index = base_args.index("--stdout-file") + 1

            fifo = root / "fixtures" / "stdout.pipe"
            os.mkfifo(fifo)
            escape = root / "fixtures" / "escape.json"
            escape.symlink_to(outside)
            cases = {
                "absolute": (contract_index, str(outside)),
                "outside": (contract_index, "../outside.json"),
                "fifo": (stdout_index, fifo.relative_to(root).as_posix()),
                "symlink_escape": (contract_index, escape.relative_to(root).as_posix()),
            }
            for name, (index, replacement) in cases.items():
                with self.subTest(name=name):
                    args = list(base_args)
                    args[index] = replacement
                    result = subprocess.run(
                        args,
                        cwd=root,
                        text=True,
                        capture_output=True,
                        timeout=3,
                        check=False,
                    )
                    self.assertEqual(result.returncode, 1)
                    self.assertRegex(result.stderr, r"workspace-relative|outside workspace|regular file")

    def test_record_accepts_only_exact_redaction_placeholders(self) -> None:
        unsafe_values = (
            "super-secret-value",
            "prefix<redacted>",
            "<redacted>-suffix",
            "Bearer <redacted> suffix",
            '" <redacted> "',
        )
        for value in unsafe_values:
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                args = self.prepare_record(
                    root,
                    "run-001-baseline",
                    "baseline",
                    1,
                    stdout=f"access_token={value}\n",
                )

                result = subprocess.run(
                    args,
                    cwd=root,
                    text=True,
                    capture_output=True,
                    check=False,
                )

                self.assertEqual(result.returncode, 1)
                self.assertIn("unredacted secret-like material", result.stderr)
                self.assertFalse((root / ".issueops-runtime" / "efficiency" / "run-001-baseline").exists())

    def test_record_requires_fixed_relevant_environment_keys(self) -> None:
        mutations = {
            "empty": lambda variables: variables.clear(),
            "missing": lambda variables: variables.pop("GOFLAGS"),
            "missing_gomaxprocs": lambda variables: variables.pop("GOMAXPROCS"),
            "missing_gogc": lambda variables: variables.pop("GOGC"),
            "missing_gomemlimit": lambda variables: variables.pop("GOMEMLIMIT"),
            "missing_goexperiment": lambda variables: variables.pop("GOEXPERIMENT"),
            "missing_godebug": lambda variables: variables.pop("GODEBUG"),
            "extra": lambda variables: variables.update({"ARBITRARY": "value"}),
            "raw_path": lambda variables: variables.update({"PATH_SHA256": "/fixture/bin"}),
            "empty_value": lambda variables: variables.update({"CC": ""}),
        }
        for name, mutate in mutations.items():
            with self.subTest(case=name), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                execution = self.execution_metadata()
                mutate(execution["environment"]["variables"])

                result = subprocess.run(
                    self.prepare_record(root, "run-001-baseline", "baseline", 1, execution=execution),
                    cwd=root,
                    text=True,
                    capture_output=True,
                    check=False,
                )

                self.assertEqual(result.returncode, 1)
                self.assertIn("environment variables", result.stderr)

    def test_record_rejects_go_count_sample_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            execution = self.execution_metadata(sample_count=20)
            execution["execution"]["sample_count"] = 19
            args = self.prepare_record(
                root,
                "run-001-baseline",
                "baseline",
                1,
                execution=execution,
            )

            result = subprocess.run(args, cwd=root, text=True, capture_output=True, check=False)

            self.assertEqual(result.returncode, 1)
            self.assertIn("sample_count does not match go test -count", result.stderr)

    def test_record_derives_go_count_from_argv_and_goflags(self) -> None:
        cases = (
            ("argv_only", "<unset>", True, None),
            ("goflags_only", "-count=20", False, None),
            ("matching_duplicate", "-count 20", True, None),
            ("conflict", "-count=19", True, "conflicting go test -count"),
            ("ambiguous", "-count", False, "go test -count has no value"),
        )
        for name, goflags, keep_argv_count, expected_error in cases:
            with self.subTest(case=name), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                execution = self.execution_metadata()
                execution["environment"]["variables"]["GOFLAGS"] = goflags
                if name == "argv_only":
                    index = execution["command"]["argv"].index("-count=20")
                    execution["command"]["argv"][index : index + 1] = ["-count", "20"]
                elif not keep_argv_count:
                    execution["command"]["argv"].remove("-count=20")

                result = subprocess.run(
                    self.prepare_record(root, "run-001-baseline", "baseline", 1, execution=execution),
                    cwd=root,
                    text=True,
                    capture_output=True,
                    check=False,
                )

                if expected_error is None:
                    self.assertEqual(result.returncode, 0, result.stderr)
                else:
                    self.assertEqual(result.returncode, 1)
                    self.assertIn(expected_error, result.stderr)

    def test_compare_rejects_changed_inputs_environment_and_non_alternating_runs(self) -> None:
        cases = ("fixed_inputs", "environment", "alternating_sequence")
        for condition in cases:
            with self.subTest(condition=condition), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
                sequence = 2
                execution = self.execution_metadata()
                if condition == "fixed_inputs":
                    (root / "fixtures" / "fixed-input.json").write_text('{"cycles":1000}\n', encoding="utf-8")
                elif condition == "environment":
                    execution = self.execution_metadata(machine="other-machine")
                else:
                    sequence = 3
                candidate = self.record_fixture(
                    root,
                    "run-002-candidate",
                    "candidate",
                    sequence,
                    execution=execution,
                )

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1, result.stdout)
                comparison = json.loads(result.stdout)
                self.assertFalse(comparison["comparable"])
                conditions = {
                    drift.get("condition")
                    for drift in comparison["drifts"]
                    if drift["kind"] == "measurement"
                }
                self.assertIn(condition, conditions)

    def test_compare_rejects_go_runtime_environment_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
            execution = self.execution_metadata()
            execution["environment"]["variables"]["GOMAXPROCS"] = "1"
            candidate = self.record_fixture(
                root,
                "run-002-candidate",
                "candidate",
                2,
                execution=execution,
            )

            result = self.compare(root, baseline, candidate)

            self.assertEqual(result.returncode, 1, result.stdout)
            comparison = json.loads(result.stdout)
            self.assertFalse(comparison["comparable"])
            conditions = {
                item.get("condition")
                for item in comparison["drifts"]
                if item["kind"] == "measurement"
            }
            self.assertIn("environment", conditions)

    def test_compare_reports_contract_drift_when_measurements_are_not_comparable(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1)
            candidate = self.record_fixture(
                root,
                "run-003-candidate",
                "candidate",
                3,
                contract={**self.base_contract(), "new_field": True},
            )

            result = self.compare(root, baseline, candidate)

            self.assertEqual(result.returncode, 1, result.stderr)
            comparison = json.loads(result.stdout)
            self.assertFalse(comparison["comparable"])
            self.assertFalse(comparison["contract_equal"])
            kinds = {item["kind"] for item in comparison["drifts"]}
            self.assertIn("measurement", kinds)
            self.assertIn("output", kinds)
            self.assertIn("field", kinds)

    def test_compare_rejects_execution_and_coverage_count_reductions(self) -> None:
        lines = self.go_test_output().splitlines(keepends=True)
        cases = {
            "command_invocations": ({}, None),
            "samples": ({"sample_count": 19}, None),
            "packages_started": ({}, "".join(lines[1:])),
            "packages_finished": ({}, "".join(lines[:4] + lines[5:])),
            "tests_started": ({}, "".join(lines[:2] + lines[3:])),
            "tests_finished": ({}, "".join(lines[:3] + lines[4:])),
        }
        for condition, (change, stdout) in cases.items():
            with self.subTest(condition=condition), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline_execution = self.execution_metadata()
                if condition == "command_invocations":
                    baseline_execution["execution"]["command_invocations"] = 2
                baseline = self.record_fixture(
                    root,
                    "run-001-baseline",
                    "baseline",
                    1,
                    execution=baseline_execution,
                )
                execution = self.execution_metadata(sample_count=change.get("sample_count", 20))
                execution["execution"].update(change.get("execution", {}))
                candidate = self.record_fixture(
                    root,
                    "run-002-candidate",
                    "candidate",
                    2,
                    execution=execution,
                    stdout=stdout,
                )

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1, result.stdout)
                comparison = json.loads(result.stdout)
                self.assertFalse(comparison["comparable"])
                self.assertTrue(comparison["contract_equal"])
                conditions = {
                    item.get("condition")
                    for item in comparison["drifts"]
                    if item["kind"] == "measurement"
                }
                self.assertIn(condition, conditions)

    def test_compare_rejects_manifest_outside_efficiency_root_before_reading(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            candidate = self.record_fixture(root, "run-002-candidate", "candidate", 2)
            outside = root / "fixtures" / "outside-manifest.pipe"
            os.mkfifo(outside)

            result = self.compare(root, outside.relative_to(root), candidate)

            self.assertEqual(result.returncode, 1)
            self.assertIn("outside efficiency output root", result.stderr)


if __name__ == "__main__":
    unittest.main()
