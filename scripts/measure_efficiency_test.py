#!/usr/bin/env python3

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("measure_efficiency.py")


class MeasureEfficiencyTest(unittest.TestCase):
    def test_record_preserves_raw_evidence_and_separates_package_from_wall_time(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            contract = root / "contract.json"
            contract.write_text(
                json.dumps(
                    {
                        "ok": True,
                        "errors": [],
                        "warnings": [],
                        "access_token": "<redacted>",
                    }
                ),
                encoding="utf-8",
            )
            input_path = root / "input.json"
            input_path.write_text('{"cycles":1}\n', encoding="utf-8")
            output_dir = root / ".issueops-runtime" / "efficiency" / "run-001-baseline"
            program = textwrap.dedent(
                """
                import json
                import sys

                events = [
                    {"Action": "start", "Package": "example/one"},
                    {"Action": "start", "Package": "example/two"},
                    {"Action": "run", "Package": "example/one", "Test": "TestOne"},
                    {"Action": "pass", "Package": "example/one", "Test": "TestOne", "Elapsed": 0.01},
                    {"Action": "pass", "Package": "example/one", "Elapsed": 1.5},
                    {"Action": "pass", "Package": "example/two", "Elapsed": 1.75},
                ]
                for event in events:
                    print(json.dumps(event))
                print("fixture stderr", file=sys.stderr)
                """
            )

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "record",
                    "--workspace-root",
                    str(root),
                    "--output-dir",
                    str(output_dir),
                    "--label",
                    "same revision baseline",
                    "--series-id",
                    "same-revision",
                    "--variant",
                    "baseline",
                    "--sequence",
                    "1",
                    "--revision",
                    "a" * 40,
                    "--tool-name",
                    "fixture-go",
                    "--tool-version",
                    "go version go1.fixture test/arch",
                    "--parser",
                    "go-test-json",
                    "--contract-file",
                    str(contract),
                    "--input-file",
                    str(input_path),
                    "--timeout-seconds",
                    "10",
                    "--",
                    sys.executable,
                    "-c",
                    program,
                ],
                cwd=root,
                text=True,
                capture_output=True,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            status = json.loads(result.stdout)
            self.assertTrue(status["ok"])
            self.assertEqual(Path(status["manifest"]), (output_dir / "manifest.json").resolve())
            manifest = json.loads((output_dir / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(manifest["schema_version"], 1)
            self.assertEqual(manifest["measurement"]["revision"], "a" * 40)
            self.assertEqual(manifest["measurement"]["sequence"], 1)
            self.assertEqual(manifest["tool"]["version"], "go version go1.fixture test/arch")
            self.assertEqual(manifest["command"]["argv"], [sys.executable, "-c", program])
            self.assertEqual(manifest["counts"]["runs"], 1)
            self.assertEqual(manifest["counts"]["inputs"], 1)
            self.assertEqual(manifest["counts"]["packages_started"], 2)
            self.assertEqual(manifest["counts"]["packages_finished"], 2)
            self.assertEqual(manifest["counts"]["tests_started"], 1)
            self.assertEqual(manifest["counts"]["tests_finished"], 1)
            self.assertEqual(manifest["counts"]["contract_field_paths"], 4)
            self.assertEqual(manifest["counts"]["error_observations"], 1)
            self.assertEqual(manifest["counts"]["warning_observations"], 1)
            self.assertEqual(manifest["counts"]["redaction_observations"], 1)
            self.assertEqual(manifest["go_test"]["package_elapsed_sum_seconds"], 3.25)
            self.assertEqual(
                manifest["go_test"]["package_elapsed_semantics"],
                "overlapping package elapsed sum; never substitute for whole wall time",
            )
            self.assertLess(manifest["execution"]["wall_time_seconds"], 3.25)
            self.assertEqual(manifest["execution"]["exit_code"], 0)
            self.assertFalse(manifest["execution"]["timed_out"])
            self.assertIn("system", manifest["environment"])
            self.assertIn("machine", manifest["environment"])
            raw_stdout = (output_dir / manifest["artifacts"]["stdout"]).read_text(encoding="utf-8")
            self.assertEqual(len(raw_stdout.splitlines()), 6)
            self.assertEqual(json.loads(raw_stdout.splitlines()[-1])["Package"], "example/two")
            self.assertEqual(
                (output_dir / manifest["artifacts"]["stderr"]).read_text(encoding="utf-8"),
                "fixture stderr\n",
            )
            self.assertEqual(
                json.loads((output_dir / manifest["artifacts"]["contract"]).read_text(encoding="utf-8")),
                json.loads(contract.read_text(encoding="utf-8")),
            )

    def record_fixture(
        self,
        root: Path,
        output_name: str,
        variant: str,
        sequence: int,
        contract_value: object,
    ) -> Path:
        contract = root / f"{output_name}-contract.json"
        contract.write_text(json.dumps(contract_value), encoding="utf-8")
        input_path = root / "fixed-input.json"
        if not input_path.exists():
            input_path.write_text('{"cycles":100}\n', encoding="utf-8")
        output_dir = root / ".issueops-runtime" / "efficiency" / output_name
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "record",
                "--workspace-root",
                str(root),
                "--output-dir",
                str(output_dir),
                "--label",
                output_name,
                "--series-id",
                "comparison-series",
                "--variant",
                variant,
                "--sequence",
                str(sequence),
                "--revision",
                "b" * 40,
                "--tool-name",
                "fixture-tool",
                "--tool-version",
                "fixture-tool 1.0",
                "--contract-file",
                str(contract),
                "--input-file",
                str(input_path),
                "--timeout-seconds",
                "10",
                "--",
                sys.executable,
                "-c",
                "print('fixed raw output')",
            ],
            cwd=root,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return output_dir / "manifest.json"

    def compare(self, root: Path, baseline: Path, candidate: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "compare",
                "--baseline",
                str(baseline),
                "--candidate",
                str(candidate),
            ],
            cwd=root,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_compare_accepts_identical_revision_contract_despite_duration_noise(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            contract = {
                "ok": True,
                "errors": [],
                "warnings": [],
                "access_token": "<redacted>",
            }
            baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1, contract)
            candidate = self.record_fixture(root, "run-002-candidate", "candidate", 2, contract)

            result = self.compare(root, baseline, candidate)

            self.assertEqual(result.returncode, 0, result.stderr)
            comparison = json.loads(result.stdout)
            self.assertTrue(comparison["ok"])
            self.assertTrue(comparison["comparable"])
            self.assertTrue(comparison["contract_equal"])
            self.assertEqual(comparison["drifts"], [])
            self.assertEqual(comparison["baseline_revision"], "b" * 40)
            self.assertEqual(comparison["candidate_revision"], "b" * 40)
            self.assertIn("wall_time_seconds", comparison["timing"]["baseline"])
            self.assertIn("wall_time_seconds", comparison["timing"]["candidate"])

    def test_compare_rejects_field_error_warning_and_redaction_drift(self) -> None:
        baseline_contract = {
            "ok": True,
            "errors": [],
            "warnings": [],
            "access_token": "<redacted>",
        }
        cases = {
            "field": {**baseline_contract, "new_field": 1},
            "error": {**baseline_contract, "errors": [{"code": "fixture_failure"}]},
            "warning": {**baseline_contract, "warnings": ["fixture_warning"]},
            "redaction": {**baseline_contract, "access_token": "fixture-visible-value"},
        }
        for expected_kind, candidate_contract in cases.items():
            with self.subTest(kind=expected_kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1, baseline_contract)
                candidate = self.record_fixture(root, "run-002-candidate", "candidate", 2, candidate_contract)

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1, result.stdout)
                comparison = json.loads(result.stdout)
                self.assertFalse(comparison["ok"])
                self.assertTrue(comparison["comparable"])
                self.assertFalse(comparison["contract_equal"])
                kinds = {drift["kind"] for drift in comparison["drifts"]}
                self.assertIn("output", kinds)
                self.assertIn(expected_kind, kinds)

    def test_compare_rejects_changed_inputs_and_non_alternating_runs(self) -> None:
        contract = {"ok": True, "errors": [], "warnings": []}
        cases = ("fixed_inputs", "alternating_sequence")
        for condition in cases:
            with self.subTest(condition=condition), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                baseline = self.record_fixture(root, "run-001-baseline", "baseline", 1, contract)
                if condition == "fixed_inputs":
                    (root / "fixed-input.json").write_text('{"cycles":1000}\n', encoding="utf-8")
                    candidate_sequence = 2
                else:
                    candidate_sequence = 3
                candidate = self.record_fixture(
                    root,
                    "run-002-candidate",
                    "candidate",
                    candidate_sequence,
                    contract,
                )

                result = self.compare(root, baseline, candidate)

                self.assertEqual(result.returncode, 1, result.stdout)
                comparison = json.loads(result.stdout)
                self.assertFalse(comparison["ok"])
                self.assertFalse(comparison["comparable"])
                self.assertTrue(comparison["contract_equal"])
                conditions = {
                    drift.get("condition")
                    for drift in comparison["drifts"]
                    if drift["kind"] == "measurement"
                }
                self.assertIn(condition, conditions)


if __name__ == "__main__":
    unittest.main()
