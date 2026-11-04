package guard

import (
	"fmt"
	guardcontract "issueops/internal/contract/guard"
)

func dedupeFindings(findings []guardcontract.Finding) []guardcontract.Finding {
	seen := map[string]bool{}
	out := []guardcontract.Finding{}
	for _, finding := range findings {
		key := finding.Severity + "\x00" + finding.Rule + "\x00" + finding.File + "\x00" + fmt.Sprint(finding.Line) + "\x00" + finding.Evidence
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

func severityRank(severity string) int {
	switch severity {
	case "block":
		return 0
	case "warn":
		return 1
	case "review":
		return 2
	default:
		return 3
	}
}
