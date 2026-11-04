package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type PreparationObserver interface {
	Read(context.Context, string) (model.Record, error)
	Fingerprint(context.Context, model.Record) string
	Head(context.Context, model.Record) string
}

type PublicationStore interface {
	WithinTransaction(context.Context, string, func(context.Context) error) error
	Read(context.Context, string) (model.Record, error)
	ReadPayload(context.Context, string) (contract.IntentPayload, error)
	RecordSnapshot(context.Context, string) (contract.RecordSnapshot, error)
	PayloadRaw(context.Context, string) ([]byte, error)
	DecodeSnapshot(contract.Intent) (model.Record, contract.IntentPayload, error)
	Persist(context.Context, model.Record, contract.IntentMutation) (model.Record, error)
}

type PublicationEnvironment interface {
	PathsMatch(string, string) bool
	Timestamp() string
	NewOperationID() (string, error)
}

type RecordTransition func(model.Record) (model.Record, error)

type RecordStore interface {
	Update(context.Context, string, RecordTransition) (model.Record, error)
}

type Authority interface {
	Authorize(context.Context, model.Record, model.Actor) error
}
