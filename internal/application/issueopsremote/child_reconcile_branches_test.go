package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// faultyRecovery wraps the recording provider and injects read/attach faults.
type faultyRecovery struct {
	*recoveryChildProvider
	readErr, attachErr error
	attachNoEffect     bool
}

func (f *faultyRecovery) ReadChild(ctx context.Context, repo, parent, url string) (port.ChildSnapshot, error) {
	if f.readErr != nil {
		return port.ChildSnapshot{}, f.readErr
	}
	return f.recoveryChildProvider.ReadChild(ctx, repo, parent, url)
}

func (f *faultyRecovery) AttachChild(ctx context.Context, repo, parent, url string) error {
	if f.attachErr != nil {
		return f.attachErr
	}
	if f.attachNoEffect {
		f.attaches++
		return nil
	}
	return f.recoveryChildProvider.AttachChild(ctx, repo, parent, url)
}

// strandedChild leaves one child created remotely whose hierarchy link is missing.
func strandedChild(t *testing.T) (*testChildStore, *recoveryChildProvider, ChildReconciler, ChildCreateResult, ChildCreateCommand) {
	t.Helper()
	store, p, creator, reconciler, cmd := childRecoveryFixture("github")
	p.fail = errors.New("lost response")
	first, err := creator.Create(context.Background(), cmd, nil)
	if err == nil || first.ChildURL == "" {
		t.Fatalf("setup must strand a child: %+v %v", first, err)
	}
	return store, p, reconciler, first, cmd
}

func TestChildReconcileRefusesUnknownOrUnnamedOperation(t *testing.T) {
	store, _, reconciler, first, cmd := strandedChild(t)
	for _, id := range []string{"", strings.Repeat("f", 32)} {
		_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: id, Confirm: true}, nil)
		if err == nil || !strings.Contains(err.Error(), "child operation not found") {
			t.Fatalf("operation %q err = %v", id, err)
		}
	}
	if op := store.read().ChildCreateOperations[0]; op.OperationID != first.OperationID || op.Status == model.IssueCreateIntentCompleted {
		t.Fatalf("operation must be untouched: %+v", op)
	}
}

func TestChildReconcileObservesAncestryOnlyForExecutionRecords(t *testing.T) {
	boom := errors.New("ancestry refused")
	store, _, reconciler, first, cmd := strandedChild(t)
	observed := 0
	observe := func() ([]model.NativeProcessReceipt, error) { observed++; return nil, boom }
	if _, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID}, observe); err != nil || observed != 0 {
		t.Fatalf("records without an execution never observe ancestry: err=%v observed=%d", err, observed)
	}
	store.mu.Lock()
	store.record.Execution = &model.Execution{Lease: model.WriteLease{Generation: 1}}
	store.mu.Unlock()
	if _, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID}, observe); !errors.Is(err, boom) || observed != 1 {
		t.Fatalf("err=%v observed=%d", err, observed)
	}
}

func TestChildReconcileRefusesWhenProviderCannotBeResolved(t *testing.T) {
	boom := errors.New("no provider")
	_, _, reconciler, first, cmd := strandedChild(t)
	reconciler.Resolve = func(string) (port.IssueProviderChildCreateRecovery, error) { return nil, boom }
	if _, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestChildReconcileRecordsReadbackFailureOnlyWhenConfirmed(t *testing.T) {
	boom := errors.New("readback down")
	for _, confirm := range []bool{false, true} {
		t.Run(map[bool]string{false: "preview", true: "confirm"}[confirm], func(t *testing.T) {
			store, p, reconciler, first, cmd := strandedChild(t)
			reconciler.Resolve = func(string) (port.IssueProviderChildCreateRecovery, error) {
				return &faultyRecovery{recoveryChildProvider: p, readErr: boom}, nil
			}
			_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: confirm}, nil)
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
			op := store.read().ChildCreateOperations[0]
			if confirm && (op.Status != model.IssueCreateIntentVerificationFailed || !strings.Contains(op.Failure, "readback down")) {
				t.Fatalf("a confirmed failure must be durable: %+v", op)
			}
			if !confirm && op.Status == model.IssueCreateIntentVerificationFailed {
				t.Fatalf("a preview must not record failure: %+v", op)
			}
		})
	}
}

func TestChildReconcileAttachFailuresStayUnadopted(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fault func(*recoveryChildProvider) *faultyRecovery
		want  string
	}{
		{"attach call fails", func(p *recoveryChildProvider) *faultyRecovery {
			return &faultyRecovery{recoveryChildProvider: p, attachErr: errors.New("attach denied")}
		}, "attach denied"},
		{"hierarchy still missing after attach", func(p *recoveryChildProvider) *faultyRecovery {
			return &faultyRecovery{recoveryChildProvider: p, attachNoEffect: true}
		}, "child hierarchy verification failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, p, reconciler, first, cmd := strandedChild(t)
			reconciler.Resolve = func(string) (port.IssueProviderChildCreateRecovery, error) { return tc.fault(p), nil }
			_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v", err)
			}
			rec := store.read()
			if rec.ChildCreateOperations[0].Status != model.IssueCreateIntentVerificationFailed || len(rec.IssueLinks) != 0 {
				t.Fatalf("op=%+v links=%d", rec.ChildCreateOperations[0], len(rec.IssueLinks))
			}
		})
	}
}

// driftingStore changes the record's branch after the first read, as another
// writer could between observation and the remote write.
type driftingStore struct {
	*testChildStore
	reads int
}

func (s *driftingStore) Read(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	s.reads++
	record, err := s.testChildStore.Read(ctx, id)
	if s.reads == 1 {
		s.mu.Lock()
		s.record.Branch = "other-branch"
		s.mu.Unlock()
	}
	return record, err
}

func TestChildReconcileRefusesWriteWhenAuthorityDriftsAfterObservation(t *testing.T) {
	store, p, reconciler, first, cmd := strandedChild(t)
	reconciler.Records = &driftingStore{testChildStore: store}
	_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "child recovery authority changed") || p.attaches != 0 {
		t.Fatalf("err=%v attaches=%d", err, p.attaches)
	}
	if len(store.read().IssueLinks) != 0 {
		t.Fatal("no link may be recorded after authority drift")
	}
}

func TestChildReconcileReauthorizationReadFailureStopsBeforeRemoteWrite(t *testing.T) {
	boom := errors.New("store down")
	store, p, reconciler, first, cmd := strandedChild(t)
	reconciler.Records = &failingSecondRead{testChildStore: store, err: boom}
	if _, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil); !errors.Is(err, boom) || p.attaches != 0 {
		t.Fatalf("err=%v attaches=%d", err, p.attaches)
	}
}

type failingSecondRead struct {
	*testChildStore
	err   error
	reads int
}

func (s *failingSecondRead) Read(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	s.reads++
	if s.reads > 1 {
		return model.IssueOpsRecord{}, s.err
	}
	return s.testChildStore.Read(ctx, id)
}

func TestChildReconcileRefusesWhenOperationBelongsToDifferentParent(t *testing.T) {
	store, p, reconciler, first, cmd := strandedChild(t)
	store.mu.Lock()
	store.record.IssueURL = "https://github.com/acme/repo/issues/99"
	store.mu.Unlock()
	_, err := reconciler.Reconcile(context.Background(), ChildReconcileCommand{ID: cmd.ID, OperationID: first.OperationID, Confirm: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "parent changed") || p.attaches != 0 {
		t.Fatalf("err=%v attaches=%d", err, p.attaches)
	}
}
