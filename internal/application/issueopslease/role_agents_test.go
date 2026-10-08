package issueopslease

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type roleAgentStages struct {
	resumeTraceStages
	args map[string][]string
}

func (s roleAgentStages) Invoke(ctx context.Context, intent ResumeIntentState, roleAgentArgs []string) (leasecontract.ResumeStageReceipt, error) {
	s.args[intent.Stage] = roleAgentArgs
	return s.resumeTraceStages.Invoke(ctx, intent, roleAgentArgs)
}

func resumeRequest(record Record) ResumeRequest {
	return ResumeRequest{
		ID: record.ID, ExpectedGeneration: 4, Actor: resumeApplicationActor(),
		Ancestry: []leasedomain.ProcessReceipt{*resumeApplicationActor().Process}, CWD: "/worktree", Confirm: true,
	}
}

func TestResumeInjectsRoleAgentsIntoTheTerminalStageFromTheBinding(t *testing.T) {
	trace := []string{}
	record := resumeApplicationTestRecord(4)
	record.Stable.Repo = "/source"
	repository := &resumeTraceRepository{record: record, trace: &trace}
	stages := roleAgentStages{resumeTraceStages: resumeTraceStages{trace: &trace}, args: map[string][]string{}}
	var calls []string
	service := resumeApplicationStageServiceWith(record, repository, stages, func(_ context.Context, host, repo string) ([]string, error) {
		calls = append(calls, host+"@"+repo)
		return []string{"--agents", "{}"}, nil
	})
	if _, err := service.Resume(context.Background(), resumeRequest(record)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"codex@/source"}) {
		t.Fatalf("role agents resolved for %v, want the binding owner host", calls)
	}
	for stage, args := range stages.args {
		want := []string(nil)
		if stage == "terminal_create" {
			want = []string{"--agents", "{}"}
		}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("stage %s args = %q, want %q", stage, args, want)
		}
	}
}

func TestResumeRoleAgentFailureLeavesNoPendingIntent(t *testing.T) {
	trace := []string{}
	record := resumeApplicationTestRecord(4)
	repository := &resumeTraceRepository{record: record, trace: &trace}
	service := resumeApplicationStageServiceWith(record, repository, resumeTraceStages{trace: &trace}, func(context.Context, string, string) ([]string, error) {
		return nil, errors.New("/xdg/issueops/agent-models.json: unsupported version 2")
	})
	_, err := service.Resume(context.Background(), resumeRequest(record))
	if err == nil || !strings.Contains(err.Error(), "agent-models.json") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(trace, "begin") {
		t.Fatalf("a settings error must stop before BeginIntent: %v", trace)
	}
}

func TestReconcileRoleAgentUsesProbeHost(t *testing.T) {
	repository := reconcileRepositoryFixture("terminal_create", "not_invoked_proven", 0)
	repository.state.ProbeHost, repository.state.ProbeRepo = "claude", "/source"
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	var calls []string
	_, err := NewReconcileService(repository, stages, func(_ context.Context, host, repo string) ([]string, error) {
		calls = append(calls, host+"@"+repo)
		return []string{"--agents", "{}"}, nil
	}).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"claude@/source"}) || !reflect.DeepEqual(stages.roleAgentArgs, []string{"--agents", "{}"}) {
		t.Fatalf("calls=%v args=%q", calls, stages.roleAgentArgs)
	}

	other := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	otherStages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	if _, err := NewReconcileService(other, otherStages, func(context.Context, string, string) ([]string, error) {
		t.Fatal("non-terminal stages must not resolve role agents")
		return nil, nil
	}).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileRoleAgentFailureKeepsNotInvoked(t *testing.T) {
	repository := reconcileRepositoryFixture("terminal_create", "not_invoked_proven", 0)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	_, err := NewReconcileService(repository, stages, func(context.Context, string, string) ([]string, error) {
		return nil, errors.New("/repo/.issueops/agent-models.local.json: unknown role")
	}).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "agent-models.local.json") {
		t.Fatalf("err = %v", err)
	}
	if repository.markCalls != 0 || stages.invokeCalls != 0 || repository.failureCalls != 0 {
		t.Fatalf("mark=%d invoke=%d failure=%d", repository.markCalls, stages.invokeCalls, repository.failureCalls)
	}
}
