package guard

import contract "issueops/internal/contract/guard"

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
