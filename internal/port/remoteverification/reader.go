// Package remoteverification defines the outbound port for reading remote
// issue and pull/merge request state during verification.
package remoteverification

import (
	"context"
	model "issueops/internal/contract/remoteverification"
)

// Reader is the outbound port that reads remote GitHub/GitLab state for
// verification: the linked artifact, a GitHub child issue (existence check),
// and GitLab child task metadata.
type Reader interface {
	Artifact(context.Context, model.Target) (model.Artifact, error)
	GitHubChild(context.Context, string) error
	GitLabChild(context.Context, model.ChildTarget, bool) (model.TaskMetadata, error)
}
