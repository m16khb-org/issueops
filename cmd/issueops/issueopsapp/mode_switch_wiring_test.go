package issueopsapp

import (
	"context"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeSwitchKeepsCapturedGitCapabilities(t *testing.T) {
	repo := makeGitRepoForContract(t)
	stateRoot := t.TempDir()
	branch := "301-mode-switch"
	worktree := filepath.Join(repo+".worktrees", branch)
	record := model.IssueOpsRecord{OK: true, SchemaVersion: model.IssueOpsSchemaVersion, ID: "io-0123456789ab", Repo: repo, Branch: branch, Phase: model.IssueOpsPhaseImplement, WorktreePath: worktree, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-01-01T00:00:00Z"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusReleased}}}
	if _, err := (core.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	runners := newIssueOpsExecutionRunners()
	t.Chdir(t.TempDir())
	got, err := runners.SwitchExecutionMode(context.Background(), stateRoot, model.ExecutionSwitchModeRequest{ID: record.ID, Mode: "orca"})
	if err != nil || !got.OK || got.Fingerprint == "" {
		t.Fatalf("preview=%+v err=%v", got, err)
	}
}
