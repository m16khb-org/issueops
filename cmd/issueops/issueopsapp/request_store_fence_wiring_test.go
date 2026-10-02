package issueopsapp

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	loopcontract "issueops/internal/contract/looprun"
	policycontract "issueops/internal/contract/policy"
	"issueops/internal/port"
)

func boundLoopRequest(t *testing.T, repo, grant, tool string) context.Context {
	t.Helper()
	key, token, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(t.Context(), grant)
	if err != nil {
		t.Fatal(err)
	}
	bound, _, err := bindIssueOpsAuthority(t.Context(), authoritycontract.Use{Key: key, Token: token, WorkspaceRoot: repo, CWD: repo, Tool: tool})
	if err != nil {
		t.Fatalf("bind %s: %v", tool, err)
	}
	return bound
}

func loopFenceFixture(t *testing.T) (string, string, model.NativeProcessReceipt, loopcontract.LoopRun) {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grant := authorizeSessionForTest(t, repo, "loop-fence", self)
	started, err := newScopedLoopService(repo).Start(boundLoopRequest(t, repo, grant, "loop_start"), loopcontract.StartLoopRequest{Repo: repo, Name: "fence", Goal: "recheck the grant under lock"})
	if err != nil {
		t.Fatalf("bound loop start: %v", err)
	}
	return repo, grant, self, started
}

func requireNoLoopAttempt(t *testing.T, id string) {
	t.Helper()
	status, err := newLoopService().Status(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Loop.Attempts) != 0 {
		t.Fatalf("loop recorded an attempt without a live grant: %+v", status.Loop.Attempts)
	}
}

func TestScopedLoopWriteRechecksGrantUnderLockAfterRotation(t *testing.T) {
	repo, grant, self, started := loopFenceFixture(t)
	bound := boundLoopRequest(t, repo, grant, "loop_record_attempt")
	authorizeSessionForTest(t, repo, "loop-fence", self)
	_, err := newScopedLoopService(repo).RecordAttempt(bound, started.ID, loopcontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"after rotation"}})
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("loop write after rotation err = %v, want revoked grant", err)
	}
	requireNoLoopAttempt(t, started.ID)
}

// worker_run_read_only writes its job record in the fixed worker store, but the
// capability that authorized the workspace command must still be live under the
// grant lock: a revocation committed while the request waits rejects the run.
func TestScopedWorkerRunLosesGrantRevokedWhileFenceIsHeld(t *testing.T) {
	repo, grant, _, _ := loopFenceFixture(t)
	bound := boundLoopRequest(t, repo, grant, "worker_run_read_only")
	key, _, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(t.Context(), grant)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := sqlstore.Open(issueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	if err := grants.WithSpan(t.Context(), func(spanCtx context.Context) error {
		go func() {
			_, err := newScopedWorkerService().RunReadOnly(bound, "fence-probe", "", policycontract.CommandPolicyRequest{
				WorkspaceRoot: repo, CWD: repo, Argv: []string{"git", "status", "--short"}, Timeout: "30s",
			})
			result <- err
		}()
		return grants.Apply(spanCtx, []port.RecordMutation{{Bucket: authoritycontract.Bucket, ID: key, Delete: true}})
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "revoked") {
			t.Fatalf("worker run after revocation under the fence err = %v, want revoked grant", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("fenced worker run never returned")
	}
	listed, err := newWorkerService().List()
	if err != nil || len(listed.Jobs) != 0 {
		t.Fatalf("worker recorded a job without a live grant: %+v err=%v", listed.Jobs, err)
	}
}

// A write on the grant root that carries a bound capability rechecks the
// grant inside its own data transaction, even outside any span, so a rotation
// committed after Bind rejects it. Unbound native writes are unaffected.
func TestBoundGrantRootWriteRechecksGrantAfterRotation(t *testing.T) {
	repo, grant, self, _ := loopFenceFixture(t)
	bound := boundLoopRequest(t, repo, grant, "issueops_execution")
	db, err := sqlstore.Open(issueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	probe := func(id string) port.RecordMutation {
		return port.RecordMutation{Bucket: "grant_write_probe", ID: id, Data: []byte(`{}`)}
	}
	if err := db.Apply(bound, []port.RecordMutation{probe("before")}); err != nil {
		t.Fatalf("write with a live grant: %v", err)
	}
	authorizeSessionForTest(t, repo, "loop-fence", self)
	if err := db.Apply(bound, []port.RecordMutation{probe("after")}); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("write after rotation err = %v, want revoked grant", err)
	}
	before, _, err := db.Get("grant_write_probe", "before")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CompareAndApply(bound, []port.ExpectedRecord{{Bucket: "grant_write_probe", ID: "before", Data: before}}, []port.RecordMutation{probe("cas-after")}); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("CAS write after rotation err = %v, want revoked grant", err)
	}
	for _, id := range []string{"after", "cas-after"} {
		if _, found, err := db.Get("grant_write_probe", id); err != nil || found {
			t.Fatalf("rejected write %s persisted found=%v err=%v", id, found, err)
		}
	}
	if err := db.Apply(t.Context(), []port.RecordMutation{probe("native")}); err != nil {
		t.Fatalf("unbound write: %v", err)
	}
}

func TestScopedLoopWriteLosesGrantRevokedWhileFenceIsHeld(t *testing.T) {
	repo, grant, _, started := loopFenceFixture(t)
	bound := boundLoopRequest(t, repo, grant, "loop_record_attempt")
	key, _, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(t.Context(), grant)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := sqlstore.Open(issueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	if err := grants.WithSpan(t.Context(), func(spanCtx context.Context) error {
		go func() {
			_, err := newScopedLoopService(repo).RecordAttempt(bound, started.ID, loopcontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"while fenced"}})
			result <- err
		}()
		return grants.Apply(spanCtx, []port.RecordMutation{{Bucket: authoritycontract.Bucket, ID: key, Delete: true}})
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "revoked") {
			t.Fatalf("loop write after revocation under the fence err = %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("fenced loop write never returned")
	}
	requireNoLoopAttempt(t, started.ID)
}
