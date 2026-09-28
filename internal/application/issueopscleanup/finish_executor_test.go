package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

type finishTestRecords struct {
	snapshot                                       model.CleanupFinishSnapshot
	events                                         *[]string
	armErr, checkErr, auditErr, deleteErr, failErr error
	failContextErr                                 error
	retained                                       bool
}

func (s *finishTestRecords) Load(context.Context, string) (model.CleanupFinishSnapshot, error) {
	*s.events = append(*s.events, "load")
	return s.snapshot, nil
}
func (s *finishTestRecords) Arm(_ context.Context, _ model.CleanupFinishSnapshot, a model.IssueOpsCleanupFinishAttempt) (model.CleanupFinishSnapshot, error) {
	*s.events = append(*s.events, "arm")
	s.snapshot.Record.CleanupFinishAttempt = &a
	return s.snapshot, s.armErr
}
func (s *finishTestRecords) Check(context.Context, model.CleanupFinishSnapshot) error {
	*s.events = append(*s.events, "check")
	return s.checkErr
}
func (s *finishTestRecords) Fail(ctx context.Context, _ model.CleanupFinishSnapshot, _ model.IssueOpsCleanupFinishFailure, drained bool) (model.CleanupFinishSnapshot, error) {
	*s.events = append(*s.events, "fail")
	s.retained = !drained
	s.failContextErr = ctx.Err()
	return s.snapshot, s.failErr
}
func (s *finishTestRecords) MarkAuditReflected(context.Context, model.CleanupFinishSnapshot, string) (model.CleanupFinishSnapshot, error) {
	*s.events = append(*s.events, "receipt")
	return s.snapshot, s.auditErr
}
func (s *finishTestRecords) Delete(context.Context, model.CleanupFinishSnapshot) error {
	*s.events = append(*s.events, "delete")
	return s.deleteErr
}

type finishTestLease struct {
	events   *[]string
	drainErr error
}

func (l *finishTestLease) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, finishTestContextKey{}, true)
}
func (l *finishTestLease) Close() error { *l.events = append(*l.events, "close"); return nil }
func (l *finishTestLease) Drain(context.Context) (FinishLifetime, error) {
	*l.events = append(*l.events, "drain")
	if l.drainErr != nil {
		return nil, l.drainErr
	}
	return l, nil
}

type finishTestContextKey struct{}

func finishExecutorFixture(t *testing.T) (FinishExecutor, *finishTestRecords, *finishTestLease, *[]string) {
	t.Helper()
	events := []string{}
	lease := &finishTestLease{events: &events}
	records := &finishTestRecords{events: &events, snapshot: model.CleanupFinishSnapshot{Record: model.IssueOpsRecord{ID: "io-finish", Repo: "/repo"}, Revision: "initial"}}
	checkContext := func(ctx context.Context) {
		t.Helper()
		if ctx.Value(finishTestContextKey{}) != true {
			t.Fatal("effect lost lifetime context")
		}
	}
	executor := FinishExecutor{
		Records: records,
		Acquire: func(context.Context, string) (FinishLifetime, error) {
			events = append(events, "acquire")
			return lease, nil
		},
		Observe: func(ctx context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishRequest, error) {
			checkContext(ctx)
			events = append(events, "observe")
			return r, nil
		},
		Plan: func(ctx context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
			checkContext(ctx)
			events = append(events, "plan")
			return model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/worktree", Branch: "feature", BranchOID: "oid", OrcaWorktreeID: "orca", OrcaRuntimeReady: true}, model.CleanupFinishResult{OK: true, ID: r.ID, Preview: !r.Apply}
		},
		Fingerprint: func(model.CleanupFinishInventory) (string, error) { return "fingerprint", nil },
		NewAttempt: func() (model.IssueOpsCleanupFinishAttempt, error) {
			return model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}, nil
		},
		Completion: func(model.IssueOpsRecord) model.RemoteCompletionSection {
			events = append(events, "completion")
			return model.RemoteCompletionSection{}
		},
		Stop: func(ctx context.Context, _ model.CleanupFinishInventory, _ []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error) {
			checkContext(ctx)
			events = append(events, "stop")
			return nil, 0, nil
		},
		RemoveOrca: func(ctx context.Context, _ string) error {
			checkContext(ctx)
			events = append(events, "orca")
			return nil
		},
		Directory: func(string) (bool, error) { events = append(events, "directory"); return true, nil },
		Git: func(ctx context.Context, _ string, args ...string) (int, string) {
			checkContext(ctx)
			events = append(events, strings.Join(args, " "))
			return 0, ""
		},
		ReflectAudit: func(ctx context.Context, _ model.IssueOpsRecord, _ model.RemoteCompletionSection, _ string) error {
			checkContext(ctx)
			events = append(events, "audit")
			return nil
		},
		Now: func() time.Time { return time.Date(2026, 9, 29, 0, 0, 1, 0, time.UTC) },
	}
	return executor, records, lease, &events
}
func finishApplyRequest() model.CleanupFinishRequest {
	return model.CleanupFinishRequest{ID: "io-finish", Apply: true, Confirm: true, Fingerprint: "fingerprint"}
}

func TestFinishExecutorOrdersBoundEffectsAndDrainsBeforeDelete(t *testing.T) {
	executor, _, _, events := finishExecutorFixture(t)
	result, err := executor.Run(context.Background(), finishApplyRequest())
	if err != nil || !result.RecordDeleted || !result.AuditReflected {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{"acquire", "load", "observe", "plan", "completion", "arm", "check", "stop", "check", "orca", "directory", "check", "worktree remove /worktree", "check", "update-ref -d refs/heads/feature oid", "check", "audit", "receipt", "drain", "delete", "close"}
	if !reflect.DeepEqual(*events, want) {
		t.Fatalf("events=%v want=%v", *events, want)
	}
}
func TestFinishExecutorRejectsDriftBeforeDestructiveEffects(t *testing.T) {
	executor, records, _, events := finishExecutorFixture(t)
	records.armErr = errors.New("artifact changed")
	result, err := executor.Run(context.Background(), finishApplyRequest())
	if err == nil || result.RecordDeleted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, event := range *events {
		if event == "stop" || event == "orca" || event == "delete" {
			t.Fatalf("effect after stale observation: %v", *events)
		}
	}
}
func TestFinishExecutorRetainsUndrainedAttemptAndDoesNotDelete(t *testing.T) {
	executor, records, lease, events := finishExecutorFixture(t)
	lease.drainErr = errors.New("child still active")
	result, err := executor.Run(context.Background(), finishApplyRequest())
	if err == nil || result.RecordDeleted || !records.retained {
		t.Fatalf("result=%+v retained=%v err=%v", result, records.retained, err)
	}
	if !reflect.DeepEqual((*events)[len(*events)-2:], []string{"drain", "fail"}) {
		t.Fatalf("effects after drainage failure: %v", *events)
	}
}
func TestFinishExecutorAuditOwnershipFailureBlocksDeletion(t *testing.T) {
	executor, records, _, events := finishExecutorFixture(t)
	records.auditErr = errors.New("owner changed")
	result, err := executor.Run(context.Background(), finishApplyRequest())
	if err == nil || result.RecordDeleted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, event := range *events {
		if event == "delete" {
			t.Fatalf("deleted after local receipt failure: %v", *events)
		}
	}
}
func TestFinishExecutorPreviewDoesNotArm(t *testing.T) {
	executor, _, _, events := finishExecutorFixture(t)
	result, err := executor.Run(context.Background(), model.CleanupFinishRequest{ID: "io-finish"})
	if err != nil || result.Fingerprint != "fingerprint" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(*events, []string{"acquire", "load", "observe", "plan", "close"}) {
		t.Fatalf("preview effects=%v", *events)
	}
}

func TestFinishExecutorCancellationStillDrainsAndRecordsFailure(t *testing.T) {
	executor, records, _, _ := finishExecutorFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executor.Stop = func(context.Context, model.CleanupFinishInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error) {
		cancel()
		return nil, 0, ctx.Err()
	}
	persistenceFailure := errors.New("receipt persistence failed")
	records.failErr = persistenceFailure
	result, err := executor.Run(ctx, finishApplyRequest())
	if result.RecordDeleted || !errors.Is(err, context.Canceled) || !errors.Is(err, persistenceFailure) || records.retained || records.failContextErr != nil {
		t.Fatalf("cancellation finalization: result=%+v err=%v retained=%v finalContext=%v", result, err, records.retained, records.failContextErr)
	}
}

func TestFinishExecutorProviderAuditFailureRemainsBestEffort(t *testing.T) {
	executor, _, _, events := finishExecutorFixture(t)
	executor.ReflectAudit = func(context.Context, model.IssueOpsRecord, model.RemoteCompletionSection, string) error {
		return errors.New("provider unavailable")
	}
	result, err := executor.Run(context.Background(), finishApplyRequest())
	if err != nil || !result.RecordDeleted || result.AuditReflected || result.AuditError != "provider unavailable" {
		t.Fatalf("audit failure: result=%+v err=%v", result, err)
	}
	for _, event := range *events {
		if event == "receipt" {
			t.Fatal("unconfirmed provider write stamped local receipt")
		}
	}
}
