package issueopsbranch

import model "issueops/internal/contract/issueops"

type OrcaBranchObservations struct {
	ReadRecord func(string) (model.IssueOpsRecord, error)
	RefOID     func(repo, ref string) (string, bool)
}
