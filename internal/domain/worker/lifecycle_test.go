package worker

import (
	"testing"

	workercontract "issueops/internal/contract/worker"
)

func TestWorkerLifecycleRejectsInvalidTransitions(t *testing.T) {
	if _, err := NewQueued("bad/path", "payload", "job-1", "/worker", "time"); err == nil {
		t.Fatal("invalid kind accepted")
	}
	queued, err := NewQueued("read-only", "redacted", "job-1", "/worker", "time")
	if err != nil || queued.Status != workercontract.WorkerStatusQueued {
		t.Fatalf("queued=%+v err=%v", queued, err)
	}
	running, err := Start(queued, []string{"go", "version"}, 22, "later", "updated")
	if err != nil || running.Status != workercontract.WorkerStatusRunning || running.PID != 22 {
		t.Fatalf("running=%+v err=%v", running, err)
	}
	if _, err := Cancel(running, "later"); err == nil {
		t.Fatal("running job cancelled")
	}
	failed := Finish(running, false, "done")
	if failed.Status != workercontract.WorkerStatusFailed || failed.OK {
		t.Fatalf("failed=%+v", failed)
	}
	cancelled, err := Cancel(queued, "later")
	if err != nil || cancelled.Status != workercontract.WorkerStatusCancelled {
		t.Fatalf("cancelled=%+v err=%v", cancelled, err)
	}
	again, err := Cancel(cancelled, "after")
	if err != nil || again.UpdatedAt != cancelled.UpdatedAt {
		t.Fatalf("idempotent cancel=%+v err=%v", again, err)
	}
	stuck, changed := MarkStuck(running, false, "after")
	if !changed || stuck.Status != workercontract.WorkerStatusFailed || stuck.Result != nil {
		t.Fatalf("stuck=%+v changed=%v", stuck, changed)
	}
	if _, changed := MarkStuck(running, true, "after"); changed {
		t.Fatal("live PID marked failed")
	}
}
