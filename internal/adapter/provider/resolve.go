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

// capability narrows a resolved provider to one optional port capability, or
// reports that the named provider lacks it.
func capability[T any](name string, resolved port.IssueProvider, ability string) (T, error) {
	capable, ok := resolved.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("provider %q cannot %s", name, ability)
	}
	return capable, nil
}

// ResolveMilestoneSetter returns the milestone capability of the named provider.
func ResolveMilestoneSetter(name string) (port.IssueProviderMilestoneSetter, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderMilestoneSetter](name, resolved, "set milestones")
}

// ResolveAssigner returns the assignee capability of the named provider.
func ResolveAssigner(name string) (port.IssueProviderAssigner, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderAssigner](name, resolved, "add assignees")
}

// ResolveRetitler returns the title capability of the named provider.
func ResolveRetitler(name string) (port.IssueProviderRetitler, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderRetitler](name, resolved, "change titles")
}

// ResolveLabeler returns the label capability of the named provider.
func ResolveLabeler(name string) (port.IssueProviderLabeler, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderLabeler](name, resolved, "add labels")
}

// ResolveCommenter returns the comment capability of the named provider.
func ResolveCommenter(name string) (port.IssueProviderCommenter, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderCommenter](name, resolved, "post comments")
}

// ResolvePullRequestMerger returns the merge capability of the named provider.
func ResolvePullRequestMerger(name string) (port.IssueProviderPullRequestMerger, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return capability[port.IssueProviderPullRequestMerger](name, resolved, "merge pull requests")
}
