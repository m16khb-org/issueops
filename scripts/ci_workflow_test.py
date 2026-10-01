"""Execute CI verification blocks through disposable command boundaries."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]


def run_blocks():
    """Read the name/run subset used by this workflow, without a YAML dependency."""
    steps = []
    for section in re.split(r"^      - ", (ROOT / ".github/workflows/ci.yml").read_text(), flags=re.M)[1:]:
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
    def test_verification_ownership(self):
        blocks = run_blocks()
        self.assertEqual(sum("unittest discover" in body for _, body in blocks), 0)
        self.assertEqual(sum(body.strip() == "go test ./... -count=1" for _, body in blocks), 0)
        self.assertEqual(sum(body.strip() == "go test -race ./... -count=1" for _, body in blocks), 1)
        self.assertEqual(sum(body.strip() == "go build -o bin/issueops ./cmd/issueops" for _, body in blocks), 1)
        self.assertEqual(sum("./bin/issueops self-verify" in body for _, body in blocks), 1)
        selfverify = next(body for _, body in blocks if "./bin/issueops self-verify" in body)
        self.assertIn("--llm-eval=false", selfverify)

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
                for _, body in run_blocks():
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
                    self.assertEqual(labels, ['build', 'race', 'install', 'self-verify', 'python', 'go', 'golden', 'native'])
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


if __name__ == '__main__':
    unittest.main()
