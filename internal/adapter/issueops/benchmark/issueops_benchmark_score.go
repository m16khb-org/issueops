package benchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
)

func ScoreIssueOpsBenchmarkArtifact(fixture contract.IssueOpsBenchmarkFixture, artifact contract.IssueOpsBenchmarkArtifact) IssueOpsBenchmarkScore {
	return domain.ScoreArtifact(fixture, artifact, issueOpsSkillRoutingFidelityComplete(fixture, artifact))
}
