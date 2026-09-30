"""Run a single complete document battery; keep full logs outside tracked docs."""
import datetime
import json
import os
import pathlib
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[3]
os.environ['PATH'] = '/opt/homebrew/bin:' + os.environ['PATH']
os.environ['GOFLAGS'] = '-p=2'
os.environ['GOMAXPROCS'] = '4'
log = pathlib.Path(tempfile.mkdtemp(prefix='issueops-515-battery-'))
commands = [
    ['python3', 'docs/research/2026-09-30-readability/verify.py', mode]
    for mode in ('snapshot', 'metrics', 'evidence', 'decision', 'remote', 'fixtures')
]
commands.append(['python3', '-m', 'unittest', 'discover', '-s', 'scripts', '-p', '*_test.py'])
commands.append(['./bin/issueops', 'self-verify', '--seed=100', '--target-score=95', '--llm-eval=false', '--json'])
results = []
for i, cmd in enumerate(commands):
    started = datetime.datetime.now(datetime.UTC).isoformat()
    with (log / f'{i}.stdout').open('w') as out, (log / f'{i}.stderr').open('w') as err:
        result = subprocess.run(cmd, cwd=ROOT, stdout=out, stderr=err, timeout=1800)
    results.append({'command':cmd, 'started_at':started, 'exit_code':result.returncode})
    (log / 'commands.json').write_text(json.dumps(results, indent=2))
    if result.returncode:
        print('battery failed; preserved logs:', log)
        print((log / f'{i}.stdout').read_text())
        print((log / f'{i}.stderr').read_text())
        raise SystemExit(result.returncode)
    if cmd[0] == './bin/issueops':
        value = json.loads((log / f'{i}.stdout').read_text())
        assert value['ok']
        steps = [step for run in value['runs'] for step in run['steps']]
        assert len(steps) == 26 and all(step['ok'] for step in steps)
        print('self-verify steps verified:', len(steps))
print('logs:', log)
print('document battery passed')
