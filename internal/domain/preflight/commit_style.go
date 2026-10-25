package preflight

import (
	preflightcontract "issueops/internal/contract/preflight"
	"regexp"
	"strings"
	"sync"
)

var conventionalSubjectRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+`)
})

// CommitStyleHints summarizes recent commit style. message_policy_source says
// whose policy message_policy_doc_path is: QA read the harness default path
// in another repository as a wrong path.
func CommitStyleHints(recent []preflightcontract.CommitInfo, bodies []string, policyPath, repoRoot string) map[string]any {
	conv := 0
	for _, c := range recent {
		if conventionalSubjectRe().MatchString(c.Subject) {
			conv++
		}
	}
	lore := 0
	for _, body := range bodies {
		for line := range strings.SplitSeq(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "Lore:" || strings.HasPrefix(line, "Lore-") {
				lore++
				break
			}
		}
	}
	return map[string]any{
		"recent_count":            len(recent),
		"conventional_subjects":   conv,
		"lore_bodies":             lore,
		"recommended":             "conventional_subject_plus_lore_body",
		"message_policy_doc_path": policyPath,
		"message_policy_source":   commitPolicySource(policyPath, repoRoot),
	}
}

func commitPolicySource(policyPath, repoRoot string) string {
	if repoRoot != "" && strings.HasPrefix(policyPath, strings.TrimSuffix(repoRoot, "/")+"/") {
		return "repository"
	}
	return "issueops_default"
}
