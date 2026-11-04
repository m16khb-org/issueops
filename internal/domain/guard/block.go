package guard

import contract "issueops/internal/contract/guard"

// BlockingError returns nil for a passing result, or a BlockedError that lists only the blocking findings.
func BlockingError(result contract.CheckResult) error {
	if result.OK {
		return nil
	}
	blockers := []contract.Finding{}
	for _, finding := range result.Findings {
		if finding.Severity == "block" {
			blockers = append(blockers, finding)
		}
	}
	return contract.BlockedError{Findings: blockers}
}
