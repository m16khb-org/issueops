// Package issueopsstatus builds the `issueops status` result by reading a
// stored cycle record and projecting it. It is an application-layer service
// that depends on storage and projection only through ports.
package issueopsstatus

import (
	"context"

	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

// Repository reads the stored issueops record for a cycle ID.
type Repository interface {
	Read(context.Context, string, string) (issueopsstatuscontract.Record, error)
}

type projector interface {
	Project(issueopsstatuscontract.Record) issueopsstatuscontract.Record
}
