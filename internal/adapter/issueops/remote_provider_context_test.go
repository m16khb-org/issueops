package issueops

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"
	"testing"

	"issueops/internal/port"
)

type contextPullRequestProvider struct {
	*fakeCompletionProvider
	seen context.Context
}

func (provider *contextPullRequestProvider) CreatePullRequestContext(
	ctx context.Context,
	_ port.IssueProviderCreatePullRequestRequest,
) (executionissue.IssueProviderCreatePullRequestResult, error) {
	provider.seen = ctx
	return executionissue.IssueProviderCreatePullRequestResult{OK: true}, nil
}

func TestCreateRemotePullRequestViaProviderPropagatesContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	provider := &contextPullRequestProvider{fakeCompletionProvider: &fakeCompletionProvider{}}

	result, err := CreateRemotePullRequestViaProviderContext(
		ctx,
		port.IssueProviderCreatePullRequestRequest{},
		provider,
	)

	if err != nil || !result.OK || provider.seen == nil || provider.seen.Value(contextKey{}) != "request" {
		t.Fatalf("result=%+v seen=%v error=%v", result, provider.seen, err)
	}
}
