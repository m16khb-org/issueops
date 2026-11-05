package issueops

import (
	"strings"

	"issueops/internal/adapter/issueops/readinesspaths"
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

// ReadinessGit answers the git questions of readiness checks. Run returns exit code, stdout and
// stderr; Output returns trimmed stdout, or an empty string when git fails.
type ReadinessGit struct {
	Run    func(string, ...string) (int, string, string)
	Output func(string, ...string) string
}

// NewReadinessGitObservations dedupes the branch and porcelain status reads of
// one readiness operation; all other probes stay fresh, and a fetch starts a
// new phase.
func NewReadinessGitObservations(run func(string, ...string) (int, string, string)) (ReadinessGit, func()) {
	type result struct {
		code        int
		out, stderr string
	}
	type key struct {
		root string
		argv [2]string
	}
	results := map[key]result{}
	reset := func() { clear(results) }
	observe := func(root string, args ...string) (int, string, string) {
		reuse := len(args) == 2 && ((args[0] == "branch" && args[1] == "--show-current") || (args[0] == "status" && args[1] == "--porcelain=v1"))
		if !reuse {
			// Fetch is the only mutation in readiness and starts a new phase.
			if len(args) > 0 && args[0] == "fetch" {
				reset()
			}
			return run(root, args...)
		}
		k := key{root: root, argv: [2]string{args[0], args[1]}}
		if observed, ok := results[k]; ok {
			return observed.code, observed.out, observed.stderr
		}
		code, out, stderr := run(root, args...)
		results[k] = result{code: code, out: out, stderr: stderr}
		return code, out, stderr
	}
	output := func(root string, args ...string) string {
		code, out, _ := observe(root, args...)
		if code != 0 {
			return ""
		}
		return strings.TrimSpace(out)
	}
	return ReadinessGit{Run: observe, Output: output}, reset
}

// Root returns the strict git root of the record workspace, or an empty string when none
// resolves.
func (ReadinessGit) Root(record model.Record) string {
	return readinesspaths.StrictGitRoot(record)
}

// Head returns the HEAD commit of the record workspace, or an empty string when it cannot
// be read.
func (g ReadinessGit) Head(record model.Record) string {
	root := g.Root(record)
	if root == "" {
		return ""
	}
	code, out, _ := g.Run(root, "rev-parse", "HEAD")
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// IsWorktree reports whether root is a git worktree. It asks for the top-level rather than --is-inside-work-tree: the
// answer is the same for readiness (a bare repo or a .git directory fails
// both), and next's observation already read the top-level of this root, so
// the probe is not another git process.
func (g ReadinessGit) IsWorktree(root string) bool {
	code, out, _ := g.Run(root, "rev-parse", "--show-toplevel")
	return code == 0 && strings.TrimSpace(out) != ""
}

// Branch returns the current branch name at root, or an empty string when detached or unreadable.
func (g ReadinessGit) Branch(root string) string {
	return strings.TrimSpace(g.Output(root, "branch", "--show-current"))
}

// Clean reports whether git status at root prints nothing. A failed status prints nothing
// too, so it also reads as clean.
func (g ReadinessGit) Clean(root string) bool {
	return strings.TrimSpace(g.Output(root, "status", "--porcelain=v1")) == ""
}

// BaseState reports whether ref resolves to a commit and whether HEAD is behind it
// (ref is not an ancestor of HEAD).
func (g ReadinessGit) BaseState(root, ref string) (bool, bool) {
	if code, _, _ := g.Run(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}"); code != 0 {
		return false, false
	}
	code, _, _ := g.Run(root, "merge-base", "--is-ancestor", ref, "HEAD")
	return true, code != 0
}

// IsAncestor reports whether commit is an ancestor of, or equal to, ref.
func (g ReadinessGit) IsAncestor(root, commit, ref string) bool {
	code, _, _ := g.Run(root, "merge-base", "--is-ancestor", commit, ref)
	return code == 0
}

// ContentAbsorbed reports whether the changes of commit are already contained in ref even
// without commit being an ancestor.
func (g ReadinessGit) ContentAbsorbed(root, ref, commit string) bool {
	return gitContentAbsorbed(func(args ...string) (int, string) {
		code, stdout, _ := g.Run(root, args...)
		return code, stdout
	}, ref, commit)
}

// Upstream returns the upstream branch name of root, or an empty string when none is set.
func (g ReadinessGit) Upstream(root string) string {
	return strings.TrimSpace(g.Output(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
}

// Fetch runs a quiet pruning fetch at root and reports whether it failed, with its stderr.
func (g ReadinessGit) Fetch(root string) review.UpstreamFetch {
	code, _, stderr := g.Run(root, "fetch", "--quiet", "--prune")
	return review.UpstreamFetch{Root: root, Failed: code != 0, Stderr: strings.TrimSpace(stderr)}
}

// Counts returns the left-right commit counts of HEAD against its upstream, as printed by
// git rev-list --count.
func (g ReadinessGit) Counts(root string) string {
	return g.Output(root, "rev-list", "--left-right", "--count", "HEAD...@{u}")
}
