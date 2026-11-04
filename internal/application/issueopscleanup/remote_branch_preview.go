package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
)

type RemoteBranchEnvironment interface {
	RemoteRef(context.Context, string, string) (string, error)
	OriginURL(context.Context, string) (string, error)
	TipReachedBase(context.Context, string, string, string) bool
	Delete(context.Context, string, string, string) error
}

type RemoteBranchPreviewer struct {
	Environment          RemoteBranchEnvironment
	VerifyMergedArtifact func(model.RemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error)
	ObserveArtifact      func(context.Context, string) (domain.ArtifactObservation, error)
}

func (s RemoteBranchPreviewer) Plan(ctx context.Context, record model.Record, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchInventory, model.CleanupRemoteBranchResult) {
	inventory := domain.CleanupRemoteBranchTargets(record)
	facts := domain.CleanupRemoteBranchObservation{MergeConfigured: s.VerifyMergedArtifact != nil}
	s.observeArtifactFacts(ctx, record, req, &facts)
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
			facts.TipSupersedeError = s.verifySuperseding(ctx, record, req.SupersededBy)
			if facts.TipSupersedeError == nil {
				facts.TipSupersededBy = strings.TrimSpace(req.SupersededBy)
			}
		}
	}
	return domain.BuildCleanupRemoteBranchPreview(record, req, inventory, facts)
}

// observeArtifactFacts records the merged-artifact, superseding and identity evidence for the record's remote artifact.
func (s RemoteBranchPreviewer) observeArtifactFacts(ctx context.Context, record model.Record, req model.CleanupRemoteBranchRequest, facts *domain.CleanupRemoteBranchObservation) {
	artifact := record.RemoteArtifact
	if artifact == nil {
		return
	}
	if facts.MergeConfigured {
		facts.Head, facts.MergeError = s.VerifyMergedArtifact(*artifact)
		if facts.MergeError != nil {
			facts.ArtifactSupersedeError = s.verifySuperseding(ctx, record, req.SupersededBy)
			if facts.ArtifactSupersedeError == nil {
				facts.ArtifactSupersededBy = strings.TrimSpace(req.SupersededBy)
			}
		}
	}
	key, err := issueopsremote.CleanupBranchArtifactProject(artifact.URL, artifact.Provider, artifact.Kind)
	if err == nil {
		var origin string
		origin, err = s.Environment.OriginURL(ctx, record.Repo)
		if err == nil {
			err = issueopsremote.ValidateCleanupBranchOrigin(key, origin, artifact.Provider)
		}
	}
	facts.IdentityError = err
}

func (s RemoteBranchPreviewer) verifySuperseding(ctx context.Context, record model.Record, candidate string) error {
	candidate = strings.TrimSpace(candidate)
	if err := domain.ValidateCleanupSupersedeInput(record, candidate, s.ObserveArtifact != nil); err != nil {
		return err
	}
	replacement, err := s.ObserveArtifact(ctx, candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	return domain.ValidateSupersedingArtifact(domain.ArtifactObservation{URL: record.RemoteArtifact.URL, Provider: record.RemoteArtifact.Provider}, replacement)
}
