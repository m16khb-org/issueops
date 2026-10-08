package operationalhealth

import (
	issueopscontract "issueops/internal/contract/issueops"
)

type IssueOpsReader struct {
	StateRoot string
	// Scan visits every stored cycle in id order from one read; err reports a
	// row that could not be decoded.
	Scan             func(stateRoot string, visit func(id string, record issueopscontract.IssueOpsRecord, err error)) error
	ListLeaseHolders func(string) ([]issueopscontract.LeaseHolderIndex, error)
}
