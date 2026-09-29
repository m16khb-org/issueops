package issueopscli

import (
	"encoding/json"
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
	benchmarkdomain "issueops/internal/domain/issueopsbenchmark"

	"testing"
)

func TestRunIssueOpsBenchmarkGateCLIKeepsCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	baseline := benchmarkcontract.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []benchmarkcontract.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []benchmarkcontract.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	}
	candidateRun := baseline
	candidateRun.ID = "candidate"
	if err := saveBenchmarkRunForTest(stateDir, benchmarkdomain.FinalizeRun(baseline)); err != nil {
		t.Fatal(err)
	}
	if err := saveBenchmarkRunForTest(stateDir, benchmarkdomain.FinalizeRun(candidateRun)); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, benchmarkcontract.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Bounded IssueOps changes should pass the gate.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "skills/issueops/SKILL.md", "--json"})
	})
	var result benchmarkcontract.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.KeepCandidate {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestRunIssueOpsBenchmarkGateCLIDiscardsOutsideEditSurface(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	run := benchmarkdomain.FinalizeRun(benchmarkcontract.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []benchmarkcontract.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []benchmarkcontract.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	})
	if err := saveBenchmarkRunForTest(stateDir, run); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, benchmarkcontract.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Only skill changes are allowed.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "baseline", "--candidate-file", candidatePath, "--changed-path", "cmd/issueops/issueops.go", "--json"})
	})
	var result benchmarkcontract.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 {
		t.Fatalf("expected edit-surface discard: %+v", result)
	}
}
