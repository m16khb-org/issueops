package issueopsbodysync

import (
	"context"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type RecordTransition func(model.IssueOpsRecord) (model.IssueOpsRecord, error)

type Repository interface {
	Read(context.Context, string) (model.IssueOpsRecord, error)
	Update(context.Context, string, RecordTransition) (model.IssueOpsRecord, error)
}

type Authority interface {
	Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error
}

type Provider interface {
	Name() string
	port.IssueProviderArtifactBodyReader
	port.IssueProviderArtifactBodyReplacer
	port.IssueProviderChildHierarchyVerifier
}
