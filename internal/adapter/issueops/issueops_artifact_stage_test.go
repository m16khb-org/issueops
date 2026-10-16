package issueops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsartifactdomain "issueops/internal/domain/issueopsartifact"
)

func TestReadLinkedPlanIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*testing.T, *issueopscontract.IssueOpsRecord)
		wantError bool
	}{
		{name: "no durable plan path", wantError: true},
		{
			name: "plan is missing",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "missing.md")
			},
			wantError: true,
		},
		{
			name: "plan is a directory",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = record.WorktreePath
			},
			wantError: true,
		},
		{
			name: "plan is outside worktree",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(t.TempDir(), "plan.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Plan\n")
			},
			wantError: true,
		},
		{
			name: "plan is empty",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "empty.md")
				writePlanArtifactTestFile(t, record.PlanPath, "")
			},
			wantError: true,
		},
		{
			name: "plan is a symlink",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				target := filepath.Join(record.WorktreePath, "target.md")
				writePlanArtifactTestFile(t, target, "# Plan\n")
				record.PlanPath = filepath.Join(record.WorktreePath, "link.md")
				if err := os.Symlink(target, record.PlanPath); err != nil {
					t.Fatal(err)
				}
			},
			wantError: true,
		},
		{
			name: "plan inside worktree",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "plan.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Plan\n")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var record issueopscontract.IssueOpsRecord
			if test.configure != nil {
				test.configure(t, &record)
			}
			identity, err := readLinkedPlanIdentity(record)
			if test.wantError {
				if err == nil {
					t.Fatalf("identity=%+v, want error", identity)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := (linkedPlanIdentity{Path: record.PlanPath, Digest: digestBytes([]byte("# Plan\n"))}); identity != want {
				t.Fatalf("identity=%+v want %+v", identity, want)
			}
		})
	}
}

func TestArtifactStagingIsOpenOnlyBeforeExecutionPrepare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		execution *issueopscontract.Execution
		want      bool
	}{
		{name: "no execution", want: true},
		{name: "released", execution: preparedExecution(issueopscontract.LeaseStatusReleased)},
		{name: "claimable", execution: preparedExecution(issueopscontract.LeaseStatusClaimable)},
		{name: "active", execution: preparedExecution(issueopscontract.LeaseStatusActive)},
		{name: "revoking", execution: preparedExecution(issueopscontract.LeaseStatusRevoking)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := issueopscontract.IssueOpsRecord{Execution: test.execution}
			for _, artifact := range []string{"plan", "spec"} {
				if got := issueopsartifactdomain.CanStage(record, artifact); got != test.want {
					t.Fatalf("CanStage(%s)=%t want %t", artifact, got, test.want)
				}
			}
		})
	}
}

func TestArtifactStagingIsSealedAfterExecutionPrepare(t *testing.T) {
	t.Parallel()

	stateRoot, record := executionPrepareRecord(t)
	worktree := t.TempDir()
	record.WorktreePath = worktree
	record.Execution = preparedExecution(issueopscontract.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = worktree
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-03T00:00:00Z"
	if _, err := writeIssueOps(t.Context(), stateRoot, record); err != nil {
		t.Fatal(err)
	}
	_, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Blocked\n"))
	if err == nil {
		t.Fatal("prepared execution accepted artifact staging")
	}
	var fields interface{ IssueOpsErrorFields() map[string]any }
	if !errors.As(err, &fields) {
		t.Fatalf("error %T does not expose structured fields: %v", err, err)
	}
	got := fields.IssueOpsErrorFields()
	if got["code"] != "artifact_stage_sealed" || !strings.Contains(err.Error(), "artifacts are sealed after execution prepare") {
		t.Fatalf("fields=%+v err=%v", got, err)
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil || len(staged) != 0 {
		t.Fatalf("sealed staging must not store content: staged=%+v err=%v", staged, err)
	}
}

func preparedExecution(status issueopscontract.LeaseStatus) *issueopscontract.Execution {
	execution := &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/source", Root: "/worktree", Branch: "262-plan-readiness",
			BaseHead: strings.Repeat("a", 40), LinkedAt: "2026-08-03T00:00:00Z",
			ArtifactDir: ".issueops/issues/262/artifact",
		},
		Lease: issueopscontract.WriteLease{Generation: 3, Status: status},
	}
	switch status {
	case issueopscontract.LeaseStatusClaimable:
		execution.Lease.ClaimTokenSHA256 = strings.Repeat("b", 64)
	case issueopscontract.LeaseStatusActive, issueopscontract.LeaseStatusRevoking:
		holder := executionActor("codex", "artifact-holder")
		execution.Lease.Holder = &holder
		if status == issueopscontract.LeaseStatusActive {
			execution.Lease.ClaimedAt = "2026-08-03T00:00:00Z"
		}
	}
	return execution
}

func writePlanArtifactTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
