package issueopslease

import (
	"context"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/domain/issueopslease"
)

// Repository is the persistence port for lease transitions that read, validate, and rewrite one record atomically.
type Repository interface {
	Update(context.Context, string, RecordValidator, RecordTransition) (RepositoryResult, error)
}

type claimRepository interface {
	Claim(context.Context, ClaimRepositoryRequest) (RepositoryResult, error)
}

// claimTransaction is used within the repository's one atomic span.
// It exposes observations and persistence while the application orders decisions.
type claimTransaction interface {
	Load(string) (Record, error)
	CanonicalCWD(cwd, root string) bool
	CurrentTokenPath(leasecontract.Record) string
	ReadToken(leasecontract.Record, string) (string, error)
	Persist(context.Context, leasecontract.Record) (RepositoryResult, error)
	RemoveToken(string)
}

// ClaimRepositoryRequest is the input of a repository-level claim: the lease ID and Generation, the verified Actor, the CWD to check against the canonical root, the claim token source (TokenFile or ClaimCurrentToken), and the Clock that stamps the claim.
type ClaimRepositoryRequest struct {
	ID                string
	Generation        uint64
	Actor             issueopslease.Actor
	CWD               string
	TokenFile         string
	ClaimCurrentToken bool
	Clock             clock
}

// RecordValidator checks a loaded record and returns an error to reject the transition before it changes anything.
type RecordValidator func(Record) error

// RecordTransition returns the record that replaces the loaded one, or an error to abort the update.
type RecordTransition func(Record) (Record, error)

// Record is a lease cycle record as the repository loads it: its ID, the SourceRoot and CanonicalRoot of the workspace, the current Lease, and the Stable persisted record.
type Record struct {
	ID            string
	SourceRoot    string
	CanonicalRoot string
	Lease         leasecontract.Lease
	Stable        leasecontract.Record
}

// RepositoryResult 타입은 같은 transaction에서 저장한 v1 execution projection이다.
// inbound adapter가 후속 status read로 다른 writer의 sidecar를 섞지 않게 한다.
type RepositoryResult struct {
	Record    Record
	Execution leasecontract.Execution
}

type clock interface{ Now() time.Time }

// ProcessInspector observes one process receipt and returns its liveness status together with the receipt as currently observed.
type ProcessInspector func(context.Context, issueopslease.ProcessReceipt) (string, issueopslease.ProcessReceipt, error)
type canonicalPathMatcher interface{ Matches(string, string) bool }

type reseedFence interface {
	Within(context.Context, string, func(context.Context) error) error
}

// ReseedSnapshot is a loaded record paired with its Raw stored bytes; CommitReseed receives it together with the replacement record.
type ReseedSnapshot struct {
	Record Record
	Raw    []byte
}

// ReseedRepository is the persistence port for reseed: LoadSnapshot reads the record with its raw bytes, and CommitReseed replaces it only if the snapshot is still current.
type ReseedRepository interface {
	LoadSnapshot(context.Context, string) (ReseedSnapshot, error)
	CommitReseed(context.Context, ReseedSnapshot, Record) (RepositoryResult, error)
}

type reseedInventory interface {
	Observe(context.Context, leasecontract.Record, issueopslease.Actor) (string, error)
}

// ReseedArtifactReceipt describes the replacement artifacts prepared for a reseed: the claim token's SHA-256, the Receipt reported to the caller, and the TargetPaths of those artifacts.
type ReseedArtifactReceipt struct {
	TokenSHA256 string
	Receipt     leasecontract.ReseedReceipt
	TargetPaths []string
}

type reseedArtifacts interface {
	Prepare(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error)
	Rollback(context.Context, ReseedArtifactReceipt) error
	CleanupSuperseded(context.Context, leasecontract.Record) error
}
