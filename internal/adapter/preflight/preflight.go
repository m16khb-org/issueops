package preflight

import (
	"strings"

	preflightapp "issueops/internal/application/preflight"
	preflightcontract "issueops/internal/contract/preflight"
)

func GitPreflight(target, issueOpsRoot string) preflightcontract.PreflightResult {
	return (preflightapp.Service{Observer: gitObserver{}}).Check(target, issueOpsRoot)
}

type gitObserver struct{}

func (gitObserver) Observe(target, issueOpsRoot string) preflightapp.Observation {
	code, root, stderr := GitCmd(target, "rev-parse", "--show-toplevel")
	if code != 0 {
		return preflightapp.Observation{ErrorDetail: stderr}
	}
	root = strings.TrimSpace(root)
	branch := GitOut(root, "branch", "--show-current")
	head := GitOut(root, "rev-parse", "--short", "HEAD")
	up := GitOut(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	var upstream *string
	if up != "" {
		upstream = &up
	}
	status := splitLines(GitOut(root, "status", "--porcelain=v1", "--branch"))
	var ahead, behind *int
	if up != "" {
		counts := strings.Fields(GitOut(root, "rev-list", "--left-right", "--count", up+"...HEAD"))
		if len(counts) >= 2 {
			b := atoi(counts[0])
			a := atoi(counts[1])
			behind = &b
			ahead = &a
		}
	}
	return preflightapp.Observation{
		GitOK:            true,
		RepoRoot:         root,
		Branch:           branch,
		Head:             head,
		Upstream:         upstream,
		Ahead:            ahead,
		Behind:           behind,
		StatusLines:      status,
		Remotes:          listRemotes(root),
		LastCommit:       GitOut(root, "log", "-1", "--pretty=format:%h %s"),
		RecentCommits:    recentCommits(root, 5),
		CommitStyleHints: commitStyleHints(root, issueOpsRoot, 10),
	}
}
