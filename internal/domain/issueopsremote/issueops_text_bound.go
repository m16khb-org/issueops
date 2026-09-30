package remote

import (
	"strings"

	"issueops/internal/domain/policy"
)

func boundedIssueOpsText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return policy.TruncateBytes(s, 400) + "...[truncated]"
	}
	return s
}
