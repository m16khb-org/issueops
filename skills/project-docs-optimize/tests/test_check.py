from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import cast

SKILL_ROOT = Path(__file__).parents[1]


class DocumentationCheckTest(unittest.TestCase):
    def test_valid_document_family_passes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = self._payload(result)
            self.assertTrue(payload["ok"])
            self.assertEqual(payload["violations"], [])

    def test_oversized_module_and_broken_link_fail(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=True)

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            raw_violations = self._payload(result)["violations"]
            self.assertIsInstance(raw_violations, list)
            violations = cast("list[dict[str, object]]", raw_violations)
            codes = {item["code"] for item in violations}
            self.assertIn("line_budget_exceeded", codes)
            self.assertIn("broken_link", codes)

    def test_nested_dated_record_fails(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)
            nested = root / ".issueops" / "testing" / "lessons"
            nested.mkdir()
            _ = (nested / "2026-09-01-flaky-fixture.md").write_text(
                "# Flaky fixture\n\n[Testing index](../../TESTING.md)\n",
                encoding="utf-8",
            )

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            self.assertIn(
                ("nested_record", ".issueops/testing/lessons/2026-09-01-flaky-fixture.md"),
                self._violations(result),
            )

    def test_undeclared_directory_and_root_document_fail(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)
            docs = root / ".issueops"
            (docs / "scratch").mkdir()
            _ = (docs / "scratch" / "notes.md").write_text("# Notes\n", encoding="utf-8")
            _ = (docs / "NOTES.md").write_text("# Notes\n", encoding="utf-8")

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            violations = self._violations(result)
            self.assertIn(("undeclared_directory", ".issueops/scratch"), violations)
            self.assertIn(("undeclared_root_document", ".issueops/NOTES.md"), violations)

    def test_declared_and_runtime_directories_pass(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(
                root,
                broken=False,
                directories={".issueops/plans": "historical implementation plans"},
                root_documents=[".issueops/GUIDE.md"],
            )
            docs = root / ".issueops"
            for name in ("plans", "issues/12", "gates", "verified-execution"):
                (docs / name).mkdir(parents=True)
                _ = (docs / name / "record.md").write_text("# Record\n", encoding="utf-8")
            _ = (docs / "GUIDE.md").write_text("# Guide\n", encoding="utf-8")
            _ = (docs / "DESIGN.md").write_text("# Design\n", encoding="utf-8")

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 0, result.stdout)

    def test_gitignored_documents_are_not_checked(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)
            _ = subprocess.run(["git", "init", "-q", str(root)], check=True)
            _ = (root / ".gitignore").write_text(".issueops/evidence/\n", encoding="utf-8")
            evidence = root / ".issueops" / "evidence"
            evidence.mkdir()
            _ = (evidence / "run.md").write_text("[Missing](missing.md)\n", encoding="utf-8")

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 0, result.stdout)

    def _run(
        self,
        root: Path,
        mode: str,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                "-m",
                "scripts.check",
                "--root",
                str(root),
                "--mode",
                mode,
                "--json",
            ],
            check=False,
            capture_output=True,
            text=True,
            cwd=SKILL_ROOT,
        )

    def test_nested_family_parent_requires_fixed_documents(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(
                root,
                broken=False,
                extra_families=[(".issueops/OPERATIONS.md", ".issueops/operations/guides")],
                fixed_documents=[".issueops/operations/install.md"],
            )
            docs = root / ".issueops"
            _ = (docs / "operations" / "install.md").write_text("# Install\n", encoding="utf-8")
            _ = (docs / "operations" / "stray.md").write_text("# Stray\n", encoding="utf-8")

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            violations = self._violations(result)
            self.assertIn(("undeclared_document", ".issueops/operations/stray.md"), violations)
            self.assertNotIn(("undeclared_document", ".issueops/operations/install.md"), violations)

    def test_declared_paths_must_exist(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(
                root,
                broken=False,
                directories={".issueops/plans": "plans"},
                fixed_documents=[".issueops/testing/fixed.md"],
            )

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            violations = self._violations(result)
            self.assertIn(("missing_declared_path", ".issueops/plans"), violations)
            self.assertIn(("missing_declared_path", ".issueops/testing/fixed.md"), violations)

    def test_fully_ignored_docs_tree_fails(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)
            _ = subprocess.run(["git", "init", "-q", str(root)], check=True)
            _ = (root / ".gitignore").write_text(".issueops/\n", encoding="utf-8")

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            self.assertIn(("docs_tree_ignored", ".issueops"), self._violations(result))

    def _violations(
        self,
        result: subprocess.CompletedProcess[str],
    ) -> set[tuple[object, object]]:
        raw = self._payload(result)["violations"]
        self.assertIsInstance(raw, list)
        return {
            (item["code"], item["path"])
            for item in cast("list[dict[str, object]]", raw)
        }

    def _write_fixture(
        self,
        root: Path,
        *,
        broken: bool,
        directories: dict[str, str] | None = None,
        root_documents: list[str] | None = None,
        fixed_documents: list[str] | None = None,
        extra_families: list[tuple[str, str]] | None = None,
    ) -> None:
        docs = root / ".issueops"
        modules = docs / "testing"
        modules.mkdir(parents=True)
        root_link = "testing/unit.md"
        _ = (docs / "TESTING.md").write_text(
            f"# Testing\n\n[Unit]({root_link})\n",
            encoding="utf-8",
        )
        module_lines = [
            "# Unit",
            "",
            "[Testing index](../TESTING.md)",
            "",
            "`Candidates [](Index/Recommended/Text)` is not a Markdown link.",
        ]
        if broken:
            module_lines.extend(["", "one", "two", "three"])
            module_lines.extend(["", "[Missing](missing.md)"])
        _ = (modules / "unit.md").write_text(
            "\n".join(module_lines) + "\n",
            encoding="utf-8",
        )
        manifest: dict[str, object] = {
            "schema_version": 1,
            "max_root_lines": 100,
            "max_module_lines": 5,
            "families": [
                {
                    "root": ".issueops/TESTING.md",
                    "module_dir": ".issueops/testing",
                    "responsibility": "testing",
                }
            ],
            "single_owner_topics": dict[str, str](),
        }
        if directories is not None:
            manifest["directories"] = directories
        if root_documents is not None:
            manifest["root_documents"] = root_documents
        if fixed_documents is not None:
            manifest["fixed_documents"] = fixed_documents
        for family_root, module_dir in extra_families or []:
            module = root / module_dir
            module.mkdir(parents=True)
            _ = (module / "guide.md").write_text("# Guide\n", encoding="utf-8")
            _ = (root / family_root).write_text(
                f"# Index\n\n[Guide]({Path(module_dir).relative_to('.issueops')}/guide.md)\n",
                encoding="utf-8",
            )
            cast("list[dict[str, str]]", manifest["families"]).append(
                {"root": family_root, "module_dir": module_dir, "responsibility": "ops"},
            )
        manifest_dir = docs / "documentation"
        manifest_dir.mkdir()
        _ = (manifest_dir / "manifest.json").write_text(
            json.dumps(manifest),
            encoding="utf-8",
        )

    def _payload(
        self,
        result: subprocess.CompletedProcess[str],
    ) -> dict[str, object]:
        payload = cast("object", json.loads(result.stdout))
        self.assertIsInstance(payload, dict)
        return cast("dict[str, object]", payload)


if __name__ == "__main__":
    _ = unittest.main()
