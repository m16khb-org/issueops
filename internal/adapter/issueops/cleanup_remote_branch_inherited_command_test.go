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

func TestCleanupRemoteBranchRetainsAttemptUntilInheritedGitChildExits(t *testing.T) {
	root, record := remoteBranchTestRecord(t)
	shim := t.TempDir()
	pidFile := filepath.Join(shim, "child.pid")
	script := `#!/bin/sh
if [ "$1" = push ]; then
 sleep 30 </dev/null >/dev/null 2>&1 &
 printf '%s' "$!" > "$ISSUEOPS_REMOTE_CHILD_PID"
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ISSUEOPS_REMOTE_CHILD_PID", pidFile)
	var once sync.Once
	stop := func() {
		once.Do(func() {
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
	t.Cleanup(stop)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	deps.Git = func(ctx context.Context, repo string, args ...string) (int, string) {
		if args[0] == "push" {
			return defaultExecutionSyncBaseGit(ctx, repo, args...)
		}
		return git.run(ctx, repo, args...)
	}
	preview, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || got.OK || !got.Deleted || got.FailedStep != "record_release" {
		t.Fatalf("live child allowed finalization: %+v %v", got, err)
	}
	current, err := ReadIssueOps(root, record.ID)
	if err != nil || current.CleanupAttempt == nil {
		t.Fatalf("lost undrained attempt: %v", err)
	}
	if _, err := writeIssueOps(context.Background(), root, record); err == nil {
		t.Fatal("ordinary writer entered undrained attempt")
	}
	if _, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps); err == nil {
		t.Fatal("preview bypassed inherited lifetime")
	}
	stop()
	deadline := time.Now().Add(3 * time.Second)
	for {
		lease, err := (CleanupLifetimeLock{StateRoot: root}).Acquire(context.Background(), record.ID)
		if err == nil {
			_ = lease.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child lifetime not released: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	git.remoteOID = ""
	preview, err = CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil || !preview.AlreadyAbsent {
		t.Fatalf("absence preview: %+v %v", preview, err)
	}
	got, err = CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, true, "stale"), deps)
	if err != nil || !got.AlreadyAbsent || got.Deleted {
		t.Fatalf("drained absence recovery: %+v %v", got, err)
	}
	current, err = ReadIssueOps(root, record.ID)
	if err != nil || current.CleanupAttempt != nil {
		t.Fatalf("recovered attempt remains: %v", err)
	}
}
