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
	CommitStyleHints map[string]any
}

type Observer interface {
	Observe(target, issueOpsRoot string) Observation
}

type Service struct{ Observer Observer }

func (service Service) Check(target, issueOpsRoot string) preflightcontract.PreflightResult {
	facts := service.Observer.Observe(target, issueOpsRoot)
	if !facts.GitOK {
		return preflightcontract.PreflightResult{OK: false, Error: "not_git_repo", Path: target, Detail: facts.ErrorDetail, Upstream: nil, Ahead: nil, Behind: nil}
	}
	status := preflightdomain.AnalyzeStatus(facts.StatusLines)
	return preflightcontract.PreflightResult{
		OK:               true,
		RepoRoot:         facts.RepoRoot,
		Branch:           facts.Branch,
		Head:             facts.Head,
		Upstream:         facts.Upstream,
		Ahead:            facts.Ahead,
		Behind:           facts.Behind,
		IsClean:          status.IsClean,
		StatusLines:      facts.StatusLines,
		Remotes:          facts.Remotes,
		LastCommit:       facts.LastCommit,
		RecentCommits:    facts.RecentCommits,
		CommitStyleHints: facts.CommitStyleHints,
		StagedFiles:      status.Staged,
		UnstagedFiles:    status.Unstaged,
		UntrackedFiles:   status.Untracked,
		SecretLikePaths:  status.SecretLike,
		Warnings:         preflightdomain.Warnings(facts.Branch, facts.Upstream != nil, status.SecretLike),
	}
}
