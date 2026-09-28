//go:build unix

package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCleanupFinishRetainsAttemptUntilInheritedGitChildExits(t *testing.T) {
	root, record, _ := finishTestRecord(t, true)
	shim := t.TempDir()
	pidFile := filepath.Join(shim, "child.pid")
	script := `#!/bin/sh
if [ "$1" = worktree ] && [ "$2" = remove ] && [ ! -f "$ISSUEOPS_FINISH_CHILD_PID" ]; then
  sleep 30 </dev/null >/dev/null 2>&1 &
  printf '%s' "$!" > "$ISSUEOPS_FINISH_CHILD_PID"
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ISSUEOPS_FINISH_CHILD_PID", pidFile)
	var stopOnce sync.Once
	stopChild := func() {
		stopOnce.Do(func() {
			raw, err := os.ReadFile(pidFile)
			if err != nil {
				return
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil {
				return
			}
			child, err := os.FindProcess(pid)
			if err == nil {
				_ = child.Kill()
			}
		})
	}
	t.Cleanup(stopChild)
	git := &fakeFinishGit{branchOID: "abc123"}
	executor := finishExecutorForTests(root, finishDeps(git))
	// Plan observations are deterministic; destructive commands use the actual
	// production runner, which must inherit the lifetime descriptor into the shim.
	executor.Git = (CleanupFinishRuntime{}).Git
	preview, err := executor.Run(context.Background(), finishRequest(record.ID, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor.Run(context.Background(), finishRequest(record.ID, true, preview.Fingerprint))
	if err == nil || result.RecordDeleted {
		t.Fatalf("live descendant allowed finalization: result=%+v err=%v", result, err)
	}
	kept, err := ReadIssueOps(root, record.ID)
	if err != nil || kept.CleanupFinishAttempt == nil {
		t.Fatalf("undrained ownership lost: %v", err)
	}
	if _, err := executor.Run(context.Background(), finishRequest(record.ID, false, "")); err == nil {
		t.Fatal("preview acquired live descendant's lifetime")
	}
	stopChild()
	deadline := time.Now().Add(3 * time.Second)
	for {
		lease, err := (FinishLifetimeLock{StateRoot: root}).Acquire(context.Background(), record.ID)
		if err == nil {
			_ = lease.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child exit did not release lifetime: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	preview, err = executor.Run(context.Background(), finishRequest(record.ID, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	result, err = executor.Run(context.Background(), finishRequest(record.ID, true, preview.Fingerprint))
	if err != nil || !result.RecordDeleted {
		t.Fatalf("exclusive retry failed: result=%+v err=%v", result, err)
	}
}
