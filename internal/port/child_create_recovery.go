package port

import "context"

type ChildSnapshot struct {
	URL               string
	Title             string
	Body              string
	TypeVerified      bool
	HierarchyVerified bool
	Labels, Assignees []string
}

type IssueProviderChildCreateRecovery interface {
	FindChildCreateCandidates(context.Context, IssueProviderFindIssueCreateCandidatesRequest) (IssueProviderFindIssueCreateCandidatesResult, error)
	ReadChild(context.Context, string, string, string) (ChildSnapshot, error)
	AttachChild(context.Context, string, string, string) error
}
