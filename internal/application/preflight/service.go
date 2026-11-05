// Package preflight inspects a git repository before an atomic commit and
// reports its state. It is an application-layer service that gets git facts
// through the Observer port.
package preflight

import (
	preflightcontract "issueops/internal/contract/preflight"
	preflightdomain "issueops/internal/domain/preflight"
)

// Observation holds the raw git facts an Observer collected for one target
// path. GitOK is false when the target is not a git repository, in which case
// ErrorDetail carries the reason and the other fields are unset.
type Observation struct {
	GitOK            bool
	ErrorDetail      string
	RepoRoot         string
	Branch           string
	Head             string
	Upstream         *string
	Ahead            *int
	Behind           *int
	StatusLines      []string
	Remotes          []preflightcontract.RemoteInfo
	LastCommit       string
	RecentCommits    []preflightcontract.CommitInfo
	StyleCommits     []preflightcontract.CommitInfo
	CommitBodies     []string
	CommitPolicyPath string
}

// Observer collects the git facts for target. issueOpsRoot locates the project
// commit policy used for style hints.
type Observer interface {
	Observe(target, issueOpsRoot string) Observation
}

// Service turns git observations into the preflight result.
type Service struct{ Observer Observer }

// Check observes target and returns the preflight result: repository state,
// staged/unstaged/untracked files, secret-like paths, and commit style hints.
// A non-git target yields a not_git_repo result with empty lists. List fields
// are never nil.
func (service Service) Check(target, issueOpsRoot string) preflightcontract.Result {
	facts := service.Observer.Observe(target, issueOpsRoot)
	if !facts.GitOK {
		return preflightcontract.Result{
			OK: false, Error: "not_git_repo", Path: target, Detail: facts.ErrorDetail,
			StatusLines: []string{}, Remotes: []preflightcontract.RemoteInfo{}, RecentCommits: []preflightcontract.CommitInfo{},
			StagedFiles: []string{}, UnstagedFiles: []string{}, UntrackedFiles: []string{}, SecretLikePaths: []string{}, Warnings: []string{},
		}
	}
	status := preflightdomain.AnalyzeStatus(facts.StatusLines)
	// Lists are [] when empty so `jq '.staged_files[]'` never meets null.
	return preflightcontract.Result{
		OK:               true,
		RepoRoot:         facts.RepoRoot,
		Branch:           facts.Branch,
		Head:             facts.Head,
		Upstream:         facts.Upstream,
		Ahead:            facts.Ahead,
		Behind:           facts.Behind,
		IsClean:          status.IsClean,
		StatusLines:      list(facts.StatusLines),
		Remotes:          list(facts.Remotes),
		LastCommit:       facts.LastCommit,
		RecentCommits:    list(facts.RecentCommits),
		CommitStyleHints: preflightdomain.CommitStyleHints(facts.StyleCommits, facts.CommitBodies, facts.CommitPolicyPath, facts.RepoRoot),
		StagedFiles:      list(status.Staged),
		UnstagedFiles:    list(status.Unstaged),
		UntrackedFiles:   list(status.Untracked),
		SecretLikePaths:  list(status.SecretLike),
		Warnings:         list(preflightdomain.Warnings(facts.Branch, facts.Upstream != nil, status.SecretLike)),
	}
}

func list[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
