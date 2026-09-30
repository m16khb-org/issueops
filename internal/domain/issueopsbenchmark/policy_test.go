package issueopsbenchmark

import (
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func TestJudgeMetadataRejectsSelfAttributedRun(t *testing.T) {
	judge := contract.IssueOpsJudgeMap{SourceRunID: "run-1", Provenance: "offline sample"}
	if _, err := ValidateJudgeMetadata(judge, "run-1"); err == nil {
		t.Fatal("self-attributed judge must be rejected")
	}
	if source, err := ValidateJudgeMetadata(judge, "run-2"); err != nil || source != "run-1" {
		t.Fatalf("source=%q err=%v", source, err)
	}
}

func TestBenchmarkGateRejectsUnscoredCandidate(t *testing.T) {
	result := EvaluateAutoresearchGate(contract.IssueOpsAutoresearchGateRequest{
		Candidate: contract.IssueOpsAutoresearchCandidate{
			ID: "candidate", Hypothesis: "improve quality", TargetDimensions: []string{"issue_quality"}, EditSurface: []string{"skills/**"},
		},
		BaselineRun:  contract.IssueOpsBenchmarkRunResult{OK: true},
		CandidateRun: contract.IssueOpsBenchmarkRunResult{},
	})
	if result.KeepCandidate || result.OK {
		t.Fatalf("empty benchmark result was accepted: %+v", result)
	}
}

func TestConsensusRejectsDuplicateSample(t *testing.T) {
	samples := []contract.JudgeSample{{SampleID: "one", Provenance: "offline"}, {SampleID: "one", Provenance: "offline"}}
	if _, err := ConsensusJudgeVerdict(samples); err == nil {
		t.Fatal("duplicate sample must not create a second vote")
	}
}
