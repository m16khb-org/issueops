package issueopspreparation

import (
	"strings"
	"testing"
)

func TestOrcaStagePrerequisitesKeepInvocationAndInspectionDistinct(t *testing.T) {
	for _, tt := range []struct {
		name, mode, stage string
		prepared, launch  bool
		wantOwner         bool
		wantError         string
	}{
		{name: "worktree invocation", mode: OrcaStageInvocation, stage: "worktree_create"},
		{name: "terminal invocation", mode: OrcaStageInvocation, stage: "terminal_create", prepared: true, wantOwner: true},
		{name: "invocation needs prepared", mode: OrcaStageInvocation, stage: "terminal_create", wantError: "sealed worktree receipt"},
		{name: "inspection needs launch", mode: OrcaStageInspection, stage: "terminal_create", prepared: true, wantError: "sealed worktree and launch receipts"},
		{name: "terminal inspection", mode: OrcaStageInspection, stage: "terminal_create", prepared: true, launch: true, wantOwner: true},
		{name: "unknown stage", mode: OrcaStageInspection, stage: "unknown", wantError: "unsupported Orca execution intent stage"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			owner, err := ValidateOrcaStagePrerequisites(OrcaStageFacts{Mode: tt.mode, Stage: tt.stage, Prepared: tt.prepared, Launch: tt.launch})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error=%v want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || owner != tt.wantOwner {
				t.Fatalf("owner=%v err=%v want owner=%v", owner, err, tt.wantOwner)
			}
		})
	}
}

func TestOrcaStageReceiptsRejectLaterOrMissingStages(t *testing.T) {
	for _, tt := range []struct {
		name, stage, terminal, run, task string
		bound                            bool
		wantError                        string
	}{
		{name: "worktree clean", stage: "worktree_create"},
		{name: "worktree later receipt", stage: "worktree_create", terminal: "pty-1", wantError: "worktree intent contains a later-stage receipt"},
		{name: "terminal clean", stage: "terminal_create"},
		{name: "terminal later receipt", stage: "terminal_create", run: "run-1", wantError: "terminal intent contains a later-stage receipt"},
		{name: "run missing terminal", stage: "run_create", wantError: "Run intent requires exactly one terminal receipt"},
		{name: "run clean", stage: "run_create", terminal: "pty-1"},
		{name: "run bind missing run", stage: "run_bind", terminal: "pty-1", wantError: "Run bind intent requires terminal and Run receipts"},
		{name: "run bind clean", stage: "run_bind", terminal: "pty-1", run: "run-1"},
		{name: "task unbound", stage: "task_create", terminal: "pty-1", run: "run-1", wantError: "task intent requires terminal and bound Run receipts"},
		{name: "task clean", stage: "task_create", terminal: "pty-1", run: "run-1", bound: true},
		{name: "dispatch missing task", stage: "dispatch", terminal: "pty-1", run: "run-1", bound: true, wantError: "dispatch intent requires terminal, bound Run, and task receipts"},
		{name: "dispatch clean", stage: "dispatch", terminal: "pty-1", run: "run-1", bound: true, task: "task-1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrcaStageReceipts(OrcaStageFacts{
				Stage: tt.stage, Prepared: tt.stage != "worktree_create", TerminalPTYID: tt.terminal,
				RunID: tt.run, RunBound: tt.bound, TaskID: tt.task,
			})
			if tt.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Fatalf("error=%v want %q", err, tt.wantError)
			}
		})
	}
}
