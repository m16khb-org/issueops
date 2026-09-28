package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type PreparationObserver interface {
	Read(context.Context, string) (model.IssueOpsRecord, error)
	Fingerprint(context.Context, model.IssueOpsRecord) string
	Head(context.Context, model.IssueOpsRecord) string
}

type PublicationStore interface {
	WithinTransaction(context.Context, string, func(context.Context) error) error
	Read(context.Context, string) (model.IssueOpsRecord, error)
	ReadPayload(context.Context, string) (contract.IntentPayload, error)
	RecordSnapshot(context.Context, string) (contract.RecordSnapshot, error)
	PayloadRaw(context.Context, string) ([]byte, error)
	DecodeSnapshot(contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error)
	Persist(context.Context, model.IssueOpsRecord, contract.IntentMutation) (model.IssueOpsRecord, error)
}

type PublicationEnvironment interface {
	PathsMatch(string, string) bool
	Timestamp() string
	NewOperationID() (string, error)
}

type RecordTransition func(model.IssueOpsRecord) (model.IssueOpsRecord, error)

type RecordStore interface {
	Update(context.Context, string, RecordTransition) (model.IssueOpsRecord, error)
}

type PublicationAuthority interface {
	Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error
}
