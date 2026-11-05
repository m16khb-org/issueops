package worker

import (
	"context"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
)

type workerEffects struct {
	jobs   map[string]workercontract.Job
	events []string
}

func (*workerEffects) Dir() (string, error)   { return "/worker", nil }
func (*workerEffects) EnsureDir(string) error { return nil }
func (fake *workerEffects) WithLock(_ context.Context, _, _ string, fn func(context.Context) error) error {
	fake.events = append(fake.events, "lock")
	return fn(context.Background())
}
func (fake *workerEffects) Read(id string) (workercontract.Job, error) {
	return fake.jobs[id], nil
}
func (fake *workerEffects) Write(_ context.Context, job workercontract.Job) error {
	fake.jobs[job.ID] = job
	fake.events = append(fake.events, job.Status)
	return nil
}
func (*workerEffects) Now() time.Time { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }
func (*workerEffects) PID() int       { return 42 }
func (fake *workerEffects) Run(policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	fake.events = append(fake.events, "run")
	return policycontract.CommandRunResult{OK: true}
}
func (*workerEffects) ListIDs(string) ([]string, error) { return nil, nil }
func (*workerEffects) PIDAlive(int) bool                { return true }

func TestRunReadOnlyReleasesLockBeforeCommand(t *testing.T) {
	fake := &workerEffects{jobs: map[string]workercontract.Job{}}
	result, err := (Service{Effects: fake}).RunReadOnly(context.Background(), "safe", "payload", policycontract.CommandPolicyRequest{Argv: []string{"go", "version"}})
	if err != nil || result.Status != workercontract.StatusSucceeded {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{"lock", "queued", "lock", "running", "run", "lock", "succeeded"}
	if len(fake.events) != len(want) {
		t.Fatalf("events=%v", fake.events)
	}
	for i, event := range want {
		if fake.events[i] != event {
			t.Fatalf("events=%v", fake.events)
		}
	}
}
