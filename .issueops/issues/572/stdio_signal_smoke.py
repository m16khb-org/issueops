"""Gate G14 for issue #572: a real `issueops mcp` stdio process ends on SIGTERM.

Builds the CLI, starts `issueops mcp` over stdio with a throwaway state dir,
sends initialize and a waiting channel_recv call, then SIGTERM while the call is
in flight and stdin stays open. The process must exit with status 0 within the
drain bound (10 s) plus slack. POSIX only.
"""

import json
import os
import pathlib
import signal
import subprocess
import sys
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parents[3]
DEADLINE_SECONDS = 20


def send(proc, message):
    proc.stdin.write((json.dumps(message) + "\n").encode())
    proc.stdin.flush()


def main():
    with tempfile.TemporaryDirectory() as tmp:
        binary = pathlib.Path(tmp) / "issueops"
        subprocess.run(["go", "build", "-o", str(binary), "./cmd/issueops"], cwd=ROOT, check=True)
        env = {**os.environ, "ISSUEOPS_STATE_DIR": str(pathlib.Path(tmp) / "state")}
        proc = subprocess.Popen([str(binary), "mcp"], cwd=tmp, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        send(proc, {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "smoke", "version": "0"}}})
        proc.stdout.readline()
        send(proc, {"jsonrpc": "2.0", "method": "notifications/initialized"})
        send(proc, {"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "channel_recv", "arguments": {"channel": "smoke-572", "wait": True, "timeout_seconds": 120}}})
        time.sleep(1)
        if proc.poll() is not None:
            print(f"mcp exited before the signal: {proc.returncode}", file=sys.stderr)
            return 1
        started = time.monotonic()
        proc.send_signal(signal.SIGTERM)
        try:
            code = proc.wait(timeout=DEADLINE_SECONDS)
        except subprocess.TimeoutExpired:
            proc.kill()
            print(f"mcp still running {DEADLINE_SECONDS}s after SIGTERM", file=sys.stderr)
            return 1
        elapsed = time.monotonic() - started
        if code != 0:
            print(f"mcp exit status {code} after SIGTERM ({elapsed:.1f}s)", file=sys.stderr)
            return 1
    print("stdio-signal-ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
