package operationalhealth

import (
	issueopscontract "issueops/internal/contract/issueops"
)

type IssueOpsReader struct {
	StateRoot        string
	ListIDs          func(string) ([]string, error)
	ListLeaseHolders func(string) ([]issueopscontract.LeaseHolderIndex, error)
	Read             func(string, string) (issueopscontract.IssueOpsRecord, error)
}
