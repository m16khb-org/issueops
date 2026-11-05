package operationalhealth

import (
	issueopscontract "issueops/internal/contract/issueops"
)

// IssueOpsReader reads persisted IssueOps cycle records under StateRoot. Scan enumerates the
// stored cycles and ListLeaseHolders lists the lease holders indexed for a state root.
type IssueOpsReader struct {
	StateRoot string
	// Scan visits every stored cycle in id order from one read; err reports a
	// row that could not be decoded.
	Scan             func(stateRoot string, visit func(id string, record issueopscontract.Record, err error)) error
	ListLeaseHolders func(string) ([]issueopscontract.LeaseHolderIndex, error)
}
