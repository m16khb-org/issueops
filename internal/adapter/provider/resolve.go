// Package provider selects a concrete remote issue provider by name. It lives in
// the adapter layer so the domain and application layers never import concrete
// provider implementations, preserving the hexagonal boundary: they depend only
// on the port.IssueProvider abstraction and receive a resolved provider from callers.
package provider

import (
	"fmt"

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

// ResolveLabeler returns the label capability of the named provider.
func ResolveLabeler(name string) (port.IssueProviderLabeler, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	labeler, ok := resolved.(port.IssueProviderLabeler)
	if !ok {
		return nil, fmt.Errorf("provider %q cannot add labels", name)
	}
	return labeler, nil
}

// ResolveCommenter returns the comment capability of the named provider.
func ResolveCommenter(name string) (port.IssueProviderCommenter, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	commenter, ok := resolved.(port.IssueProviderCommenter)
	if !ok {
		return nil, fmt.Errorf("provider %q cannot post comments", name)
	}
	return commenter, nil
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
