package issueopsbodysync

import (
	"context"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type RecordTransition func(model.Record) (model.Record, error)

type Repository interface {
	Read(context.Context, string) (model.Record, error)
	Update(context.Context, string, RecordTransition) (model.Record, error)
}

type Authority interface {
	Authorize(context.Context, model.Record, model.Actor) error
}

type Provider interface {
	Name() string
	port.IssueProviderArtifactBodyReader
	port.IssueProviderArtifactBodyReplacer
	port.IssueProviderChildHierarchyVerifier
}
