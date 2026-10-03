# Verified Execution Evidence Contract

Optional examples for `../SKILL.md`, which owns the complete actor contract.
Full mode requires a Manual-QA channel; the root's proportionate low-risk
exception permits auxiliary CLI/data/docs evidence and optional metrics.
Tests alone never prove user-facing completion.

## The Contract

```
I, <agent name>, assert that criterion <id> is PASS.

Evidence type: <HTTP call | terminal/tmux | browser use | computer use | proportionate auxiliary>
Channel command: <exact command run>
Artifact path: <.issueops/verified-execution/evidence/<goal>-<criterion>-<channel>.ext>
Artifact summary: <what the artifact proves>

Cleanup receipt:
  - <resource> → <action taken> → verified by <check>, or "none spawned"
  - ...

Metrics:
  - Rework count for this criterion: <N>
  - Attempts: <N>
  - Cycle: <current cycle of 5>

Signed: <timestamp>
```

## Channel-Specific Evidence Requirements

### HTTP call
```bash
curl -i <url> 2>&1 | tee .issueops/verified-execution/evidence/<goal>-<criterion>.txt
# Artifact MUST contain: HTTP status line, response headers, response body
```

### tmux
```bash
tmux new-session -d -s verified-execution-qa-<criterion>
tmux send-keys -t verified-execution-qa-<criterion> '<command>' Enter
# Observe the exact completion event with a bounded timeout before capture.
tmux capture-pane -t verified-execution-qa-<criterion> -pS -E - > .issueops/verified-execution/evidence/<goal>-<criterion>.txt
tmux kill-session -t verified-execution-qa-<criterion>
# Cleanup receipt: "tmux kill-session verified-execution-qa-<criterion>; verified tmux ls shows no session"
```

### Browser use
```
1. Open Chrome/agent-browser to <url>
2. Perform actions: <exact steps with selectors>
3. Take screenshot: .issueops/verified-execution/evidence/<goal>-<criterion>.png
4. Record action log: .issueops/verified-execution/evidence/<goal>-<criterion>-actions.txt
5. Close browser context
# Cleanup receipt: "browser context closed; no lingering chrome processes"
```

### Computer use
```
1. Launch <application>
2. Perform actions: <exact steps>
3. Take screenshot: .issueops/verified-execution/evidence/<goal>-<criterion>.png
4. Record action log: .issueops/verified-execution/evidence/<goal>-<criterion>-actions.txt
5. Close application
# Cleanup receipt: "application closed; PID <N> confirmed dead"
```

## Invalid Evidence (Reject Immediately)

- "Tests pass" without the required channel or proportionate auxiliary artifact
- `--dry-run` output
- "Should respond with..." (speculation, not observation)
- "Looks correct" (subjective, not binary)
- Worker self-report without re-verification
- Artifact path exists but file is empty or truncated
- Cleanup receipt missing
- Evidence from wrong channel (e.g., CLI dump for browser-facing criterion);
  the root's low-risk auxiliary exception remains valid

## Evidence Quality Standards

1. **Binary pass/fail**: The artifact must make PASS or FAIL obvious without interpretation.
2. **Reproducible**: Another agent reading the channel command must be able to re-run it.
3. **Complete**: No truncation. If output exceeds 32KB, note the truncation point
   and retain/read the full artifact before PASS.
4. **Named**: File name encodes goal + criterion + channel. Example: `G1-C2-http.txt`.
