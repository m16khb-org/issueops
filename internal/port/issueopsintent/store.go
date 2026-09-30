package issueopsintent

import model "issueops/internal/contract/issueops"

type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Now        func() string
}
