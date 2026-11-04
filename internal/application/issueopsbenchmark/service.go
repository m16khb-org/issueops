// Package issueopsbenchmark orchestrates the IssueOps benchmark use cases: scoring fixture
// runs, comparing persisted runs, evaluating the autoresearch gate, and computing
// reliability and judge consensus. Scoring rules live in the domain; file and run
// persistence come in through ports.
package issueopsbenchmark

import (
	"fmt"
	"time"

	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	port "issueops/internal/port/issueopsbenchmark"
)

// Service wires the benchmark use cases to their ports. Files loads fixtures and judge
// inputs, Runs persists scored runs, Now stamps run IDs, and Artifact produces the
// artifact scored for each fixture.
type Service struct {
	Files    port.Files
	Runs     port.Runs
	Now      func() time.Time
	Artifact func(contract.Fixture) contract.Artifact
}

// Run loads the fixtures at fixturesPath, scores the artifact built for each, and saves the
// finalized run. judge is "file" (merge per-fixture scores from the judge map at judgeFile,
// whose provenance must resolve to a persisted run) or "none"; any other value is an error.
func (s Service) Run(fixturesPath, judge, judgeFile string) (contract.RunResult, error) {
	fixtures, err := s.Files.LoadFixtures(fixturesPath)
	if err != nil {
		return contract.RunResult{}, err
	}
	artifacts := make(map[string]contract.Artifact, len(fixtures))
	for _, fixture := range fixtures {
		artifacts[fixture.ID] = s.Artifact(fixture)
	}
	result := domain.ScoreRun("issueops-benchmark-"+s.Now().UTC().Format("20060102T150405.000000000Z"), fixtures, artifacts)
	switch judge {
	case "file":
		judgeMap, err := s.Files.ReadJudgeMap(judgeFile, fixtures)
		if err != nil {
			return contract.RunResult{}, err
		}
		if err := s.ValidateJudgeProvenance(judgeMap, result.ID); err != nil {
			return contract.RunResult{}, err
		}
		for i, fixture := range fixtures {
			result.Scores[i] = domain.MergeScoreWithJudge(result.Scores[i], judgeMap.Scores[fixture.ID])
		}
	case "none":
	default:
		return contract.RunResult{}, fmt.Errorf("unsupported issueops benchmark judge %q", judge)
	}
	result = domain.FinalizeRun(result)
	if err := s.Runs.Save(result); err != nil {
		return contract.RunResult{}, err
	}
	return result, nil
}

// ValidateJudgeProvenance checks the judge map's metadata against scoredRunID and verifies
// that its source_run_id resolves to a persisted run.
func (s Service) ValidateJudgeProvenance(judge contract.IssueOpsJudgeMap, scoredRunID string) error {
	sourceID, err := domain.ValidateJudgeMetadata(judge, scoredRunID)
	if err != nil {
		return err
	}
	if _, err := s.Runs.Read(sourceID); err != nil {
		return fmt.Errorf("judge map source_run_id %q does not resolve to a persisted run: %w", sourceID, err)
	}
	return nil
}

// Compare reads the baseline and candidate runs by ID and returns their comparison.
func (s Service) Compare(baselineID, candidateID string) (contract.CompareResult, error) {
	baseline, err := s.Runs.Read(baselineID)
	if err != nil {
		return contract.CompareResult{}, err
	}
	candidate, err := s.Runs.Read(candidateID)
	if err != nil {
		return contract.CompareResult{}, err
	}
	return domain.CompareRuns(baseline, candidate), nil
}

// Gate reads the candidate file and the baseline and candidate runs, then evaluates the
// autoresearch gate for the given changed paths.
func (s Service) Gate(candidateFile, baselineID, candidateID string, changedPaths []string) (contract.IssueOpsAutoresearchGateResult, error) {
	candidate, err := s.Files.ReadCandidate(candidateFile)
	if err != nil {
		return contract.IssueOpsAutoresearchGateResult{}, err
	}
	baseline, err := s.Runs.Read(baselineID)
	if err != nil {
		return contract.IssueOpsAutoresearchGateResult{}, err
	}
	candidateRun, err := s.Runs.Read(candidateID)
	if err != nil {
		return contract.IssueOpsAutoresearchGateResult{}, err
	}
	return domain.EvaluateAutoresearchGate(contract.IssueOpsAutoresearchGateRequest{Candidate: candidate, BaselineRun: baseline, CandidateRun: candidateRun, ChangedPaths: changedPaths}), nil
}

// Reliability reads recorded outcomes from path and computes the reliability report for
// significance level alpha.
func (s Service) Reliability(path string, alpha float64) (contract.ReliabilityReport, error) {
	recorded, err := s.Files.ReadOutcomes(path)
	if err != nil {
		return contract.ReliabilityReport{}, err
	}
	return domain.ComputeReliability(recorded, alpha)
}

// Consensus reads judge samples from path and returns their consensus verdict.
func (s Service) Consensus(path string) (contract.ConsensusVerdict, error) {
	samples, err := s.Files.ReadSamples(path)
	if err != nil {
		return contract.ConsensusVerdict{}, err
	}
	return domain.ConsensusJudgeVerdict(samples)
}
