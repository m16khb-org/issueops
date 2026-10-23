package issueopsinventory

import (
	"context"
	"time"

	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

type Repository interface {
	ScanEach(
		context.Context,
		string,
		func(issueopsinventorycontract.Record) error,
	) ([]issueopsinventorycontract.RecordDiagnostic, error)
}

type Clock interface {
	Now() time.Time
}

// PathNormalizer maps a path to its repository identity. Normalize keeps the
// lexical form callers store and print; Canonical resolves symlinks and is used
// only to decide whether two identities name the same directory.
type PathNormalizer interface {
	Normalize(string) string
	Canonical(string) string
}
