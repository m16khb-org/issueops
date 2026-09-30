package issueopsbenchmark

import (
	"fmt"
	"time"

	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	port "issueops/internal/port/issueopsbenchmark"
)

type Service struct {
	Files    port.Files
	Runs     port.Runs
	Now      func() time.Time
	Artifact func(contract.IssueOpsBenchmarkFixture) contract.IssueOpsBenchmarkArtifact
}

func (s Service) Run(fixturesPath, judge, judgeFile string) (contract.IssueOpsBenchmarkRunResult, error) {
	fixtures, err := s.Files.LoadFixtures(fixturesPath)
	if err != nil {
		return contract.IssueOpsBenchmarkRunResult{}, err
	}
	artifacts := make(map[string]contract.IssueOpsBenchmarkArtifact, len(fixtures))
	for _, fixture := range fixtures {
		artifacts[fixture.ID] = s.Artifact(fixture)
	}
	result := domain.ScoreRun("issueops-benchmark-"+s.Now().UTC().Format("20060102T150405.000000000Z"), fixtures, artifacts)
	switch judge {
	case "file":
		judgeMap, err := s.Files.ReadJudgeMap(judgeFile, fixtures)
		if err != nil {
			return contract.IssueOpsBenchmarkRunResult{}, err
		}
		if err := s.ValidateJudgeProvenance(judgeMap, result.ID); err != nil {
			return contract.IssueOpsBenchmarkRunResult{}, err
		}
		for i, fixture := range fixtures {
			result.Scores[i] = domain.MergeScoreWithJudge(result.Scores[i], judgeMap.Scores[fixture.ID])
		}
	case "none":
	default:
		return contract.IssueOpsBenchmarkRunResult{}, fmt.Errorf("unsupported issueops benchmark judge %q", judge)
	}
	result = domain.FinalizeRun(result)
	if err := s.Runs.Save(result); err != nil {
		return contract.IssueOpsBenchmarkRunResult{}, err
	}
	return result, nil
}

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

func (s Service) Compare(baselineID, candidateID string) (contract.IssueOpsBenchmarkCompareResult, error) {
	baseline, err := s.Runs.Read(baselineID)
	if err != nil {
		return contract.IssueOpsBenchmarkCompareResult{}, err
	}
	candidate, err := s.Runs.Read(candidateID)
	if err != nil {
		return contract.IssueOpsBenchmarkCompareResult{}, err
	}
	return domain.CompareRuns(baseline, candidate), nil
}

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

func (s Service) Reliability(path string, alpha float64) (contract.ReliabilityReport, error) {
	recorded, err := s.Files.ReadOutcomes(path)
	if err != nil {
		return contract.ReliabilityReport{}, err
	}
	return domain.ComputeReliability(recorded, alpha)
}

func (s Service) Consensus(path string) (contract.ConsensusVerdict, error) {
	samples, err := s.Files.ReadSamples(path)
	if err != nil {
		return contract.ConsensusVerdict{}, err
	}
	return domain.ConsensusJudgeVerdict(samples)
}
