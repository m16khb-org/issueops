package issueopslease

import (
	"context"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/domain/issueopslease"
)

type Repository interface {
	Update(context.Context, string, RecordValidator, RecordTransition) (RepositoryResult, error)
}

type ClaimRepository interface {
	Claim(context.Context, ClaimRepositoryRequest) (RepositoryResult, error)
}

// ClaimTransaction is used within the repository's one atomic span.
// It exposes observations and persistence while the application orders decisions.
type ClaimTransaction interface {
	Load(string) (Record, error)
	CanonicalCWD(cwd, root string) bool
	CurrentTokenPath(leasecontract.Record) string
	ReadToken(leasecontract.Record, string) (string, error)
	Persist(context.Context, leasecontract.Record) (RepositoryResult, error)
	RemoveToken(string)
}

type ClaimRepositoryRequest struct {
	ID                string
	Generation        uint64
	Actor             issueopslease.Actor
	CWD               string
	TokenFile         string
	ClaimCurrentToken bool
	Clock             Clock
}

type RecordValidator func(Record) error
type RecordTransition func(Record) (Record, error)

type Record struct {
	ID            string
	SourceRoot    string
	CanonicalRoot string
	Lease         leasecontract.Lease
	Stable        leasecontract.Record
}

// RepositoryResult은 같은 transaction에서 저장한 v1 execution projection이다.
// inbound adapter가 후속 status read로 다른 writer의 sidecar를 섞지 않게 한다.
type RepositoryResult struct {
	Record    Record
	Execution leasecontract.Execution
}

type Clock interface{ Now() time.Time }
type ProcessInspector func(context.Context, issueopslease.ProcessReceipt) (string, issueopslease.ProcessReceipt, error)
type CanonicalPathMatcher interface{ Matches(string, string) bool }

type ReseedFence interface {
	Within(context.Context, string, func(context.Context) error) error
}

type ReseedSnapshot struct {
	Record Record
	Raw    []byte
}

type ReseedRepository interface {
	LoadSnapshot(context.Context, string) (ReseedSnapshot, error)
	CommitReseed(context.Context, ReseedSnapshot, Record) (RepositoryResult, error)
}

type ReseedInventory interface {
	Observe(context.Context, leasecontract.Record, issueopslease.Actor) (string, error)
}

type ReseedArtifactReceipt struct {
	TokenSHA256 string
	Receipt     leasecontract.ReseedReceipt
	TargetPaths []string
}

type ReseedArtifacts interface {
	Prepare(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error)
	Rollback(context.Context, ReseedArtifactReceipt) error
	CleanupSuperseded(context.Context, leasecontract.Record) error
}
