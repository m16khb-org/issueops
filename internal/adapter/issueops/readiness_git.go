package issueops

import (
	"strings"

	"issueops/internal/adapter/issueops/readinesspaths"
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

type ReadinessGit struct {
	Run    func(string, ...string) (int, string, string)
	Output func(string, ...string) string
}

func (ReadinessGit) Root(record model.IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
}
func (g ReadinessGit) Head(record model.IssueOpsRecord) string {
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
func (g ReadinessGit) IsWorktree(root string) bool {
	code, out, _ := g.Run(root, "rev-parse", "--is-inside-work-tree")
	return code == 0 && strings.TrimSpace(out) == "true"
}
func (g ReadinessGit) Branch(root string) string {
	return strings.TrimSpace(g.Output(root, "branch", "--show-current"))
}
func (g ReadinessGit) Clean(root string) bool {
	return strings.TrimSpace(g.Output(root, "status", "--porcelain=v1")) == ""
}
func (g ReadinessGit) BaseAdvanced(root, ref string) bool {
	if code, _, _ := g.Run(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}"); code != 0 {
		return false
	}
	code, _, _ := g.Run(root, "merge-base", "--is-ancestor", ref, "HEAD")
	return code != 0
}
func (g ReadinessGit) Upstream(root string) string {
	return strings.TrimSpace(g.Output(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
}
func (g ReadinessGit) Fetch(root string) review.UpstreamFetch {
	code, _, stderr := g.Run(root, "fetch", "--quiet")
	return review.UpstreamFetch{Root: root, Failed: code != 0, Stderr: strings.TrimSpace(stderr)}
}
func (g ReadinessGit) Counts(root string) string {
	return g.Output(root, "rev-list", "--left-right", "--count", "HEAD...@{u}")
}
