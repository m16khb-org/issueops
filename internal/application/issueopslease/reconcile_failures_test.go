package issueopslease

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

type reconcileApplyFailureRepository struct {
	*reconcileRepositoryFake
	applyErr error
}

func (r reconcileApplyFailureRepository) ApplyReceipt(context.Context, ReconcileIntentState, leasecontract.ReconcileStageReceipt) (ReconcileProgress, error) {
	r.applyCalls++
	return ReconcileProgress{}, r.applyErr
}

func TestReconcileServiceRequiresAllDependencies(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	stages := &reconcileStageExecutorFake{}
	tests := map[string]*ReconcileService{
		"nil service":        nil,
		"no repository":      NewReconcileService(nil, stages, noRoleAgents),
		"no stages":          NewReconcileService(repository, nil, noRoleAgents),
		"no role agents":     NewReconcileService(repository, stages, nil),
		"zero value service": {},
	}
	for name, service := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := service.Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
			if err == nil || !strings.Contains(err.Error(), "reconcile service dependencies are required") || result.OK || result.ID != "io-1" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
	if stages.inspectCalls != 0 {
		t.Fatalf("inspected with missing dependencies: %d", stages.inspectCalls)
	}
}

func TestReconcilePreservesIntentWhenInventoryCannotAuthorizeAMutation(t *testing.T) {
	tests := []struct {
		name      string
		inventory leasecontract.ReconcileStageInventory
		attempts  int
		want      string
	}{
		{"multiple candidates", leasecontract.ReconcileStageInventory{Candidates: []leasecontract.ReconcileStageReceipt{{TaskID: "a"}, {TaskID: "b"}}}, 1, "Orca intent inventory found multiple candidates; intent retained"},
		{"exact replay exhausted", leasecontract.ReconcileStageInventory{ExactReplay: true}, 2, "Orca intent retry is exhausted; intent retained"},
		{"proven zero exhausted", leasecontract.ReconcileStageInventory{AuthoritativeZero: true}, 2, "Orca intent retry is exhausted; intent retained"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := "not_invoked_proven"
			repository := reconcileRepositoryFixture("task_create", state, tt.attempts)
			stages := &reconcileStageExecutorFake{attempted: true, inventory: tt.inventory}
			result, err := NewReconcileService(repository, stages, noRoleAgents).Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
			if result.OK || result.Code != "orca_reconcile_ambiguous" || !result.ExternalStateInspected {
				t.Fatalf("result=%+v", result)
			}
			if repository.failureCalls != 1 || repository.failureState != state || repository.markCalls != 0 || stages.invokeCalls != 0 || repository.cleared {
				t.Fatalf("failures=%d state=%q marks=%d invokes=%d cleared=%v", repository.failureCalls, repository.failureState, repository.markCalls, stages.invokeCalls, repository.cleared)
			}
		})
	}
}

func TestReconcileExactReplayInvokesWithinRetryBudget(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "unknown", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{ExactReplay: true}}
	result, err := NewReconcileService(repository, stages, noRoleAgents).Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != "orca_reconcile_advanced_dispatch" || stages.invokeCalls != 1 || repository.markCalls != 1 || repository.applyCalls != 1 {
		t.Fatalf("result=%+v invokes=%d marks=%d applies=%d", result, stages.invokeCalls, repository.markCalls, repository.applyCalls)
	}
}

func TestReconcileCompletesWhenTheFinalStageIsApplied(t *testing.T) {
	repository := reconcileRepositoryFixture("dispatch", "not_invoked_proven", 0)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	result, err := NewReconcileService(repository, stages, noRoleAgents).Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
	if err != nil || !result.OK || !result.Reconciled || result.Code != "orca_reconcile_completed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestReconcileClassifiesInvokeFailureByReportedState(t *testing.T) {
	tests := []struct {
		name         string
		reported     string
		wantRecorded string
	}{
		{"executor reports not invoked", "not_invoked_proven", "not_invoked_proven"},
		{"executor reports nothing", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
			invokeErr := errors.New("orca refused")
			stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}, invokeErr: invokeErr, failureState: tt.reported}
			result, err := NewReconcileService(repository, stages, noRoleAgents).Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
			if !errors.Is(err, invokeErr) || !strings.Contains(err.Error(), "requires execution reconcile; mutation was not repeated") {
				t.Fatalf("error=%v", err)
			}
			if result.OK || result.Code != "orca_reconcile_ambiguous" {
				t.Fatalf("result=%+v", result)
			}
			if repository.failureCalls != 1 || repository.failureState != tt.wantRecorded || repository.applyCalls != 0 {
				t.Fatalf("failures=%d state=%q applies=%d", repository.failureCalls, repository.failureState, repository.applyCalls)
			}
		})
	}
}

func TestReconcileRecordsUnknownWhenApplyingTheReceiptFails(t *testing.T) {
	repository := reconcileRepositoryFixture("run_bind", "unknown", 1)
	applyErr := errors.New("record moved")
	wrapped := reconcileApplyFailureRepository{reconcileRepositoryFake: repository, applyErr: applyErr}
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{Candidates: []leasecontract.ReconcileStageReceipt{{RunID: "run-1", RunBound: true}}}}
	result, err := NewReconcileService(wrapped, stages, noRoleAgents).Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
	if !errors.Is(err, applyErr) || result.OK || result.Code != "orca_reconcile_ambiguous" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if repository.failureCalls != 1 || repository.failureState != "unknown" {
		t.Fatalf("failures=%d state=%q", repository.failureCalls, repository.failureState)
	}
}

func TestReconcileResolvesRoleAgentsFromTheProbeBeforeMarkingTerminalInvoke(t *testing.T) {
	newRepository := func() *reconcileRepositoryFake {
		repository := reconcileRepositoryFixture("terminal_create", "not_invoked_proven", 0)
		repository.state.ProbeHost, repository.state.ProbeRepo = "codex", "/source"
		return repository
	}
	zero := leasecontract.ReconcileStageInventory{AuthoritativeZero: true}

	repository := newRepository()
	stages := &reconcileStageExecutorFake{attempted: true, inventory: zero}
	var seen []string
	service := NewReconcileService(repository, stages, func(_ context.Context, host, repo string) ([]string, error) {
		seen = append(seen, host+"@"+repo)
		return []string{"--agents", "{}"}, nil
	})
	if _, err := service.Reconcile(t.Context(), ReconcileRequest{ID: "io-1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []string{"codex@/source"}) || !reflect.DeepEqual(stages.roleAgentArgs, []string{"--agents", "{}"}) {
		t.Fatalf("resolved=%v launched with=%v", seen, stages.roleAgentArgs)
	}

	repository = newRepository()
	stages = &reconcileStageExecutorFake{attempted: true, inventory: zero}
	settingsErr := errors.New("settings.json is malformed")
	failing := NewReconcileService(repository, stages, func(context.Context, string, string) ([]string, error) { return nil, settingsErr })
	result, err := failing.Reconcile(t.Context(), ReconcileRequest{ID: "io-1"})
	if !errors.Is(err, settingsErr) || !strings.Contains(err.Error(), "resolve role agents before the Orca terminal launch") {
		t.Fatalf("error=%v", err)
	}
	if result.OK || repository.markCalls != 0 || stages.invokeCalls != 0 || repository.failureCalls != 0 {
		t.Fatalf("settings failure must leave the intent untouched: marks=%d invokes=%d failures=%d", repository.markCalls, stages.invokeCalls, repository.failureCalls)
	}
}
