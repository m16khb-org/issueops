package issueopsowner

import (
	"context"
	"encoding/json"
	"fmt"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func (s Service) ReadPreparationEvidence(ctx context.Context, snapshot preparationcontract.Snapshot) (preparationcontract.OwnerEvidence, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	if _, err := s.RequirePlan(record); err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	owner, err := s.ReadSnapshot(ctx, record)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	identity, err := preparationdomain.PrepareIssueIdentity(snapshot.Record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(snapshot.Record.BranchPrepare))
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	return preparationcontract.OwnerEvidence{
		IssueURL: owner.Issue.URL, IssueBody: owner.Issue.Body, BodySHA256: owner.Issue.BodySHA256,
		Provider: identity.Provider, Issue: identity.Issue,
	}, nil
}

func (s Service) Prepare(
	ctx context.Context,
	snapshot preparationcontract.Snapshot,
	command preparationcontract.Command,
	intent preparationcontract.Intent,
	receipt preparationcontract.IntentReceipt,
) (preparationcontract.OwnerArtifacts, error) {
	if receipt.Workspace == nil {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
	}
	got := receipt.Workspace
	sourceMatches := s.Files.SamePath(got.Workspace.SourceRoot, intent.Workspace.SourceRoot)
	rootMatches := false
	if sourceMatches {
		rootMatches = s.Files.SamePath(got.Workspace.Root, intent.Workspace.Root)
	}
	if !domain.OwnerWorkspaceMatches(sourceMatches, rootMatches, got.Workspace.Branch == intent.Workspace.Branch, got.Workspace.BaseHead == intent.Workspace.BaseHead, got.Workspace.Driver, got.RuntimeID, got.RepoID, got.WorktreeID) {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
	}
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	record.WorktreePath = got.Workspace.Root
	record.Execution.Workspace = issueops.Workspace{SourceRoot: got.Workspace.SourceRoot, Root: got.Workspace.Root, Branch: got.Workspace.Branch, BaseHead: got.Workspace.BaseHead, ParentWorktree: got.Workspace.ParentWorktree, Driver: got.Workspace.Driver, LinkedAt: intent.StartedAt, ArtifactDir: OwnerArtifactDir(record)}
	owner, err := s.ReadSnapshot(ctx, record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	if owner.Issue.BodySHA256 != intent.IssueBodySHA256 {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("remote issue body drifted before owner launch recovery")
	}
	tokenSHA256, err := s.Files.CreateOrAdoptToken(record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	plan, manifest, err := s.MaterializePlan(record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	record.PlanPath = plan.Path
	artifacts, err := s.Build(record, issueops.ExecutionPrepareRequest{
		ID: command.ID, Mode: preparationcontract.ModeOrca,
		OwnerHost: command.OwnerHost, OwnerModel: command.OwnerModel, OwnerEffort: command.OwnerEffort,
	}, owner, manifest)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	return preparationcontract.OwnerArtifacts{
		PlanPath:       plan.Path,
		ClaimTokenPath: s.Files.TokenPath(record), ClaimTokenSHA256: tokenSHA256,
		ContextPacketPath: artifacts.PacketPath, ContextPacketSHA256: artifacts.PacketSHA256,
		OwnerPromptPath: artifacts.PromptPath, OwnerPromptSHA256: artifacts.PromptSHA256,
	}, nil
}

func executionPreparationCoreRecord(snapshot preparationcontract.Snapshot) (issueops.IssueOpsRecord, error) {
	if len(snapshot.RecordRaw) == 0 {
		return issueops.IssueOpsRecord{}, fmt.Errorf("preparation raw record snapshot is required")
	}
	var record issueops.IssueOpsRecord
	if err := json.Unmarshal(snapshot.RecordRaw, &record); err != nil {
		return issueops.IssueOpsRecord{}, err
	}
	return record, nil
}
