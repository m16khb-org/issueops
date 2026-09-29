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

func (s Service) Verify(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return s.VerifyContext(context.Background(), req)
}
func (s Service) VerifyContext(ctx context.Context, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
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
func (s Service) mergedArtifact(ctx context.Context, artifact issueopscontract.IssueOpsRemoteArtifactVerification) (model.Artifact, error) {
	target, err := rules.ArtifactTarget(artifact.Provider, artifact.Kind, artifact.URL, true)
	if err != nil {
		return model.Artifact{}, err
	}
	return s.Reader.Artifact(ctx, target)
}
func (s Service) Merged(ctx context.Context, artifact issueopscontract.IssueOpsRemoteArtifactVerification) error {
	_, err := s.MergedHead(ctx, artifact)
	return err
}
func (s Service) MergedHead(ctx context.Context, artifact issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
	live, err := s.mergedArtifact(ctx, artifact)
	if err != nil {
		return issueopscontract.CleanupRemoteBranchArtifactHead{}, err
	}
	if err := rules.RequireMerged(artifact.URL, live.Merged); err != nil {
		return issueopscontract.CleanupRemoteBranchArtifactHead{}, err
	}
	return issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: strings.TrimSpace(live.HeadRefName), HeadRefOID: strings.TrimSpace(live.HeadRefOID), BaseRefName: strings.TrimSpace(live.BaseRefName)}, nil
}

// A failed observation must not be interpreted as an unmerged artifact.
func (s Service) ObserveMerged(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (bool, error) {
	live, err := s.mergedArtifact(context.Background(), artifact)
	if err != nil {
		return false, err
	}
	return live.Merged, nil
}
func (s Service) ObserveTarget(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (string, error) {
	live, err := s.mergedArtifact(context.Background(), artifact)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(live.BaseRefName), nil
}
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
