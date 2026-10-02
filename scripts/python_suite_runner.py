#!/usr/bin/env python3
"""Run repository unittest suites and skill-local tests with isolated imports."""

import importlib.util
import inspect
import subprocess
import sys
import unittest
from pathlib import Path


def run_suite(files: list[Path]) -> int:
    """Load each matched file once, including existing zero-argument test functions."""
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    for index, path in enumerate(files):
        spec = importlib.util.spec_from_file_location(f"_skill_test_{index}", path)
        if spec is None or spec.loader is None:
            sys.exit(f"cannot import Python test file: {path}")
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
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
    failed = not result.wasSuccessful()
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
        failed = completed.returncode != 0 or failed
    print(f"Python suites: root=1 skills={skill_suites} skill_files={skill_files}", flush=True)
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
