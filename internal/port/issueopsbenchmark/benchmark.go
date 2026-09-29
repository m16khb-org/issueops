package issueopsbenchmark

import contract "issueops/internal/contract/issueopsbenchmark"

type Files interface {
	LoadFixtures(string) ([]contract.IssueOpsBenchmarkFixture, error)
	ReadJudgeMap(string, []contract.IssueOpsBenchmarkFixture) (contract.IssueOpsJudgeMap, error)
	ReadCandidate(string) (contract.IssueOpsAutoresearchCandidate, error)
	ReadOutcomes(string) (contract.RecordedOutcomes, error)
	ReadSamples(string) ([]contract.JudgeSample, error)
}

type Runs interface {
	Read(string) (contract.IssueOpsBenchmarkRunResult, error)
	Save(contract.IssueOpsBenchmarkRunResult) error
}
