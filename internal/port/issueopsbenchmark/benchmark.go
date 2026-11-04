package issueopsbenchmark

import contract "issueops/internal/contract/issueopsbenchmark"

type Files interface {
	LoadFixtures(string) ([]contract.Fixture, error)
	ReadJudgeMap(string, []contract.Fixture) (contract.IssueOpsJudgeMap, error)
	ReadCandidate(string) (contract.IssueOpsAutoresearchCandidate, error)
	ReadOutcomes(string) (contract.RecordedOutcomes, error)
	ReadSamples(string) ([]contract.JudgeSample, error)
}

type Runs interface {
	Read(string) (contract.RunResult, error)
	Save(contract.RunResult) error
}
