package issueops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"issueops/internal/contract/issueops"
)

// abandonResidueFixture는 execution 없이 record가 link한 실제 워크트리·브랜치를
// 가진 레코드다(#342/#433 residue 선례). 프로세스 표면만 fake로 바꾼다.
func abandonResidueFixture(t *testing.T) (string, issueops.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "477-abandon-processes")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = nil
		rec.WorktreePath = fixture.worktree
	})
	return stateRoot, fixture.record, fixture.worktree
}

// AC-01: abandon preview도 점유자를 나열하되 막지 않는다.
func TestCleanupAbandonPreviewListsOccupantsWithoutBlocking(t *testing.T) {
	stateRoot, record, _ := abandonResidueFixture(t)
	deps := CleanupAbandonDeps{Processes: worldCleanupProcesses(occupiedWorld(t, codexOccupant()))}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("occupants must not block abandon preview: %v missing=%v", err, result.Missing)
	}
	if len(result.WorkspaceProcesses) != 1 || result.WorkspaceProcesses[0].PID != 4321 {
		t.Fatalf("preview must list what apply will stop: %+v", result)
	}
	quiet, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil || quiet.Fingerprint == result.Fingerprint {
		t.Fatalf("fingerprint must bind the occupancy: err=%v same=%v", err, quiet.Fingerprint == result.Fingerprint)
	}
}

// AC-03: abandon apply는 워크트리를 지우기 전에 점유자를 닫는다.
func TestCleanupAbandonApplyStopsOccupantsBeforeWorktreeRemove(t *testing.T) {
	stateRoot, record, worktree := abandonResidueFixture(t)
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGHUP
	processes := worldCleanupProcesses(world)
	presentAtSignal := false
	processes.Signal = func(pid int, sig syscall.Signal) error {
		if _, err := os.Stat(worktree); err == nil {
			presentAtSignal = true
		}
		return world.signal(pid, sig)
	}
	deps := CleanupAbandonDeps{Processes: processes}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !applied.RecordDeleted || !applied.WorktreeRemoved {
		t.Fatalf("apply must stop occupants and then remove the worktree: err=%v result=%+v", err, applied)
	}
	if !presentAtSignal {
		t.Fatal("signals must be sent while the worktree still exists (stop runs before worktree remove)")
	}
	if len(applied.WorkspaceProcessesStopped) != 1 {
		t.Fatalf("apply must report the stopped inventory: %+v", applied)
	}
	if _, err := os.Stat(worktree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("worktree must be removed after the stop step: %v", err)
	}
}

// AC-04: 점유가 남으면 workspace_processes_stop에서 멈추고 레코드와 워크트리를
// 보존한다. 그 failure receipt는 다음 preview를 cleanup_failure_inventory로 막지 않는다.
func TestCleanupAbandonApplyFailsClosedWithWorkspaceProcessesStop(t *testing.T) {
	stateRoot, record, worktree := abandonResidueFixture(t)
	world := occupiedWorld(t, codexOccupant()) // never dies
	deps := CleanupAbandonDeps{Processes: worldCleanupProcesses(world)}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree must survive a failed stop: %v", err)
	}
	if !strings.Contains(result.NextCommand, "--preview") {
		t.Fatalf("recovery must go through a fresh preview: %q", result.NextCommand)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("failure receipt must name the stop step and stay readable: err=%v failure=%+v", err, kept.CleanupAbandonFailure)
	}
	again, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil || containsString(again.Missing, "cleanup_failure_inventory") {
		t.Fatalf("a stop failure must be re-previewable, not a permanent inventory mismatch: err=%v missing=%v", err, again.Missing)
	}
}

// pr-review #478 finding 1: #433이 허용하는 worktree-only 잔여(branch 부재)에서 ①′가
// 실패하면 receipt는 WorktreeHead만 갖는다. 그 receipt가 다음 preview를
// cleanup_failure_inventory로 영구히 막으면 안 된다.
func TestCleanupAbandonStopFailureInWorktreeOnlyResidueIsRePreviewable(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "canonical-worktree")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		rec.WorktreePath = root
	})
	git := &asymmetricAbandonGit{root: root, branch: record.Branch, head: "abc123"}
	deps := CleanupAbandonDeps{
		Processes: worldCleanupProcesses(occupiedWorld(t, codexOccupant())), // never dies
		Git:       git.run,
	}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, "branch ref was removed elsewhere"), deps)
	if err != nil || !preview.WorktreePresent || preview.BranchPresent {
		t.Fatalf("worktree-only residue must preview: err=%v result=%+v", err, preview)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.BranchOID != "" || kept.CleanupAbandonFailure.WorktreeHead == "" {
		t.Fatalf("receipt must record the worktree-only shape: err=%v failure=%+v", err, kept.CleanupAbandonFailure)
	}
	again, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, "branch ref was removed elsewhere"), deps)
	if err != nil || containsString(again.Missing, "cleanup_failure_inventory") || again.Fingerprint == "" {
		t.Fatalf("a worktree-only stop failure must be re-previewable: err=%v missing=%v", err, again.Missing)
	}
}
