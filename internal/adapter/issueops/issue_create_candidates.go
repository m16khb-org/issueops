package issueops

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

// IssueCreateCandidateSource finds issues that an interrupted issue create may already
// have produced. Resolve maps a provider name to its issue provider.
type IssueCreateCandidateSource struct {
	Resolve func(string) (port.IssueProvider, error)
}

// Find asks the named provider for issue-create candidates. A provider that does not
// support create reconciliation is an error.
func (s IssueCreateCandidateSource) Find(ctx context.Context, provider string, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	resolved, err := s.Resolve(provider)
	if err != nil {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, err
	}
	reconciler, ok := resolved.(port.IssueProviderIssueCreateReconciler)
	if !ok {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, fmt.Errorf("provider %s does not support issue create reconciliation", provider)
	}
	return reconciler.FindIssueCreateCandidates(ctx, req)
}

var _ application.IssueCandidateSource = IssueCreateCandidateSource{}
