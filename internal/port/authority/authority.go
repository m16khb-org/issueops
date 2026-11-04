// Package authority declares the ports of the authority use case: grant storage,
// actor and process verification, credential files, and workspace scope
// resolution. Adapters implement them; the application layer depends only on
// these interfaces.
package authority

import (
	"context"

	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

// RecordReader reads committed grant records without creating state. Get reports
// found=false for an absent id.
type RecordReader interface {
	Get(bucket, id string) ([]byte, bool, error)
}

// Repository serializes grant updates. Within passes fn the grant stored under
// key (nil when absent) and persists the record fn returns; a nil result writes
// nothing, and errors from fn are returned unchanged.
type Repository interface {
	Within(ctx context.Context, key string, fn func(*authoritycontract.Record) (*authoritycontract.Record, error)) error
}

// ActorVerifier proves a native actor's identity and returns the verified actor,
// or an error when the claim cannot be established.
type ActorVerifier interface {
	Verify(context.Context, model.NativeActor) (model.VerifiedActor, error)
}

// ProcessInspector observes the live process named by a receipt. Inspect returns
// its status and the receipt as it stands now.
type ProcessInspector interface {
	Inspect(context.Context, model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)
}

// ProcessArgumentsInspector is an optional ProcessInspector capability that
// reads a live process's argv, only while its start identity still matches
// the receipt. Verification of opencode receipts requires it.
type ProcessArgumentsInspector interface {
	Arguments(context.Context, model.NativeProcessReceipt) ([]string, error)
}

// CredentialFiles stores the bearer token of a grant outside the state database.
// Write returns the file path, Read returns the key and token stored at a path,
// and Prune removes superseded files.
type CredentialFiles interface {
	Write(ctx context.Context, key, token string) (path string, err error)
	Read(ctx context.Context, path string) (key, token string, err error)
	// Prune removes the key's credential files other than keepDigest (the
	// token the current grant record names; empty when there is none). Files
	// it removes hold tokens that can no longer authorize.
	Prune(ctx context.Context, key, keepDigest string) error
}

// ScopeResolver resolves workspaceRoot and cwd to the canonical Scope (source
// root and Git common directory) a grant is checked against.
type ScopeResolver interface {
	Resolve(ctx context.Context, workspaceRoot, cwd string) (authoritycontract.Scope, error)
}
