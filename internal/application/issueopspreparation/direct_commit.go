package issueopspreparation

import (
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func ValidateDirectCommit(commit DirectCommit) error {
	return preparationdomain.ValidateSelectionReceipt(commit.Selection, commit.Command, commit.Probe, preparationcontract.ModeDirect)
}

func ApplyDirectCommit(current leasecontract.Record, commit DirectCommit) (leasecontract.Record, preparationcontract.Result, error) {
	if current.Execution != nil {
		return current, preparationcontract.Result{}, fmt.Errorf("IssueOps execution is already prepared")
	}
	record := current
	record.WorktreePath = commit.Workspace.Root
	actor := commit.Command.Clone().Actor
	selection := commit.Selection
	record.Execution = &leasecontract.Execution{
		Mode:      preparationcontract.ModeDirect,
		Selection: &selection,
		Workspace: leasecontract.Workspace{
			SourceRoot: commit.Workspace.SourceRoot, Root: commit.Workspace.Root,
			Branch: commit.Workspace.Branch, BaseHead: commit.Workspace.BaseHead,
			ParentWorktree: commit.Workspace.ParentWorktree, Driver: "git", LinkedAt: commit.LinkedAt,
			ArtifactDir: commit.ArtifactDir,
		},
		Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: &actor, ClaimedAt: commit.ClaimedAt},
	}
	result := preparationcontract.Result{
		OK: true, ID: record.ID, RequestedMode: commit.RequestedMode,
		ResolvedMode: preparationcontract.ModeDirect, FallbackCode: commit.FallbackCode,
		Workspace: record.Execution.Workspace, Execution: record.Execution,
		ProbeAttempted: commit.Selection.ProbeAttempted, ProbeAvailable: commit.Selection.ProbeAvailable,
		ProbeReady: commit.Selection.ProbeReady, ProbeCode: commit.Selection.ProbeCode,
		ReadinessFingerprint: commit.Selection.ReadinessFingerprint, ExplicitDirectReason: commit.Selection.ExplicitDirectReason,
	}
	return record, result.Clone(), nil
}
