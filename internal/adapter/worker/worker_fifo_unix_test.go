//go:build unix

package worker

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	policy "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
)

func TestWorkerRunReadOnlyTimeoutFailsJobWithBoundedStderr(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	t.Setenv("ISSUEOPS_WORKER_DIR", dir)
	fifo := filepath.Join(root, "blocked.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := RunReadOnlyWorkerJob("read-only-timeout", "", policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"cat", "blocked.fifo"},
		Timeout:       "20ms",
	})
	if err != nil {
		t.Fatalf("worker run timeout: %v", err)
	}
	if job.Status != workercontract.StatusFailed || job.Result == nil || !job.Result.TimedOut || job.Result.ExitCode != 124 {
		t.Fatalf("expected timed-out failed job: %+v", job)
	}
	if !strings.Contains(job.Result.Stderr, "command timed out") || len(job.Result.Stderr) > 4096 {
		t.Fatalf("unexpected timeout stderr: %q", job.Result.Stderr)
	}
	stored, err := ReadWorkerJob(job.ID)
	if err != nil {
		t.Fatalf("read stored job: %v", err)
	}
	if stored.Status != workercontract.StatusFailed || stored.Result == nil || !stored.Result.TimedOut {
		t.Fatalf("stored timeout job missing result: %+v", stored)
	}
}
