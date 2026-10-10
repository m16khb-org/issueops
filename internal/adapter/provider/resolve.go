// Package provider selects a concrete remote issue provider by name. It lives in
// the adapter layer so the domain and application layers never import concrete
// provider implementations, preserving the hexagonal boundary: they depend only
// on the port.IssueProvider abstraction and receive a resolved provider from callers.
package provider

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"

	"issueops/internal/adapter/provider/github"
	"issueops/internal/adapter/provider/gitlab"
	"issueops/internal/port"
)

// Resolve returns the issue provider registered under name, or an error naming
// the supported providers.
func Resolve(name string) (port.IssueProvider, error) {
	switch name {
	case "github":
		return github.NewProvider(), nil
	case "gitlab":
		return gitlab.NewProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q; supported: github, gitlab", name)
	}
}

// ResolvePullRequestMerger returns the merge capability of the named provider.
func ResolvePullRequestMerger(name string) (port.IssueProviderPullRequestMerger, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	merger, ok := resolved.(port.IssueProviderPullRequestMerger)
	if !ok {
		return nil, fmt.Errorf("provider %q cannot merge pull requests", name)
	}
	return merger, nil
}

func ReadExecutionIssueSnapshot(ctx context.Context, name string, req executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return executionissue.ExecutionIssueSnapshot{}, err
	}
	reader, ok := resolved.(port.ExecutionIssueSnapshotReader)
	if !ok {
		return executionissue.ExecutionIssueSnapshot{}, fmt.Errorf("provider %q cannot read issue snapshots", name)
	}
	return reader.ReadIssueSnapshot(ctx, req)
}
