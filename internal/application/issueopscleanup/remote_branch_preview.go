package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type RemoteBranchEnvironment interface {
	RemoteRef(context.Context, string, string) (string, error)
	OriginURL(context.Context, string) (string, error)
	TipReachedBase(context.Context, string, string, string) bool
	Delete(context.Context, string, string, string) error
}

type RemoteBranchPreviewer struct {
	Environment          RemoteBranchEnvironment
	VerifyMergedArtifact func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error)
	ObserveArtifact      func(string) (domain.ArtifactObservation, error)
}

func (s RemoteBranchPreviewer) Plan(ctx context.Context, record model.IssueOpsRecord, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchInventory, model.CleanupRemoteBranchResult) {
	inventory := domain.CleanupRemoteBranchTargets(record)
	facts := domain.CleanupRemoteBranchObservation{MergeConfigured: s.VerifyMergedArtifact != nil}
	if artifact := record.RemoteArtifact; artifact != nil {
		if facts.MergeConfigured {
			facts.Head, facts.MergeError = s.VerifyMergedArtifact(*artifact)
			if facts.MergeError != nil {
				facts.ArtifactSupersedeError = s.verifySuperseding(record, req.SupersededBy)
				if facts.ArtifactSupersedeError == nil {
					facts.ArtifactSupersededBy = strings.TrimSpace(req.SupersededBy)
				}
			}
		}
		key, err := remote.CleanupBranchArtifactProject(artifact.URL, artifact.Provider, artifact.Kind)
		if err == nil {
			var origin string
			origin, err = s.Environment.OriginURL(ctx, record.Repo)
			if err == nil {
				err = remote.ValidateCleanupBranchOrigin(key, origin, artifact.Provider)
			}
		}
		facts.IdentityError = err
	}
	if inventory.Branch != "" {
		facts.RemoteOID, facts.RemoteError = s.Environment.RemoteRef(ctx, record.Repo, inventory.Branch)
	}
	if domain.CleanupRemoteBranchNeedsTipEvidence(facts) {
		if record.BranchPrepare != nil {
			if base := strings.TrimSpace(record.BranchPrepare.BaseBranch); base != "" {
				facts.TipReachedBase = s.Environment.TipReachedBase(ctx, record.Repo, facts.RemoteOID, base)
			}
		}
		if !facts.TipReachedBase {
			facts.TipSupersedeError = s.verifySuperseding(record, req.SupersededBy)
			if facts.TipSupersedeError == nil {
				facts.TipSupersededBy = strings.TrimSpace(req.SupersededBy)
			}
		}
	}
	return domain.BuildCleanupRemoteBranchPreview(record, req, inventory, facts)
}

func (s RemoteBranchPreviewer) verifySuperseding(record model.IssueOpsRecord, candidate string) error {
	candidate = strings.TrimSpace(candidate)
	if err := domain.ValidateCleanupSupersedeInput(record, candidate, s.ObserveArtifact != nil); err != nil {
		return err
	}
	replacement, err := s.ObserveArtifact(candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	return domain.ValidateSupersedingArtifact(domain.ArtifactObservation{URL: record.RemoteArtifact.URL, Provider: record.RemoteArtifact.Provider}, replacement)
}
