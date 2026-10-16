package issueopsexecution

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type stubVerifier struct {
	identity model.NativeActor
	err      error
}

func (v stubVerifier) Verify(context.Context, model.NativeActor) (model.VerifiedActor, error) {
	return model.VerifiedActor{Identity: v.identity}, v.err
}

func reconcileService(exec *model.Execution, readErr error, verr error) Service {
	return Service{
		ReadRecord: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{ID: "cycle", Execution: exec}, readErr
		},
		SamePath: func(a, b string) bool { return a == b },
		Verifier: stubVerifier{identity: model.NativeActor{Host: "h"}, err: verr},
	}
}

func execWithPending(kind string) *model.Execution {
	e := &model.Execution{Workspace: model.Workspace{SourceRoot: "/src", Root: "/wt"}}
	if kind != "" {
		e.Pending = &model.ExternalIntent{OperationID: "op", Kind: kind}
	}
	return e
}

func TestReconcilePreconditionFailures(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		svc  Service
		cwd  string
		msg  string
		is   error
	}{
		{name: "no verifier", svc: Service{}, msg: "execution reconcile actor verifier is required"},
		{name: "verify error", svc: reconcileService(nil, nil, boom), is: boom},
		{name: "read error", svc: reconcileService(nil, boom, nil), is: boom},
		{name: "not prepared", svc: reconcileService(nil, nil, nil), msg: "IssueOps execution v1 is not prepared"},
		{name: "foreign cwd", svc: reconcileService(execWithPending(""), nil, nil), cwd: "/elsewhere", msg: "execution reconcile cwd must be source_root or the canonical worktree"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.svc.Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Preview: true, CWD: tc.cwd}, port.ExecutionReconcileDependencies{})
			if got.OK || got.ID != "cycle" || err == nil || (tc.msg != "" && err.Error() != tc.msg) || (tc.is != nil && !errors.Is(err, tc.is)) {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
}

func TestReconcilePreviewCodes(t *testing.T) {
	cases := []struct {
		kind, code string
	}{
		{"", "no_pending_external_intent"},
		{"remote_pr_create", "remote_reconcile_required"},
		{"mystery", "unsupported_external_intent"},
	}
	for _, tc := range cases {
		t.Run(tc.code+"/"+tc.kind, func(t *testing.T) {
			svc := reconcileService(execWithPending(tc.kind), nil, nil)
			got, err := svc.Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Preview: true, CWD: "/wt"}, port.ExecutionReconcileDependencies{})
			if err != nil || !got.OK || !got.Preview || got.Code != tc.code {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if (tc.kind == "") != (got.Pending == nil) {
				t.Fatalf("pending=%+v", got.Pending)
			}
			if tc.kind == "" && !got.Reconciled {
				t.Fatal("no pending intent must report reconciled")
			}
		})
	}
}

func holderActor() model.NativeActor {
	return model.NativeActor{Host: "codex", SessionID: "owner", SessionProcess: &model.NativeProcessReceipt{PID: 7, StartedAt: "t0", Executable: "/bin/codex"}}
}

func guardedService(kind string) Service {
	holder := holderActor()
	exec := &model.Execution{
		Workspace: model.Workspace{SourceRoot: "/src", Root: "/wt", Branch: "b", BaseHead: "base", LinkedAt: "t"},
		Lease:     model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: "t"},
		Pending:   &model.ExternalIntent{OperationID: "op", Kind: kind, Marker: "m", StartedAt: "t"},
	}
	svc := reconcileService(exec, nil, nil)
	svc.Verifier = stubVerifier{identity: holder}
	return svc
}

func TestReconcileConfirmRoutesRemoteIntentToHandlerWithSnapshot(t *testing.T) {
	remoteCalled := false
	deps := port.ExecutionReconcileDependencies{
		RemoteReconcile: func(_ context.Context, _ string, req model.ExecutionReconcileRequest) (model.ExecutionReconcileResult, error) {
			remoteCalled = req.Snapshot != nil && req.Snapshot.ID == "cycle"
			return model.ExecutionReconcileResult{OK: true, Code: "remote"}, nil
		},
	}
	got, err := guardedService("remote_pr_create").Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Confirm: true, CWD: "/wt"}, deps)
	if err != nil || got.Code != "remote" || !remoteCalled {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestReconcileConfirmAlwaysRequiresTheCurrentHolder(t *testing.T) {
	called := false
	deps := port.ExecutionReconcileDependencies{
		RemoteReconcile: func(context.Context, string, model.ExecutionReconcileRequest) (model.ExecutionReconcileResult, error) {
			called = true
			return model.ExecutionReconcileResult{OK: true}, nil
		},
	}
	svc := guardedService("remote_pr_create")
	svc.Verifier = stubVerifier{identity: model.NativeActor{Host: "codex", SessionID: "intruder", SessionProcess: holderActor().SessionProcess}}
	got, err := svc.Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Confirm: true, CWD: "/wt"}, deps)
	if err == nil || !strings.Contains(err.Error(), "requires the current write lease holder") || got.OK || called {
		t.Fatalf("got=%+v err=%v called=%v", got, err, called)
	}
}

func TestReconcileConfirmWithoutRemoteHandlerFailsClosed(t *testing.T) {
	got, err := guardedService("remote_pr_create").Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Confirm: true, CWD: "/wt"}, port.ExecutionReconcileDependencies{})
	if !errors.Is(err, model.ErrRemotePullRequestReconcileHandlerUnavailable) || got.OK || got.Code != "remote_reconcile_unavailable" || got.Pending == nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestReconcileConfirmUnsupportedKindIsRejected(t *testing.T) {
	got, err := guardedService("mystery").Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Confirm: true, CWD: "/wt"}, port.ExecutionReconcileDependencies{
		RemoteReconcile: func(context.Context, string, model.ExecutionReconcileRequest) (model.ExecutionReconcileResult, error) {
			t.Fatal("handler reached")
			return model.ExecutionReconcileResult{}, nil
		}})
	if got.OK || got.Code != "unsupported_external_intent" || err == nil || !strings.Contains(err.Error(), "unsupported pending external intent kind") {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
