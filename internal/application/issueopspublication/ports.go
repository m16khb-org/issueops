package issueopspublication

import (
	"context"

	contract "issueops/internal/contract/issueopspublication"
)

type preparer interface {
	Prepare(context.Context, contract.CreateCommand) (contract.PreparedCreate, error)
}

// Repository is the durable journal of publication intents: it begins an intent before the provider call,
// records failures and completions afterward, and exposes the latest snapshot for reconcile and reporting.
type Repository interface {
	BeginCreate(context.Context, contract.PreparedCreate) (contract.Intent, error)
	LoadIntent(context.Context, string) (contract.Intent, error)
	MarkRetry(context.Context, contract.Intent) (contract.Intent, error)
	RecordFailure(context.Context, contract.Intent, contract.InvocationState, string, error) error
	Complete(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error)
	CompleteNotInvoked(context.Context, contract.Intent, error) (contract.RecordSnapshot, error)
	Latest(context.Context, string) (contract.RecordSnapshot, error)
}

// Provider is the remote issue/PR host. Create performs the single remote create and reports whether the
// invocation happened; Inspect lists remote candidates for an intent so reconcile can adopt one.
type Provider interface {
	Create(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error)
	Inspect(context.Context, contract.Intent) (contract.Inventory, bool, error)
}

type verifier interface {
	VerifyCandidate(context.Context, contract.Intent, contract.Candidate) error
	VerifyLive(context.Context, contract.Intent, string) error
}
