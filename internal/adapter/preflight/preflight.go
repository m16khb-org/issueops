package preflight

import (
	"path/filepath"
	"strings"

	preflightapp "issueops/internal/application/preflight"
)

// GitObserver reads one preflight observation. Run defaults to GitCmd.
type GitObserver struct {
	Run func(dir string, args ...string) (int, string, string)
}

func (observer GitObserver) Observe(target, issueOpsRoot string) preflightapp.Observation {
	run := observer.Run
	if run == nil {
		run = GitCmd
	}
	out := func(dir string, args ...string) string {
		code, stdout, _ := run(dir, args...)
		if code != 0 {
			return ""
		}
		return strings.TrimSpace(stdout)
	}
	code, root, stderr := run(target, "rev-parse", "--show-toplevel")
	if code != 0 {
		return preflightapp.Observation{ErrorDetail: stderr}
	}
	root = strings.TrimSpace(root)
	branch := out(root, "branch", "--show-current")
	head := out(root, "rev-parse", "--short", "HEAD")
	up := out(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	var upstream *string
	if up != "" {
		upstream = &up
	}
	status := splitLines(out(root, "status", "--porcelain=v1", "--branch"))
	var ahead, behind *int
	if up != "" {
		counts := strings.Fields(out(root, "rev-list", "--left-right", "--count", up+"...HEAD"))
		if len(counts) >= 2 {
			b := atoi(counts[0])
			a := atoi(counts[1])
			behind = &b
			ahead = &a
		}
	}
	history := parseHistory(out(root, historyArgs...))
	return preflightapp.Observation{
		GitOK:            true,
		RepoRoot:         root,
		Branch:           branch,
		Head:             head,
		Upstream:         upstream,
		Ahead:            ahead,
		Behind:           behind,
		StatusLines:      status,
		Remotes:          listRemotes(out, root),
		LastCommit:       history.last(),
		RecentCommits:    history.commits(5),
		StyleCommits:     history.commits(10),
		CommitBodies:     history.bodies(),
		CommitPolicyPath: filepath.Join(issueOpsRoot, ".issueops", "COMMIT_POLICY.md"),
	}
}
