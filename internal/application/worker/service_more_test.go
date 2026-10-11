package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
)

// scriptedEffects는 오류 주입과 PID 생존 여부를 제어할 수 있는 Effects 구현이다.
type scriptedEffects struct {
	workerEffects
	dirErr    error
	ensureErr error
	listErr   error
	writeErr  error
	runOK     bool
	ids       []string
	alive     map[int]bool
}

func (fake *scriptedEffects) Dir() (string, error) {
	if fake.dirErr != nil {
		return "", fake.dirErr
	}
	return "/worker", nil
}
func (fake *scriptedEffects) EnsureDir(string) error { return fake.ensureErr }
func (fake *scriptedEffects) Read(id string) (workercontract.WorkerJob, error) {
	job, ok := fake.jobs[id]
	if !ok {
		return workercontract.WorkerJob{}, errors.New("not found")
	}
	return job, nil
}
func (fake *scriptedEffects) Write(ctx context.Context, job workercontract.WorkerJob) error {
	if fake.writeErr != nil {
		return fake.writeErr
	}
	return fake.workerEffects.Write(ctx, job)
}
func (fake *scriptedEffects) Run(policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return policycontract.CommandRunResult{OK: fake.runOK}
}
func (fake *scriptedEffects) ListIDs(string) ([]string, error) { return fake.ids, fake.listErr }
func (fake *scriptedEffects) PIDAlive(pid int) bool            { return fake.alive[pid] }

func newScripted(jobs ...workercontract.WorkerJob) *scriptedEffects {
	fake := &scriptedEffects{workerEffects: workerEffects{jobs: map[string]workercontract.WorkerJob{}}, alive: map[int]bool{}}
	for _, job := range jobs {
		fake.jobs[job.ID] = job
		fake.ids = append(fake.ids, job.ID)
	}
	return fake
}

func TestEnqueueRejectsInvalidKindsAndPropagatesEffectErrors(t *testing.T) {
	for _, kind := range []string{"", "   ", "a/b", `a\b`, strings.Repeat("k", 81)} {
		fake := newScripted()
		job, err := (Service{Effects: fake}).Enqueue(context.Background(), kind, "p")
		if err == nil || job.OK || len(fake.jobs) != 0 {
			t.Fatalf("kind %q: job=%+v err=%v jobs=%v", kind, job, err, fake.jobs)
		}
	}
	dirErr := errors.New("dir")
	fake := newScripted()
	fake.dirErr = dirErr
	if _, err := (Service{Effects: fake}).Enqueue(context.Background(), "k", "p"); !errors.Is(err, dirErr) {
		t.Fatalf("err=%v", err)
	}
	ensureErr := errors.New("ensure")
	fake = newScripted()
	fake.ensureErr = ensureErr
	if _, err := (Service{Effects: fake}).Enqueue(context.Background(), "k", "p"); !errors.Is(err, ensureErr) || len(fake.jobs) != 0 {
		t.Fatalf("err=%v jobs=%v", err, fake.jobs)
	}
}

func TestEnqueueStoresQueuedJobWithRedactedPayload(t *testing.T) {
	fake := newScripted()
	job, err := (Service{Effects: fake}).Enqueue(context.Background(), " build ", "token=ghp_abcdefghijklmnopqrstuvwxyz0123456789")
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := fake.jobs[job.ID]
	if !ok || stored.Status != workercontract.WorkerStatusQueued || stored.Kind != "build" || !stored.OK {
		t.Fatalf("stored=%+v", stored)
	}
	if strings.Contains(stored.Payload, "ghp_abcdefghijklmnopqrstuvwxyz0123456789") {
		t.Fatalf("payload not redacted: %q", stored.Payload)
	}
	if stored.WorkerDir != "/worker" || !stored.NoShell {
		t.Fatalf("stored=%+v", stored)
	}
}

func TestCancelTransitions(t *testing.T) {
	queued := workercontract.WorkerJob{ID: "q", Status: workercontract.WorkerStatusQueued}
	running := workercontract.WorkerJob{ID: "r", Status: workercontract.WorkerStatusRunning}
	cancelled := workercontract.WorkerJob{ID: "c", Status: workercontract.WorkerStatusCancelled}
	fake := newScripted(queued, running, cancelled)
	svc := Service{Effects: fake}

	job, err := svc.Cancel(context.Background(), "q")
	if err != nil || job.Status != workercontract.WorkerStatusCancelled || fake.jobs["q"].Status != workercontract.WorkerStatusCancelled {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if job.UpdatedAt != "2000-01-01T00:00:00Z" {
		t.Fatalf("updated=%q", job.UpdatedAt)
	}

	before := len(fake.events)
	job, err = svc.Cancel(context.Background(), "c")
	if err != nil || job.Status != workercontract.WorkerStatusCancelled {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	for _, event := range fake.events[before:] {
		if event != "lock" {
			t.Fatalf("already-cancelled job must not be rewritten: %v", fake.events)
		}
	}

	job, err = svc.Cancel(context.Background(), "r")
	if err == nil || !strings.Contains(err.Error(), "cannot be cancelled") || job.Status != workercontract.WorkerStatusRunning {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if fake.jobs["r"].Status != workercontract.WorkerStatusRunning {
		t.Fatal("running job must stay running")
	}
}

func TestCancelErrors(t *testing.T) {
	fake := newScripted()
	if _, err := (Service{Effects: fake}).Cancel(context.Background(), "missing"); err == nil {
		t.Fatal("missing job must error")
	}
	dirErr := errors.New("dir")
	fake.dirErr = dirErr
	job, err := (Service{Effects: fake}).Cancel(context.Background(), "x")
	if !errors.Is(err, dirErr) || job.ID != "x" || job.OK {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	queued := workercontract.WorkerJob{ID: "q", Status: workercontract.WorkerStatusQueued}
	fake = newScripted(queued)
	writeErr := errors.New("write")
	fake.writeErr = writeErr
	if _, err := (Service{Effects: fake}).Cancel(context.Background(), "q"); !errors.Is(err, writeErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestRunReadOnlyRecordsFailureResult(t *testing.T) {
	fake := newScripted()
	job, err := (Service{Effects: fake}).RunReadOnly(context.Background(), "safe", "p", policycontract.CommandPolicyRequest{Argv: []string{"false"}})
	if err != nil {
		t.Fatal(err)
	}
	stored := fake.jobs[job.ID]
	if stored.Status != workercontract.WorkerStatusFailed || stored.OK || stored.Result == nil || stored.Result.OK {
		t.Fatalf("stored=%+v", stored)
	}
	if len(stored.Command) != 1 || stored.Command[0] != "false" || stored.PID != 42 {
		t.Fatalf("stored=%+v", stored)
	}
}

func TestRunReadOnlyErrors(t *testing.T) {
	fake := newScripted()
	if _, err := (Service{Effects: fake}).RunReadOnly(context.Background(), "", "p", policycontract.CommandPolicyRequest{}); err == nil {
		t.Fatal("empty kind must fail before anything runs")
	}
	if len(fake.events) != 0 {
		t.Fatalf("events=%v", fake.events)
	}
	writeErr := errors.New("write")
	fake = newScripted()
	fake.writeErr = writeErr
	if _, err := (Service{Effects: fake}).RunReadOnly(context.Background(), "k", "p", policycontract.CommandPolicyRequest{}); !errors.Is(err, writeErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestDetectStuckMarksOnlyDeadRunningJobsNewestFirst(t *testing.T) {
	deadOld := workercontract.WorkerJob{ID: "dead-old", Status: workercontract.WorkerStatusRunning, PID: 1, CreatedAt: "2000-01-01"}
	deadNew := workercontract.WorkerJob{ID: "dead-new", Status: workercontract.WorkerStatusRunning, PID: 2, CreatedAt: "2000-01-02"}
	live := workercontract.WorkerJob{ID: "live", Status: workercontract.WorkerStatusRunning, PID: 3, CreatedAt: "2000-01-03"}
	queued := workercontract.WorkerJob{ID: "queued", Status: workercontract.WorkerStatusQueued, CreatedAt: "2000-01-04"}
	fake := newScripted(deadOld, deadNew, live, queued)
	fake.ids = append(fake.ids, "unreadable")
	fake.alive[3] = true

	result, err := (Service{Effects: fake}).DetectStuck(context.Background())
	if err != nil || !result.OK || result.WorkerDir != "/worker" || result.Queue != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(result.Jobs) != 2 || result.Jobs[0].ID != "dead-new" || result.Jobs[1].ID != "dead-old" {
		t.Fatalf("jobs=%+v", result.Jobs)
	}
	for _, job := range result.Jobs {
		if job.Status != workercontract.WorkerStatusFailed || job.OK || fake.jobs[job.ID].Status != workercontract.WorkerStatusFailed {
			t.Fatalf("job=%+v", job)
		}
	}
	if fake.jobs["live"].Status != workercontract.WorkerStatusRunning || fake.jobs["queued"].Status != workercontract.WorkerStatusQueued {
		t.Fatal("live/queued jobs must be untouched")
	}
}

func TestDetectStuckSkipsJobsWhoseWriteFailsAndReportsListErrors(t *testing.T) {
	dead := workercontract.WorkerJob{ID: "d", Status: workercontract.WorkerStatusRunning, PID: 9}
	fake := newScripted(dead)
	fake.writeErr = errors.New("write")
	result, err := (Service{Effects: fake}).DetectStuck(context.Background())
	if err != nil || len(result.Jobs) != 0 || result.Jobs == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	listErr := errors.New("list")
	fake = newScripted()
	fake.listErr = listErr
	if result, err = (Service{Effects: fake}).DetectStuck(context.Background()); !errors.Is(err, listErr) || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	dirErr := errors.New("dir")
	fake = newScripted()
	fake.dirErr = dirErr
	if result, err = (Service{Effects: fake}).DetectStuck(context.Background()); !errors.Is(err, dirErr) || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestListSortsNewestFirstWithQueueStatsAndSkipsUnreadable(t *testing.T) {
	jobs := []workercontract.WorkerJob{
		{ID: "a", Status: workercontract.WorkerStatusQueued, CreatedAt: "2000-01-01"},
		{ID: "b", Status: workercontract.WorkerStatusRunning, CreatedAt: "2000-01-03"},
		{ID: "c", Status: workercontract.WorkerStatusSucceeded, CreatedAt: "2000-01-02"},
	}
	fake := newScripted(jobs...)
	fake.ids = append(fake.ids, "ghost")
	result, err := (Service{Effects: fake}).List()
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(result.Jobs) != 3 || result.Jobs[0].ID != "b" || result.Jobs[1].ID != "c" || result.Jobs[2].ID != "a" {
		t.Fatalf("jobs=%+v", result.Jobs)
	}
	q := result.Queue
	if q == nil || q.Total != 3 || q.Queued != 1 || q.Running != 1 || q.Succeeded != 1 || q.Depth != 2 {
		t.Fatalf("queue=%+v", q)
	}
}

func TestListAndReadErrorPaths(t *testing.T) {
	dirErr := errors.New("dir")
	fake := newScripted()
	fake.dirErr = dirErr
	if result, err := (Service{Effects: fake}).List(); !errors.Is(err, dirErr) || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	listErr := errors.New("list")
	fake = newScripted()
	fake.listErr = listErr
	if result, err := (Service{Effects: fake}).List(); !errors.Is(err, listErr) || result.Queue != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	fake = newScripted(workercontract.WorkerJob{ID: "x", Kind: "k", UpdatedAt: time.Time{}.String()})
	job, err := (Service{Effects: fake}).Read("x")
	if err != nil || job.Kind != "k" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if _, err := (Service{Effects: fake}).Read("nope"); err == nil {
		t.Fatal("missing must error")
	}
}
