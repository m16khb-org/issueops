package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type loadFailingFinishRecords struct {
	*finishTestRecords
	err error
}

func (r loadFailingFinishRecords) Load(context.Context, string) (model.CleanupSnapshot, error) {
	return model.CleanupSnapshot{}, r.err
}

func withInventory(executor *FinishExecutor, inventory model.CleanupFinishInventory) {
	executor.Plan = func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
		return inventory, model.CleanupFinishResult{OK: true, ID: r.ID}
	}
}

func TestFinishExecutorRefusesBeforeArmingWhenEvidenceIsMissing(t *testing.T) {
	loadFailure, acquireFailure, fingerprintFailure, attemptFailure := errors.New("record unreadable"), errors.New("lock busy"), errors.New("encode"), errors.New("entropy")
	cases := []struct {
		name      string
		req       model.CleanupFinishRequest
		configure func(*FinishExecutor, *finishTestRecords)
		wantErr   string
		wantIs    error
	}{
		{name: "acquire fails", req: finishApplyRequest(), configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Acquire = func(context.Context, string) (CleanupLifetime, error) { return nil, acquireFailure }
		}, wantIs: acquireFailure},
		{name: "load fails", req: finishApplyRequest(), configure: func(e *FinishExecutor, r *finishTestRecords) {
			e.Records = loadFailingFinishRecords{r, loadFailure}
		}, wantIs: loadFailure},
		{name: "another cleanup owns the cycle", req: finishApplyRequest(), configure: func(_ *FinishExecutor, r *finishTestRecords) {
			r.snapshot.Record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationAbandon}
		}, wantErr: "cleanup abandon owns this cycle"},
		{name: "plan reports missing gates", req: finishApplyRequest(), configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Plan = func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
				return model.CleanupFinishInventory{}, model.CleanupFinishResult{OK: true, ID: r.ID, Missing: []string{"completion_reflected", "branch_oid"}}
			}
		}, wantErr: "cleanup finish is not ready: completion_reflected, branch_oid"},
		{name: "fingerprint fails", req: finishApplyRequest(), configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Fingerprint = func(model.CleanupFinishInventory) (string, error) { return "", fingerprintFailure }
		}, wantIs: fingerprintFailure},
		{name: "apply without confirm", req: model.CleanupFinishRequest{ID: "io-finish", Apply: true, Fingerprint: "fingerprint"}, wantErr: "requires --confirm"},
		{name: "stale fingerprint", req: model.CleanupFinishRequest{ID: "io-finish", Apply: true, Confirm: true, Fingerprint: "old"}, wantErr: "stale cleanup fingerprint"},
		{name: "attempt token unavailable", req: finishApplyRequest(), configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.NewAttempt = func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
				return model.IssueOpsCleanupAttempt{}, attemptFailure
			}
		}, wantIs: attemptFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor, records, _, events := finishExecutorFixture(t)
			if tc.configure != nil {
				tc.configure(&executor, records)
			}
			result, err := executor.Run(t.Context(), tc.req)
			if err == nil || result.OK || result.RecordDeleted {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want %q", err, tc.wantErr)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("err=%v, want wrapping %v", err, tc.wantIs)
			}
			for _, effect := range []string{"arm", "stop", "orca", "delete"} {
				if slices.Contains(*events, effect) {
					t.Fatalf("%s ran before the gates passed: %v", effect, *events)
				}
			}
		})
	}
}

func TestFinishExecutorWrapsObservationFailuresAndSuggestsRemedy(t *testing.T) {
	observed := errors.New("provider down")
	executor, _, _, _ := finishExecutorFixture(t)
	executor.Observe = func(context.Context, model.IssueOpsRecord, model.CleanupFinishRequest) (model.CleanupFinishRequest, error) {
		return model.CleanupFinishRequest{}, observed
	}
	_, err := executor.Run(t.Context(), finishApplyRequest())
	if _, ok := errors.AsType[*port.CleanupFinishObservationError](err); !ok || !errors.Is(err, observed) {
		t.Fatalf("err=%v, want CleanupFinishObservationError wrapping the provider failure", err)
	}

	executor, _, _, _ = finishExecutorFixture(t)
	executor.Plan = func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult) {
		return model.CleanupFinishInventory{}, model.CleanupFinishResult{ID: r.ID, Missing: []string{"completion_reflected"}}
	}
	result, err := executor.Run(t.Context(), model.CleanupFinishRequest{ID: "io-finish"})
	if err == nil || !strings.Contains(result.NextCommand, "remote reflect-completion --id io-finish") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestFinishExecutorSurfacesMaterialWarningInPreview(t *testing.T) {
	executor, _, _, _ := finishExecutorFixture(t)
	executor.MaterialWarning = func(model.IssueOpsRecord) string { return "uncommitted notes in worktree" }
	result, err := executor.Run(t.Context(), model.CleanupFinishRequest{ID: "io-finish"})
	if err != nil || !slices.Equal(result.Warnings, []string{"uncommitted notes in worktree"}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !strings.Contains(result.NextCommand, "--fingerprint fingerprint") {
		t.Fatalf("next=%q", result.NextCommand)
	}
}

func TestFinishExecutorStepFailuresPreserveTheRecord(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name      string
		inventory model.CleanupFinishInventory
		configure func(*FinishExecutor, *finishTestRecords)
		wantStep  string
	}{
		{name: "record check before stop", inventory: model.CleanupFinishInventory{WorktreePresent: true, OrcaRuntimeReady: true}, configure: func(_ *FinishExecutor, r *finishTestRecords) { r.checkErr = boom }, wantStep: model.CleanupFailureStepWorkspaceProcessesStop},
		{name: "stop fails", inventory: model.CleanupFinishInventory{WorktreePresent: true, OrcaTerminals: []string{"t"}}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Stop = func(context.Context, model.CleanupFinishInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error) {
				return nil, 0, boom
			}
		}, wantStep: model.CleanupFailureStepWorkspaceProcessesStop},
		{name: "record check before orca removal", inventory: model.CleanupFinishInventory{OrcaWorktreeID: "orca"}, configure: func(_ *FinishExecutor, r *finishTestRecords) { r.checkErr = boom }, wantStep: model.CleanupFailureStepOrcaRemove},
		{name: "orca remover missing", inventory: model.CleanupFinishInventory{OrcaWorktreeID: "orca"}, configure: func(e *FinishExecutor, _ *finishTestRecords) { e.RemoveOrca = nil }, wantStep: model.CleanupFailureStepOrcaRemove},
		{name: "orca removal fails", inventory: model.CleanupFinishInventory{OrcaWorktreeID: "orca"}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.RemoveOrca = func(context.Context, string) error { return boom }
		}, wantStep: model.CleanupFailureStepOrcaRemove},
		{name: "directory unobservable after orca removal", inventory: model.CleanupFinishInventory{OrcaWorktreeID: "orca", WorktreePresent: true, WorktreeRoot: "/wt"}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Directory = func(string) (bool, error) { return false, boom }
		}, wantStep: model.CleanupFailureStepWorktreeRemove},
		{name: "record check before worktree removal", inventory: model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/wt"}, configure: func(_ *FinishExecutor, r *finishTestRecords) { r.checkErr = boom }, wantStep: model.CleanupFailureStepWorktreeRemove},
		{name: "git worktree remove fails and directory remains", inventory: model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/wt"}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Git = func(context.Context, string, ...string) (int, string) { return 128, "dirty" }
		}, wantStep: model.CleanupFailureStepWorktreeRemove},
		{name: "git worktree remove fails and directory unobservable", inventory: model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/wt"}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Git = func(context.Context, string, ...string) (int, string) { return 128, "dirty" }
			e.Directory = func(string) (bool, error) { return false, boom }
		}, wantStep: model.CleanupFailureStepWorktreeRemove},
		{name: "record check before branch delete", inventory: model.CleanupFinishInventory{BranchOID: "oid", Branch: "feature"}, configure: func(_ *FinishExecutor, r *finishTestRecords) { r.checkErr = boom }, wantStep: model.CleanupFailureStepBranchDelete},
		{name: "update-ref fails and branch still exists", inventory: model.CleanupFinishInventory{BranchOID: "oid", Branch: "feature"}, configure: func(e *FinishExecutor, _ *finishTestRecords) {
			e.Git = func(_ context.Context, _ string, args ...string) (int, string) {
				if args[0] == "update-ref" {
					return 1, "lock held"
				}
				return 0, ""
			}
		}, wantStep: model.CleanupFailureStepBranchDelete},
		{name: "record delete fails", inventory: model.CleanupFinishInventory{}, configure: func(_ *FinishExecutor, r *finishTestRecords) { r.deleteErr = boom }, wantStep: model.CleanupFailureStepRecordDelete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor, records, _, events := finishExecutorFixture(t)
			withInventory(&executor, tc.inventory)
			tc.configure(&executor, records)
			result, err := executor.Run(t.Context(), finishApplyRequest())
			if err == nil || result.FailedStep != tc.wantStep || result.OK || result.RecordDeleted {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !strings.Contains(err.Error(), "step "+tc.wantStep+" failed") || !strings.Contains(result.NextCommand, "cleanup finish --id io-finish --preview") {
				t.Fatalf("err=%v next=%q", err, result.NextCommand)
			}
			if !slices.Contains(*events, "fail") || records.retained {
				t.Fatalf("failure receipt not recorded after drain: events=%v retained=%v", *events, records.retained)
			}
			if tc.wantStep != model.CleanupFailureStepRecordDelete && slices.Contains(*events, "delete") {
				t.Fatalf("record deleted despite %s failure: %v", tc.wantStep, *events)
			}
		})
	}
}

func TestFinishExecutorTreatsAlreadyGoneResourcesAsRemoved(t *testing.T) {
	t.Run("worktree vanished after git failure", func(t *testing.T) {
		executor, _, _, _ := finishExecutorFixture(t)
		withInventory(&executor, model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/wt"})
		executor.Git = func(context.Context, string, ...string) (int, string) { return 128, "not a worktree" }
		executor.Directory = func(string) (bool, error) { return false, nil }
		result, err := executor.Run(t.Context(), finishApplyRequest())
		if err != nil || !result.WorktreeRemoved || !result.RecordDeleted {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("orca removal already deleted the directory", func(t *testing.T) {
		executor, _, _, events := finishExecutorFixture(t)
		withInventory(&executor, model.CleanupFinishInventory{OrcaWorktreeID: "orca", WorktreePresent: true, WorktreeRoot: "/wt"})
		executor.Directory = func(string) (bool, error) { return false, nil }
		result, err := executor.Run(t.Context(), finishApplyRequest())
		if err != nil || !result.OrcaRemoved || !result.WorktreeRemoved {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		for _, event := range *events {
			if strings.HasPrefix(event, "worktree remove") {
				t.Fatalf("git worktree remove ran for a vanished directory: %v", *events)
			}
		}
	})
	t.Run("branch ref vanished after update-ref failure", func(t *testing.T) {
		executor, _, _, _ := finishExecutorFixture(t)
		withInventory(&executor, model.CleanupFinishInventory{BranchOID: "oid", Branch: "feature"})
		executor.Git = func(_ context.Context, _ string, args ...string) (int, string) {
			if args[0] == "update-ref" {
				return 1, "missing"
			}
			return 1, ""
		}
		result, err := executor.Run(t.Context(), finishApplyRequest())
		if err != nil || !result.BranchDeleted {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
}

func TestFinishExecutorSkipsStopWhenNothingOccupiesTheWorktree(t *testing.T) {
	for name, inventory := range map[string]model.CleanupFinishInventory{
		"worktree absent": {WorktreePresent: false, OrcaTerminals: []string{"t"}, OrcaRuntimeReady: true},
		"nothing to stop": {WorktreePresent: true, WorktreeRoot: "/wt"},
	} {
		executor, _, _, events := finishExecutorFixture(t)
		withInventory(&executor, inventory)
		if _, err := executor.Run(t.Context(), finishApplyRequest()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if slices.Contains(*events, "stop") {
			t.Fatalf("%s: stop ran: %v", name, *events)
		}
	}
}
