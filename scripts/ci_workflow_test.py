"""Execute CI verification blocks through disposable command boundaries."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]


def job_sections():
    """Split the workflow into {job id: text} in file order, without a YAML dependency."""
    text = (ROOT / ".github/workflows/ci.yml").read_text()
    parts = re.split(r"^  ([a-z][a-z-]*):\n", text.split("\njobs:\n", 1)[1], flags=re.M)
    return dict(zip(parts[1::2], parts[2::2]))


def run_blocks(*jobs):
    """Read the name/run subset of the given jobs (default: every job), in file order."""
    steps = []
    for job, text in job_sections().items():
        if jobs and job not in jobs:
            continue
        for section in re.split(r"^      - ", text, flags=re.M)[1:]:
            name = re.match(r"name: (.+)", section)
            run = re.search(r"^        run: (.+)\n?", section, re.M)
            if not name or not run:
                continue
            body = run[1]
            if body == "|":
                body = textwrap.dedent(section[run.end():])
            steps.append((name[1], body))
    return steps


class CIWorkflowTests(unittest.TestCase):
    def test_jobs_run_in_parallel(self):
        self.assertEqual(list(job_sections()), ["lint", "race", "self-verify"])
        for job, text in job_sections().items():
            self.assertNotRegex(text, r"^    needs:", job)

    def test_lint_job_owns_static_checks(self):
        lint = run_blocks("lint")
        self.assertEqual(sum(body.startswith("unformatted=\"$(gofmt -l ") for _, body in lint), 1)
        self.assertEqual(sum(body.strip() == "go vet ./..." for _, body in lint), 1)
        self.assertEqual(sum(body.strip() == "./scripts/lint-local.sh" for _, body in lint), 1)
        self.assertEqual(sum(body.strip() == "shellcheck install.sh scripts/*.sh" for _, body in lint), 1)
        self.assertEqual(sum("scripts/validate-skill.py" in body for _, body in lint), 1)
        docs = [body for _, body in lint if "python -m scripts.check" in body]
        self.assertEqual(len(docs), 1)
        self.assertIn("--directory skills/project-docs-optimize", docs[0])
        self.assertIn("--mode check", docs[0])
        # The pinned linter version lives only in scripts/lint-local.sh, and the
        # lint run goes through that script so CI and local lint cannot drift.
        everything = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertFalse([body for _, body in run_blocks() if re.search(r"golangci-lint\"? run\b", body)])
        self.assertNotRegex(everything, r"v2\.\d+\.\d+")
        self.assertRegex((ROOT / "scripts/lint-local.sh").read_text(), r'GOLANGCI_LINT_VERSION="v2\.\d+\.\d+"')
        for job in ("race", "self-verify"):
            for _, body in run_blocks(job):
                self.assertNotIn("golangci-lint", body)
                self.assertNotIn("shellcheck", body)

    def test_lint_local_cross_step_disables_cgo(self):
        lines = [line.strip() for line in (ROOT / "scripts/lint-local.sh").read_text().splitlines()]
        self.assertEqual(lines.count("GOOS=linux CGO_ENABLED=0 golangci-lint run ./... || status=1"), 1)
        self.assertEqual([line for line in lines if re.match(r"GOOS=linux\b", line) and "CGO_ENABLED=0" not in line], [])

    def test_lint_local_runs_windows_variant_off_windows(self):
        lines = [line.strip() for line in (ROOT / "scripts/lint-local.sh").read_text().splitlines()]
        command = "GOOS=windows CGO_ENABLED=0 golangci-lint run ./... || status=1"
        self.assertEqual(lines.count(command), 1)
        self.assertEqual([line for line in lines if re.match(r"GOOS=windows", line) and "CGO_ENABLED=0" not in line], [])
        # The windows run sits inside a branch taken only when the host GOOS is not windows.
        index = lines.index(command)
        guard = max(i for i in range(index) if lines[i].startswith("if "))
        self.assertEqual(lines[guard], 'if [ "$(go env GOOS)" != "windows" ]; then')
        self.assertNotIn("fi", lines[guard + 1:index])
        self.assertEqual(lines[index + 1], "fi")

    def test_lint_local_fails_when_the_linter_fails(self):
        if os.name == "nt":
            # Next to git first (Git\cmd\git.exe or Git\ucrt64\bin\git.exe, up to three
            # roots), then the standard install directories.
            candidates = []
            git = shutil.which("git")
            if git:
                root = Path(git).resolve().parent.parent
                for _ in range(3):
                    candidates.append(root / "bin" / "bash.exe")
                    root = root.parent
            candidates += [Path(base) / "Git/bin/bash.exe"
                           for base in (os.environ.get("ProgramW6432"), os.environ.get("ProgramFiles")) if base]
            bash = next((path for path in candidates if path.is_file()), None)
            if bash is None:
                self.skipTest("Git for Windows bash.exe not found")
        else:
            bash = Path("/bin/bash")
            self.assertTrue(bash.is_file(), bash)
        self.assertNotEqual(bash.parent.name.lower(), "system32", "WSL launcher bash cannot run this script")
        version = subprocess.run([str(bash), "--noprofile", "--norc", str(ROOT / "scripts/lint-local.sh"), "--print-version"],
                                 cwd=ROOT, capture_output=True, text=True, timeout=60, check=True).stdout.strip().removeprefix("v")
        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory) / "fake-bin"
            fake_bin.mkdir()
            fake = fake_bin / "golangci-lint"
            fake.write_bytes("\n".join([
                "#!/usr/bin/env bash",
                'if [ "${1:-}" = version ]; then',
                f"  echo 'golangci-lint has version {version} built with go1.26.6'",
                "  exit 0",
                "fi",
                "echo '0 issues.'",
                "exit 1",
                "",
            ]).encode())
            fake.chmod(0o755)
            env = dict(os.environ, PATH=str(fake_bin) + os.pathsep + os.environ["PATH"])
            env.pop("ISSUEOPS_ROOT", None)
            result = subprocess.run([str(bash), "--noprofile", "--norc", str(ROOT / "scripts/lint-local.sh")],
                                    cwd=ROOT, env=env, capture_output=True, text=True, timeout=120)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            self.assertIn("0 issues.", result.stdout)

    def test_verification_ownership(self):
        blocks = run_blocks()
        self.assertEqual(sum("unittest discover" in body for _, body in blocks), 0)
        self.assertEqual(sum(body.strip() == "go test ./... -count=1" for _, body in blocks), 0)
        self.assertEqual(sum(body.strip() == "go test -race ./... -count=1" for _, body in blocks), 1)
        self.assertEqual(sum(body.strip() == "go build -o bin/issueops ./cmd/issueops" for _, body in blocks), 1)
        self.assertEqual(sum("./bin/issueops self-verify" in body for _, body in blocks), 1)
        # Each owner lives in exactly one job: race alone, build and self-verify together.
        self.assertEqual([body.strip() for _, body in run_blocks("race") if "go test" in body], ["go test -race ./... -count=1"])
        self.assertEqual(sum(body.strip() == "go build -o bin/issueops ./cmd/issueops" for _, body in run_blocks("self-verify")), 1)
        self.assertEqual(sum("./bin/issueops self-verify" in body for _, body in run_blocks("self-verify")), 1)
        self.assertEqual(sum("--mcp-transport=http" in body for _, body in run_blocks("self-verify")), 1)
        selfverify = next(body for _, body in blocks if "./bin/issueops self-verify" in body)
        self.assertIn("--llm-eval=false", selfverify)
        install = next(line for line in selfverify.splitlines() if "./scripts/install-native.sh" in line)
        self.assertIn("--mcp-transport=stdio", install)
        http = [body for _, body in blocks if "--mcp-transport=http" in body]
        self.assertEqual(len(http), 1)
        self.assertNotIn("self-verify", http[0])

    def test_blocks_propagate_failures_and_isolate_native_home(self):
        for fail_at in ("", "python", "go", "race", "install"):
            with self.subTest(fail_at=fail_at), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "bin").mkdir()
                (root / "scripts").mkdir()
                (root / "fake-bin").mkdir()
                runner = "#!" + sys.executable + "\n" + textwrap.dedent('''\
                    import json, os, pathlib, sys
                    mode = pathlib.Path(sys.argv[0]).name
                    if mode == 'go':
                        mode = 'build' if sys.argv[1] == 'build' else ('race' if '-race' in sys.argv else 'go')
                    elif mode == 'python3':
                        mode = 'python'
                    elif mode == 'install-native.sh':
                        mode = 'install'
                    else:
                        mode = 'self-verify'
                    def event(label):
                        with open(os.environ['EVENTS'], 'a') as log:
                            log.write(json.dumps({'label': label, 'home': os.environ['HOME'], 'codex': os.environ.get('CODEX_HOME')}) + '\\n')
                        if os.environ['FAIL_AT'] == label:
                            sys.exit(7)
                    event(mode)
                    if mode == 'self-verify':
                        event('python')
                        event('go')
                        event('golden')
                        event('native')
                    ''')
                for path in (root / "fake-bin/go", root / "fake-bin/python3", root / "bin/issueops", root / "scripts/install-native.sh"):
                    path.write_text(runner)
                    path.chmod(0o755)
                events = root / "events.jsonl"
                ambient_home = root / "ambient-home"
                ambient_home.mkdir()
                (ambient_home / "sentinel").write_text("preserved")
                env = dict(os.environ, PATH=str(root / "fake-bin") + os.pathsep + os.environ['PATH'],
                           HOME=str(ambient_home), CODEX_HOME=str(ambient_home / ".codex"),
                           EVENTS=str(events), FAIL_AT=fail_at)
                result = None
                # Jobs are independent in CI; chaining them here still proves each
                # job's blocks propagate failures and keep native HOME isolated.
                for _, body in run_blocks("race", "self-verify"):
                    if not any(command in body for command in ("unittest discover", "go build -o bin/issueops", "go test", "./bin/issueops self-verify")):
                        continue
                    result = subprocess.run(["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", body],
                                            cwd=root, env=env, capture_output=True, text=True, timeout=10)
                    if result.returncode:
                        break
                self.assertIsNotNone(result)
                observed = [json.loads(line) for line in events.read_text().splitlines()]
                labels = [event['label'] for event in observed]
                self.assertEqual(result.returncode, 7 if fail_at else 0, result.stderr)
                if fail_at:
                    self.assertEqual(labels[-1], fail_at)
                else:
                    self.assertEqual(labels, ['race', 'build', 'install', 'self-verify', 'python', 'go', 'golden', 'native'])
                installed = [event for event in observed if event['label'] == 'install']
                if installed:
                    isolated_home = installed[0]['home']
                    self.assertNotEqual(isolated_home, str(ambient_home))
                    for event in observed[labels.index('install'):]:
                        self.assertEqual(event['home'], isolated_home)
                        self.assertEqual(event['codex'], isolated_home + '/.codex')
                    self.assertFalse(Path(isolated_home).exists(), 'EXIT trap must clean up on success and failure')
                self.assertEqual((ambient_home / 'sentinel').read_text(), 'preserved')
                self.assertEqual(list(ambient_home.iterdir()), [ambient_home / 'sentinel'])

    def test_http_home_mismatch_block_requires_refusal_before_writes(self):
        body = next(body for _, body in run_blocks() if "--mcp-transport=http" in body)
        refusal = "unit directory /tmp/x is not searched by the running systemd user manager"
        for mode, expected in (("refuse", 0), ("succeed", 1), ("other", 1), ("unit", 1), ("credential", 1)):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "scripts").mkdir()
                install = root / "scripts/install-native.sh"
                install.write_text("#!" + sys.executable + "\n" + textwrap.dedent('''\
                    import json, os, pathlib, sys
                    home = pathlib.Path(os.environ['HOME'])
                    with open(os.environ['EVENTS'], 'a') as log:
                        log.write(json.dumps({'home': str(home), 'args': sys.argv[1:]}) + '\\n')
                    mode = os.environ['HTTP_MODE']
                    written = {'unit': '.config/systemd/user/issueops-mcp.service', 'credential': '.local/state/issueops/mcp-http/bearer'}
                    if mode in written:
                        (home / written[mode]).parent.mkdir(parents=True)
                        (home / written[mode]).write_text('x')
                    if mode == 'succeed':
                        sys.exit(0)
                    print('other failure' if mode == 'other' else os.environ['REFUSAL'], file=sys.stderr)
                    sys.exit(1)
                    '''))
                install.chmod(0o755)
                events = root / "events.jsonl"
                ambient_home = root / "ambient-home"
                ambient_home.mkdir()
                env = dict(os.environ, HOME=str(ambient_home), EVENTS=str(events), HTTP_MODE=mode, REFUSAL=refusal)
                result = subprocess.run(["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", body],
                                        cwd=root, env=env, capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
                [event] = [json.loads(line) for line in events.read_text().splitlines()]
                self.assertIn("--mcp-transport=http", event["args"])
                self.assertNotEqual(event["home"], str(ambient_home))
                self.assertFalse(Path(event["home"]).exists(), 'EXIT trap must clean up on success and failure')
                self.assertEqual(list(ambient_home.iterdir()), [])


if __name__ == '__main__':
    unittest.main()
