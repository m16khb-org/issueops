package remoteverification

import (
	"context"
	model "issueops/internal/contract/remoteverification"
)

type Reader interface {
	Artifact(context.Context, model.Target) (model.Artifact, error)
	GitHubChild(context.Context, string) error
	GitLabChild(context.Context, model.ChildTarget, bool) (model.TaskMetadata, error)
}
