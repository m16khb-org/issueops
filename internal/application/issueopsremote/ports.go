package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type PreparationObserver interface {
	NormalizeActor(context.Context, contract.Actor) (contract.Actor, error)
	Read(context.Context, string) (model.IssueOpsRecord, error)
	Authorize(context.Context, model.IssueOpsRecord, contract.CreateCommand) error
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
	Authorize(context.Context, model.IssueOpsRecord, contract.CreateCommand) error
	PathsMatch(string, string) bool
	Timestamp() string
	NewOperationID() (string, error)
}

type IssueIntentTransition func(model.IssueOpsRecord) (model.IssueOpsRecord, error)

type IssueIntentStore interface {
	Update(context.Context, string, IssueIntentTransition) (model.IssueOpsRecord, error)
}
