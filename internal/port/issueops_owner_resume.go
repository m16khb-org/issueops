package port

import "issueops/internal/contract/issueops"

type OwnerResumeFiles interface {
	Paths(issueops.IssueOpsRecord) issueops.OwnerPaths
	TokenPath(issueops.IssueOpsRecord) string
	ReadArtifact(string, string) ([]byte, error)
	ArtifactPath(issueops.IssueOpsRecord, string) string
	ReadLinkedPlan(issueops.IssueOpsRecord) (issueops.OwnerPlanIdentity, error)
	SamePath(string, string) bool
}
