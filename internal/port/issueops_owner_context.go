package port

import (
	"issueops/internal/contract/issueops"
)

type OwnerContextFiles interface {
	Paths(issueops.IssueOpsRecord) issueops.OwnerPaths
	RegularFiles(string, []string) []string
	Write(string, string, []byte) error
	ReadStaged(string) (map[string]string, error)
	ReadLinkedPlan(issueops.IssueOpsRecord) (issueops.OwnerPlanIdentity, error)
	Materialize(issueops.IssueOpsRecord) (map[string]string, error)
	SamePath(string, string) bool
	CreateOrAdoptToken(issueops.IssueOpsRecord) (string, error)
	TokenPath(issueops.IssueOpsRecord) string
}
