// Package remoteverification is the application layer that verifies remote issue, pull
// request, merge request, and child-task artifacts through a provider reader. Matching
// rules live in the domain; provider access comes in through the Reader port.
package remoteverification

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	model "issueops/internal/contract/remoteverification"
	rules "issueops/internal/domain/remoteverification"
	verificationport "issueops/internal/port/remoteverification"
	"strings"
	"time"
)

// Service binds each caller to its provider reader; no process-global callbacks.
type Service struct{ Reader verificationport.Reader }

// Verify checks the remote artifact in req with a background context. See VerifyContext.
func (s Service) Verify(req issueopscontract.RemoteArtifactVerificationRequest) error {
	return s.VerifyContext(context.Background(), req)
}

// VerifyContext reads the artifact named by req from its provider within a 30 second
// timeout and validates its URL, labels, and assignees. A nil ctx is treated as
// context.Background.
func (s Service) VerifyContext(ctx context.Context, req issueopscontract.RemoteArtifactVerificationRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := rules.ArtifactTarget(req.Provider, req.Kind, req.URL, false)
	if err != nil {
		return err
	}
	artifact, err := s.Reader.Artifact(ctx, target)
	if err != nil {
		return err
	}
	return rules.ValidateArtifact(req.URL, req.Labels, req.Assignees, artifact)
}
func (s Service) mergedArtifact(ctx context.Context, artifact issueopscontract.RemoteArtifactVerification) (model.Artifact, error) {
	target, err := rules.ArtifactTarget(artifact.Provider, artifact.Kind, artifact.URL, true)
	if err != nil {
		return model.Artifact{}, err
	}
	return s.Reader.Artifact(ctx, target)
}

// Merged returns nil only when the live remote artifact is confirmed merged.
func (s Service) Merged(ctx context.Context, artifact issueopscontract.RemoteArtifactVerification) error {
	_, err := s.MergedHead(ctx, artifact)
	return err
}

// MergedHead returns the live head and base refs of the remote artifact, trimmed of
// whitespace, and fails unless the artifact is confirmed merged.
func (s Service) MergedHead(ctx context.Context, artifact issueopscontract.RemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
	live, err := s.mergedArtifact(ctx, artifact)
	if err != nil {
		return issueopscontract.CleanupRemoteBranchArtifactHead{}, err
	}
	if err := rules.RequireMerged(artifact.URL, live.Merged); err != nil {
		return issueopscontract.CleanupRemoteBranchArtifactHead{}, err
	}
	return issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: strings.TrimSpace(live.HeadRefName), HeadRefOID: strings.TrimSpace(live.HeadRefOID), BaseRefName: strings.TrimSpace(live.BaseRefName)}, nil
}

// ObserveMerged reports whether the live remote artifact is merged. A failed observation
// returns an error and must not be interpreted as an unmerged artifact.
func (s Service) ObserveMerged(artifact issueopscontract.RemoteArtifactVerification) (bool, error) {
	live, err := s.mergedArtifact(context.Background(), artifact)
	if err != nil {
		return false, err
	}
	return live.Merged, nil
}

// ObserveTarget returns the trimmed base ref name of the live remote artifact.
func (s Service) ObserveTarget(artifact issueopscontract.RemoteArtifactVerification) (string, error) {
	live, err := s.mergedArtifact(context.Background(), artifact)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(live.BaseRefName), nil
}

// Child verifies that the child task at childURL exists on its provider. GitLab work
// items fall back to the issue API and are additionally validated as tasks; unknown
// providers are accepted without a remote call.
func (s Service) Child(childURL string) error {
	target, err := rules.ChildTarget(childURL)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch target.Provider {
	case "github":
		return s.Reader.GitHubChild(ctx, target.URL)
	case "gitlab":
		if target.Kind == "work_items" {
			if _, err := s.Reader.GitLabChild(ctx, target, false); err == nil {
				return nil
			}
			target.Kind = "issues"
			metadata, err := s.Reader.GitLabChild(ctx, target, true)
			if err != nil {
				return err
			}
			return rules.ValidateTask(target.IID, metadata)
		}
		_, err := s.Reader.GitLabChild(ctx, target, false)
		return err
	default:
		return nil
	}
}
