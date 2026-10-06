package authority

import (
	"context"

	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

type RecordReader interface {
	Get(bucket, id string) ([]byte, bool, error)
}

type Repository interface {
	Within(ctx context.Context, key string, fn func(*authoritycontract.Record) (*authoritycontract.Record, error)) error
}

type ActorVerifier interface {
	Verify(context.Context, model.NativeActor) (model.VerifiedActor, error)
}

type ProcessInspector interface {
	Inspect(context.Context, model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)
}

type CredentialFiles interface {
	Write(ctx context.Context, key, token string) (path string, err error)
	Read(ctx context.Context, path string) (key, token string, err error)
}

type ScopeResolver interface {
	Resolve(ctx context.Context, workspaceRoot, cwd string) (authoritycontract.Scope, error)
}
