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

type PathNormalizer interface {
	Normalize(string) string
}
