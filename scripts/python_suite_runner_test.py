"""Regression fixtures for the self-verify Python suite entry point."""

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import python_suite_runner


RUNNER = Path(python_suite_runner.__file__).resolve()


class PythonSuiteRunnerTests(unittest.TestCase):
    def write(self, root, path, content):
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")

    def root_suite(self, root):
        self.write(root, "scripts/root_test.py", """
import unittest
from pathlib import Path
class RootTest(unittest.TestCase):
    def test_once(self):
        with Path('root-executed').open('a') as log:
            log.write('once\\n')
""")

    def run_runner(self, root):
        return subprocess.run(
            [sys.executable, str(RUNNER)], cwd=root,
            capture_output=True, text=True, timeout=15, check=False,
        )

    def test_both_shapes_patterns_functions_and_skill_isolation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.root_suite(root)
            for skill in ("alpha", "beta"):
                self.write(root, f"skills/{skill}/scripts/helper.py", f"VALUE = '{skill}'\n")
                for path in ("tests/test_context_test.py", "scripts/context_test.py"):
                    self.write(root, f"skills/{skill}/{path}", f"""
from pathlib import Path
import helper
def test_context():
    assert helper.VALUE == '{skill}'
    assert Path.cwd().name == '{skill}'
    with Path('executed').open('a') as log:
        log.write('{path}\\n')
""")
            result = self.run_runner(root)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual((root / "root-executed").read_text(), "once\n")
            self.assertIn("root=1 skills=2 skill_files=4", result.stdout)
            for skill in ("alpha", "beta"):
                self.assertEqual(
                    sorted((root / "skills" / skill / "executed").read_text().splitlines()),
                    ["scripts/context_test.py", "tests/test_context_test.py"],
                )
            self.assertEqual(result.stdout.count("Python test file:"), 4)

    def test_skill_failures_are_visible_and_other_skills_still_run(self):
        cases = [
            ("failed", "def test_result():\n    assert False\n", "FAIL:"),
            ("malformed", "def test_result(:\n", "SyntaxError"),
            ("import", "import missing_fixture_dependency\n", "ModuleNotFoundError"),
            ("empty", "VALUE = 1\n", "no tests collected"),
            ("skip", "import unittest\n@unittest.skip('fixture')\ndef test_result():\n    pass\n", "skipped"),
        ]
        for name, body, evidence in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                self.root_suite(root)
                self.write(root, "skills/alpha/tests/test_result.py", body)
                self.write(root, "skills/beta/scripts/result_test.py",
                           "from pathlib import Path\ndef test_result():\n    Path('executed').write_text('yes')\n")
                result = self.run_runner(root)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn(evidence, result.stderr)
                self.assertEqual((root / "skills/beta/executed").read_text(), "yes")
                self.assertEqual((root / "root-executed").read_text(), "once\n")

    def test_empty_skills_are_not_empty_discovery_success(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.root_suite(root)
            (root / "skills/empty/tests").mkdir(parents=True)
            result = self.run_runner(root)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("root=1 skills=0 skill_files=0", result.stdout)
            self.assertEqual((root / "root-executed").read_text(), "once\n")

    def test_root_failure_propagates_without_duplicate_discovery(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.root_suite(root)
            self.write(root, "scripts/failure_test.py",
                       "import unittest\nclass Failure(unittest.TestCase):\n    def test_result(self):\n        self.fail('root failure')\n")
            result = self.run_runner(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("root failure", result.stderr)
            self.assertEqual((root / "root-executed").read_text(), "once\n")

    def test_empty_root_suite_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scripts").mkdir()
            result = self.run_runner(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("no tests collected from root scripts suite", result.stderr)

    def test_optional_root_skip_preserves_existing_unittest_semantics(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.root_suite(root)
            self.write(root, "scripts/optional_test.py", """
import unittest
class Optional(unittest.TestCase):
    @unittest.skip('optional local-only input')
    def test_optional(self):
        self.fail('must not run without local input')
""")
            self.write(root, "skills/alpha/tests/test_result.py",
                       "from pathlib import Path\ndef test_result():\n    Path('executed').write_text('yes')\n")
            result = self.run_runner(root)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("skipped=1", result.stderr)
            self.assertEqual((root / "root-executed").read_text(), "once\n")
            self.assertEqual((root / "skills/alpha/executed").read_text(), "yes")


if __name__ == "__main__":
    unittest.main()
