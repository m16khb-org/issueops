package preflight

import (
	preflightcontract "issueops/internal/contract/preflight"
	preflightdomain "issueops/internal/domain/preflight"
)

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

type Observer interface {
	Observe(target, issueOpsRoot string) Observation
}

type Service struct{ Observer Observer }

func (service Service) Check(target, issueOpsRoot string) preflightcontract.PreflightResult {
	facts := service.Observer.Observe(target, issueOpsRoot)
	if !facts.GitOK {
		return preflightcontract.PreflightResult{
			OK: false, Error: "not_git_repo", Path: target, Detail: facts.ErrorDetail,
			StatusLines: []string{}, Remotes: []preflightcontract.RemoteInfo{}, RecentCommits: []preflightcontract.CommitInfo{},
			StagedFiles: []string{}, UnstagedFiles: []string{}, UntrackedFiles: []string{}, SecretLikePaths: []string{}, Warnings: []string{},
		}
	}
	status := preflightdomain.AnalyzeStatus(facts.StatusLines)
	// Lists are [] when empty so `jq '.staged_files[]'` never meets null.
	return preflightcontract.PreflightResult{
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
