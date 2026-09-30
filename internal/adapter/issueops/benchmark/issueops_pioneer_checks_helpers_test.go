package benchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
)

func issueOpsPioneerSkillEvidenceComplete(fixture contract.IssueOpsBenchmarkFixture, artifact contract.IssueOpsBenchmarkArtifact) bool {
	return domain.PioneerSkillEvidenceComplete(fixture, artifact)
}
