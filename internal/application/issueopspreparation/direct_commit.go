package issueopspreparation

import (
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func ApplyDirectCommit(current leasecontract.Record, commit DirectCommit) (leasecontract.Record, preparationcontract.Result, error) {
	if current.Execution != nil {
		return current, preparationcontract.Result{}, fmt.Errorf("IssueOps execution is already prepared")
	}
	if err := requireArtifactDir(commit.ArtifactDir); err != nil {
		return current, preparationcontract.Result{}, err
	}
	record := current
	record.WorktreePath = commit.Workspace.Root
	actor := commit.Command.Clone().Actor
	record.Execution = &leasecontract.Execution{
		Workspace: leasecontract.Workspace{
			SourceRoot: commit.Workspace.SourceRoot, Root: commit.Workspace.Root,
			Branch: commit.Workspace.Branch, BaseHead: commit.Workspace.BaseHead,
			ParentWorktree: commit.Workspace.ParentWorktree, LinkedAt: commit.LinkedAt,
			ArtifactDir: commit.ArtifactDir,
		},
		Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: &actor, ClaimedAt: commit.ClaimedAt},
	}
	result := preparationcontract.Result{
		OK: true, ID: record.ID, Workspace: record.Execution.Workspace, Execution: record.Execution,
	}
	return record, result.Clone(), nil
}
