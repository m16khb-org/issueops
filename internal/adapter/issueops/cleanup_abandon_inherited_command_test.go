//go:build unix

package issueops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	github "issueops/internal/adapter/provider/github"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestAbandonInheritedChildrenRetainExecutionOwnership(t *testing.T) {
	for _, surface := range []string{"git", "provider"} {
		t.Run(surface, func(t *testing.T) {
			var root string
			var record model.IssueOpsRecord
			var executor cleanupapp.AbandonExecutor
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			shim := t.TempDir()
			pidFile := filepath.Join(shim, "child.pid")
			t.Setenv("ISSUEOPS_ABANDON_CHILD_PID", pidFile)
			t.Setenv("ISSUEOPS_ABANDON_REAL_GIT", realGit)
			script := `#!/bin/sh
if [ "$1" = worktree ] && [ "$2" = remove ] && [ ! -f "$ISSUEOPS_ABANDON_CHILD_PID" ]; then
 sleep 30 </dev/null >/dev/null 2>&1 &
 printf '%s' "$!" > "$ISSUEOPS_ABANDON_CHILD_PID"
fi
exec "$ISSUEOPS_ABANDON_REAL_GIT" "$@"
`
			name := "git"
			req := model.CleanupAbandonRequest{Reason: "cancel cycle"}
			if surface == "git" {
				var worktree string
				root, record, worktree = abandonResidueFixture(t)
				_ = worktree
				executor = abandonExecutorForTests(root, CleanupAbandonDeps{Processes: quietCleanupProcesses()})
			} else {
				root, record = remoteAbandonRecord(t)
				executor = abandonExecutorForTests(root, remoteAbandonDeps(&remoteAbandonGit{}, github.Provider{}))
				executor.Observe = cleanupapp.ObserveAbandonArtifact
				req.ClosePR = true
				name = "gh"
				script = `#!/bin/sh
if [ "$2" = close ]; then
 if [ ! -f "$ISSUEOPS_ABANDON_CHILD_PID" ]; then
  sleep 30 </dev/null >/dev/null 2>&1 &
  printf '%s' "$!" > "$ISSUEOPS_ABANDON_CHILD_PID"
 fi
 exit 0
fi
if [ -f "$ISSUEOPS_ABANDON_CHILD_PID" ]; then
 printf '%s' '{"body":"fixture","state":"CLOSED"}'
else
 printf '%s' '{"body":"fixture","state":"OPEN"}'
fi
`
			}
			if err := os.WriteFile(filepath.Join(shim, name), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
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
					p, err := os.FindProcess(pid)
					if err == nil {
						_ = p.Kill()
					}
				})
			}
			t.Cleanup(stopChild)
			req.ID = record.ID
			preview, err := executor.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			req.Apply = true
			req.Confirm = true
			req.Fingerprint = preview.Fingerprint
			got, err := executor.Run(context.Background(), req)
			if !errors.Is(err, processlease.ErrBusy) || got.RecordDeleted {
				t.Fatalf("child did not keep ownership: %+v %v", got, err)
			}
			kept, err := ReadIssueOps(root, record.ID)
			if err != nil || kept.CleanupAttempt == nil || kept.CleanupAttempt.Operation != model.CleanupOperationAbandon {
				t.Fatalf("undrained attempt lost: %+v %v", kept, err)
			}
			req.Apply = false
			executor.Provider = func(string) (port.IssueProvider, error) {
				t.Error("provider reached while inherited child owns execution")
				return nil, nil
			}
			if _, err := executor.Run(context.Background(), req); !errors.Is(err, processlease.ErrBusy) {
				t.Fatalf("child ownership not enforced: %v", err)
			}
			stopChild()
			deadline := time.Now().Add(3 * time.Second)
			for {
				lease, err := (CleanupLifetimeLock{StateRoot: root}).Acquire(context.Background(), record.ID)
				if err == nil {
					_ = lease.Close()
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("child exit did not release lifetime: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			executor.Provider = func(string) (port.IssueProvider, error) { return github.Provider{}, nil }
			preview, err = executor.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			req.Apply = true
			req.Fingerprint = preview.Fingerprint
			got, err = executor.Run(context.Background(), req)
			if err != nil || !got.RecordDeleted {
				t.Fatalf("exclusive retry failed: %+v %v", got, err)
			}
		})
	}
}
