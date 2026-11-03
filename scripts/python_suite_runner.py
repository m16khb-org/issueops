#!/usr/bin/env python3
"""Run repository unittest suites and skill-local tests with isolated imports."""

import importlib.util
import inspect
import re
import subprocess
import sys
import traceback
import unittest
from pathlib import Path

# Import errors are suite failures, never skips (.issueops/testing/unit-and-contract.md).
# The message only names the usual precondition so the failure is easy to act on.
REQUIREMENTS_HINT = (
    "install the deps declared in scripts/python_test_requirements.txt into a Python 3.13 venv "
    "(.issueops/testing/unit-and-contract.md, section 'Python 검사 환경')"
)
IMPORT_ERROR_LINE = re.compile(r"^(?:ModuleNotFoundError|ImportError): (.*)$", re.MULTILINE)
MISSING_MODULE = re.compile(r"No module named '([^']+)'")


def report_missing_module(label: str, detail: str) -> None:
    """Name the failing suite/file and the missing module on one stderr line."""
    match = MISSING_MODULE.search(detail)
    module = match.group(1) if match else detail.strip()
    print(f"precondition failed: {label}: cannot import {module}; {REQUIREMENTS_HINT}", file=sys.stderr, flush=True)


def report_import_failures(result: unittest.TestResult, label: str) -> None:
    """Emit the precondition line for each failed or errored test that ended in an import error."""
    for test, trace in [*result.errors, *result.failures]:
        # unittest wraps a failed module import in a generic ImportError whose
        # chained cause names the module, so prefer the "No module named" line.
        details = IMPORT_ERROR_LINE.findall(trace)
        if details:
            detail = next((item for item in details if MISSING_MODULE.search(item)), details[0])
            report_missing_module(f"{label} ({test.id()})", detail)


def run_suite(files: list[Path]) -> int:
    """Load each matched file once, including existing zero-argument test functions."""
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    label = f"skill {Path.cwd().name}"
    for index, path in enumerate(files):
        spec = importlib.util.spec_from_file_location(f"_skill_test_{index}", path)
        if spec is None or spec.loader is None:
            sys.exit(f"cannot import Python test file: {path}")
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        try:
            spec.loader.exec_module(module)
        except ImportError as exc:
            traceback.print_exc()
            report_missing_module(f"{label}: {path}", f"No module named '{exc.name}'" if exc.name else str(exc))
            return 1
        tests = loader.loadTestsFromModule(module)
        for name, function in inspect.getmembers(module, inspect.isfunction):
            if name.startswith("test_") and function.__module__ == module.__name__:
                tests.addTest(unittest.FunctionTestCase(function))
        count = tests.countTestCases()
        print(f"Python test file: {path} ({count} tests)", flush=True)
        if count == 0:
            sys.exit(f"no tests collected from Python test file: {path}")
        suite.addTests(tests)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    report_import_failures(result, label)
    return int(not result.wasSuccessful() or bool(result.skipped))


def main() -> int:
    """Keep root discovery once and start one checked-interpreter process per skill."""
    if sys.version_info < (3, 10):
        sys.exit("Python script tests require Python 3.10+; found " + sys.version.split()[0])
    if sys.argv[1:2] == ["--suite"]:
        cwd = Path.cwd()
        sys.path[:1] = [str(cwd), str(cwd / "scripts"), str(cwd / "tests")]
        files = [Path(arg) for arg in sys.argv[2:]]
        for directory in sorted({str(path.parent) for path in files}):
            sys.path.insert(0, directory)
        return run_suite(files)

    root = Path.cwd()
    sys.path.insert(0, str(root))
    suite = unittest.defaultTestLoader.discover(str(root / "scripts"), pattern="*_test.py")
    count = suite.countTestCases()
    print(f"Python root scripts suite: {count} tests", flush=True)
    if count == 0:
        sys.exit("no tests collected from root scripts suite")
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    report_import_failures(result, "root scripts suite")
    failed_suites = [] if result.wasSuccessful() else ["root scripts"]
    skill_files = 0
    skill_suites = 0
    for skill in sorted((root / "skills").glob("*")):
        files = sorted({
            path.relative_to(skill)
            for directory in ("tests", "scripts")
            for pattern in ("test_*.py", "*_test.py")
            for path in (skill / directory).rglob(pattern)
            if path.is_file()
        })
        if not files:
            continue
        skill_files += len(files)
        skill_suites += 1
        print(f"Python skill suite: {skill.name} ({len(files)} files)", flush=True)
        completed = subprocess.run(
            [sys.executable, str(Path(__file__).resolve()), "--suite", *map(str, files)],
            cwd=skill,
            check=False,
        )
        if completed.returncode != 0:
            failed_suites.append(f"skill {skill.name}")
    print(f"Python suites: root=1 skills={skill_suites} skill_files={skill_files}", flush=True)
    if failed_suites:
        print("Python suites failed: " + ", ".join(failed_suites), file=sys.stderr, flush=True)
    return int(bool(failed_suites))


if __name__ == "__main__":
    sys.exit(main())
