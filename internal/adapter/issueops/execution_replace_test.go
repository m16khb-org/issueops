package issueops

import (
	"context"
	app "issueops/internal/application/issueopsowner"
	executionissue "issueops/internal/contract/executionissue"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestReplacementResealRequiresExistingPlanIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*testing.T, *issueops.IssueOpsRecord, string)
		wantError bool
	}{
		{name: "missing durable plan path", wantError: true},
		{name: "outside durable plan path", wantError: true, configure: func(t *testing.T, record *issueops.IssueOpsRecord, plan string) {
			record.PlanPath = filepath.Join(t.TempDir(), "outside.md")
			writePlanArtifactTestFile(t, record.PlanPath, plan)
		}},
		{name: "durable plan digest mismatch", wantError: true, configure: func(t *testing.T, record *issueops.IssueOpsRecord, _ string) {
			record.PlanPath = filepath.Join(record.WorktreePath, "plans", "linked.md")
			writePlanArtifactTestFile(t, record.PlanPath, "# Different plan\n")
		}},
		{name: "matching durable plan", configure: func(t *testing.T, record *issueops.IssueOpsRecord, plan string) {
			record.PlanPath = filepath.Join(record.WorktreePath, "plans", "linked.md")
			writePlanArtifactTestFile(t, record.PlanPath, plan)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			const plan = "# Replacement plan\n"
			if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
				t.Fatal(err)
			}
			worktree := t.TempDir()
			record.WorktreePath = worktree
			record.Execution = &issueops.Execution{
				Mode: issueops.ExecutionModeOrca,
				Workspace: issueops.Workspace{
					SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
					BaseHead: record.BranchPrepare.BaseSHA, Driver: "orca", ArtifactDir: app.OwnerArtifactDir(record),
				},
				Lease: issueops.WriteLease{Generation: 2, Status: issueops.LeaseStatusClaimable},
				Orca: &issueops.OrcaBinding{
					RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1,
					OwnerHost: "codex", OwnerModel: "gpt-6-astra", OwnerEffort: "xhigh",
					TaskID: "task", DispatchID: "dispatch",
					RunID:                   "run_issueops_1",
					ArtifactIdentityVersion: 1,
					IssueBodySHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					ContextPacketSHA256:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					OwnerPromptSHA256:       "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				},
				Selection: selectionFixture(issueops.ExecutionModeOrca),
			}
			if test.configure != nil {
				test.configure(t, &record, plan)
			}
			token, _, err := createClaimToken(record)
			if err != nil {
				t.Fatal(err)
			}
			record.Execution.Lease.ClaimTokenSHA256 = tokenSHA256(token)
			issueBody := "## Acceptance\n- AC-01 reseal plan\n\n## Verification\n```bash\ngo test ./... -count=1\n```\n"
			readIssue := func(_ context.Context, _ string, request executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error) {
				return executionissue.ExecutionIssueSnapshot{URL: request.URL, Body: issueBody}, nil
			}

			reseal, err := ownerContextForTest(stateRoot, readIssue).Reseal(context.Background(), record)
			if test.wantError {
				if err == nil {
					t.Fatalf("reseal=%+v, want plan identity failure", reseal)
				}
				fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
				if !ok || fields.IssueOpsErrorFields()["code"] != "orca_plan_artifact_required" {
					t.Fatalf("error=%T %v want orca_plan_artifact_required", err, err)
				}
				if record.PlanPath == "" {
					invented := filepath.Join(worktree, filepath.FromSlash(sealedArtifactDir(record)), "plan.md")
					if _, statErr := os.Lstat(invented); !os.IsNotExist(statErr) {
						t.Fatalf("replacement invented durable plan %q: %v", invented, statErr)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if reseal.ContextPacketSHA256 == "" || reseal.OwnerPromptSHA256 == "" {
				t.Fatalf("incomplete reseal=%+v", reseal)
			}
		})
	}
}
