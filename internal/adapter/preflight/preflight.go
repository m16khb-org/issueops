package preflight

import (
	"path/filepath"
	"strings"
	"sync"

	preflightapp "issueops/internal/application/preflight"
)

// GitObserver reads one preflight observation. Run defaults to GitCmd and
// must be safe for concurrent use: the reads after repository discovery are
// independent and run in parallel.
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
	// Porcelain status and the other reads are root-relative wherever Git
	// runs, so they all start from target while the root is discovered.
	var (
		code                                                        int
		root, stderr                                                string
		branch, head, up, statusOut, counts, historyOut, remotesOut string
		wg                                                          sync.WaitGroup
	)
	wg.Go(func() { code, root, stderr = run(target, "rev-parse", "--show-toplevel") })
	wg.Go(func() { branch = out(target, "branch", "--show-current") })
	wg.Go(func() { head = out(target, "rev-parse", "--short", "HEAD") })
	wg.Go(func() {
		up = out(target, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
		if up != "" {
			counts = out(target, "rev-list", "--left-right", "--count", up+"...HEAD")
		}
	})
	wg.Go(func() { statusOut = out(target, "status", "--porcelain=v1", "--branch") })
	wg.Go(func() { historyOut = out(target, historyArgs...) })
	wg.Go(func() { remotesOut = out(target, "remote", "-v") })
	wg.Wait()
	if code != 0 {
		return preflightapp.Observation{ErrorDetail: stderr}
	}
	root = strings.TrimSpace(root)
	var upstream *string
	if up != "" {
		upstream = &up
	}
	status := splitLines(statusOut)
	var ahead, behind *int
	if fields := strings.Fields(counts); len(fields) >= 2 {
		b := atoi(fields[0])
		a := atoi(fields[1])
		behind = &b
		ahead = &a
	}
	history := parseHistory(historyOut)
	return preflightapp.Observation{
		GitOK:            true,
		RepoRoot:         root,
		Branch:           branch,
		Head:             head,
		Upstream:         upstream,
		Ahead:            ahead,
		Behind:           behind,
		StatusLines:      status,
		Remotes:          listRemotes(remotesOut),
		LastCommit:       history.last(),
		RecentCommits:    history.commits(5),
		StyleCommits:     history.commits(10),
		CommitBodies:     history.bodies(),
		CommitPolicyPath: filepath.Join(issueOpsRoot, ".issueops", "COMMIT_POLICY.md"),
	}
}
