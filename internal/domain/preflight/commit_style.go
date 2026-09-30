package preflight

import (
	preflightcontract "issueops/internal/contract/preflight"
	"regexp"
	"strings"
)

var conventionalSubjectRe = regexp.MustCompile(`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+`)

func CommitStyleHints(recent []preflightcontract.CommitInfo, bodies []string, policyPath string) map[string]any {
	conv := 0
	for _, c := range recent {
		if conventionalSubjectRe.MatchString(c.Subject) {
			conv++
		}
	}
	lore := 0
	for _, body := range bodies {
		for _, line := range strings.Split(body, "\n") {
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
	}
}
