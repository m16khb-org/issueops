package issueopsowner

import (
	"context"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"strings"
)

func (s Service) Reseal(ctx context.Context, record issueops.IssueOpsRecord) (issueops.ReplacementArtifacts, error) {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return issueops.ReplacementArtifacts{}, nil
	}
	if strings.TrimSpace(record.PlanPath) == "" {
		return issueops.ReplacementArtifacts{}, s.planRequiredError(record, false)
	}
	stagedPlan, err := s.RequirePlan(record)
	if err != nil {
		return issueops.ReplacementArtifacts{}, err
	}
	if s.ReadIssue == nil {
		return issueops.ReplacementArtifacts{}, fmt.Errorf("replacement cannot reseal the owner context without a remote issue reader")
	}
	snapshot, err := s.ReadSnapshot(ctx, record)
	if err != nil {
		return issueops.ReplacementArtifacts{}, fmt.Errorf("replacement stopped because the remote issue could not be read for resealing: %w", err)
	}
	plan, manifest, err := s.MaterializePlan(record)
	if err != nil {
		return issueops.ReplacementArtifacts{}, err
	}
	if plan.Path != record.PlanPath || plan.Digest != stagedPlan.Digest {
		return issueops.ReplacementArtifacts{}, s.planRequiredError(record, false)
	}
	binding := record.Execution.Orca
	artifacts, err := s.Build(record, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: string(issueops.ExecutionModeOrca), OwnerHost: binding.OwnerHost,
		OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
	}, snapshot, manifest)
	if err != nil {
		return issueops.ReplacementArtifacts{}, err
	}
	return issueops.ReplacementArtifacts{
		IssueBodySHA256:   snapshot.Issue.BodySHA256,
		ContextPacketPath: artifacts.PacketPath, ContextPacketSHA256: artifacts.PacketSHA256,
		OwnerPromptPath: artifacts.PromptPath, OwnerPromptSHA256: artifacts.PromptSHA256,
	}, nil
}

func (s Service) Reseed(ctx context.Context, id string, execution issueops.Execution) (issueops.ReplacementArtifacts, error) {
	if execution.Lease.Generation < 2 {
		return issueops.ReplacementArtifacts{}, fmt.Errorf("reseed owner artifacts require a replacement generation")
	}
	record, err := s.ReadRecord(id)
	if err != nil {
		return issueops.ReplacementArtifacts{}, err
	}
	if err := domain.ValidateReplacementGeneration(record, execution.Lease.Generation-1, false); err != nil {
		return issueops.ReplacementArtifacts{}, err
	}
	if record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil || execution.Mode != issueops.ExecutionModeOrca || execution.Orca == nil {
		return issueops.ReplacementArtifacts{}, fmt.Errorf("reseed owner artifacts require an Orca execution")
	}
	record.Execution = &execution
	return s.Reseal(ctx, record)
}
