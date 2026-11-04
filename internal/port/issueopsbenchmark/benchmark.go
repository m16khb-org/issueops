// Package issueopsbenchmark defines the ports the IssueOps quality benchmark
// uses to read its input files and to store run results.
package issueopsbenchmark

import contract "issueops/internal/contract/issueopsbenchmark"

// Files reads the benchmark inputs from disk: fixtures, the judge map,
// autoresearch candidates, recorded outcomes and judge samples. Each method
// takes the path of the file to read.
type Files interface {
	LoadFixtures(string) ([]contract.Fixture, error)
	ReadJudgeMap(string, []contract.Fixture) (contract.IssueOpsJudgeMap, error)
	ReadCandidate(string) (contract.IssueOpsAutoresearchCandidate, error)
	ReadOutcomes(string) (contract.RecordedOutcomes, error)
	ReadSamples(string) ([]contract.JudgeSample, error)
}

// Runs persists benchmark run results: Read loads the run stored at a path and
// Save writes a run result.
type Runs interface {
	Read(string) (contract.RunResult, error)
	Save(contract.RunResult) error
}
