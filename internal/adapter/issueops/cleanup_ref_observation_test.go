package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/preflight"
)

func TestCleanupFinishRejectsUnknownRefAfterDeleteFailure(t *testing.T) {
	for _, code := range []int{2, 128, -1} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			stateRoot, record, _ := finishTestRecord(t, true)
			git := &realErrorFinishGit{branchOID: "abc123", updateRefOutput: "delete failed"}
			deps := CleanupFinishDeps{Processes: quietCleanupProcesses(), Git: func(dir string, args ...string) (int, string) {
				if args[0] == "show-ref" {
					return code, "ref observation unavailable"
				}
				return git.run(dir, args...)
			}}
			preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
			if err == nil || result.OK || result.RecordDeleted || result.BranchDeleted || result.FailedStep != "branch_delete" {
				t.Fatalf("unknown ref observation permitted deletion: err=%v result=%+v", err, result)
			}
			if _, readErr := ReadIssueOps(stateRoot, record.ID); readErr != nil {
				t.Fatalf("record lost after unknown ref observation: %v", readErr)
			}
		})
	}
}

func TestCleanupAbandonRejectsUnknownRefAfterDeleteFailure(t *testing.T) {
	for _, code := range []int{2, 128, -1} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			fixture := newClaimableExecutionFixture(t, stateRoot, "293-unknown-ref")
			if err := os.Remove(fixture.tokenPath); err != nil {
				t.Fatal(err)
			}
			deleteAttempted := false
			deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: func(dir string, args ...string) (int, string) {
				if args[0] == "update-ref" {
					deleteAttempted = true
					return 1, "delete failed"
				}
				if deleteAttempted && args[0] == "show-ref" {
					return code, "ref observation unavailable"
				}
				exit, stdout, stderr := preflight.GitCmd(dir, args...)
				if exit != 0 {
					return exit, stderr
				}
				return exit, stdout
			}}
			preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
			if err == nil || result.OK || result.RecordDeleted || result.BranchDeleted || result.FailedStep != "branch_delete" || !deleteAttempted {
				t.Fatalf("unknown ref observation permitted deletion: err=%v result=%+v", err, result)
			}
			kept, readErr := ReadIssueOps(stateRoot, fixture.record.ID)
			if readErr != nil {
				t.Fatalf("record lost after unknown ref observation: %v", readErr)
			}
			if kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != "branch_delete" || kept.CleanupAbandonFailure.Fingerprint != preview.Fingerprint {
				t.Fatalf("retry receipt lost: %+v", kept.CleanupAbandonFailure)
			}
			if exit, _, _ := preflight.GitCmd(fixture.record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/"+fixture.record.Branch); exit != 0 {
				t.Fatal("failed deletion must leave the actual branch present")
			}
		})
	}
}
