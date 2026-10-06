package issueopsapp

import (
	"context"
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	model "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestOrcaBranchPrecheckKeepsCapturedGitCapabilities(t *testing.T) {
	repo := makeGitRepoForContract(t)
	root := t.TempDir()
	if code, _, stderr := preflight.GitCmd(repo, "branch", "occupied"); code != 0 {
		t.Fatal(stderr)
	}
	record := model.IssueOpsRecord{OK: true, SchemaVersion: model.IssueOpsSchemaVersion, ID: "io-0123456789ab", Repo: repo, Branch: "occupied", Phase: model.IssueOpsPhaseImplement, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}
	if _, err := (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	check := newOrcaBranchPrecheck(root)
	t.Chdir(t.TempDir())
	code, err := check.Check(record.ID, "occupied")
	if code != "orca_branch_name_taken" || err == nil || !strings.Contains(err.Error(), "locally") {
		t.Fatalf("code=%q err=%v", code, err)
	}
}
