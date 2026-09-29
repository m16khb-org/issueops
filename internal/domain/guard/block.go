package guard

import contract "issueops/internal/contract/guard"

func BlockingError(result contract.GuardCheckResult) error {
	if result.OK {
		return nil
	}
	blockers := []contract.GuardFinding{}
	for _, finding := range result.Findings {
		if finding.Severity == "block" {
			blockers = append(blockers, finding)
		}
	}
	return contract.GuardBlockedError{Findings: blockers}
}
