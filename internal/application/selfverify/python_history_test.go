package selfverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	contract "issueops/internal/contract/selfverify"
	"reflect"
	"strings"
	"testing"

	augment "issueops/internal/contract/selfaugment"
	history "issueops/internal/domain/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

func TestPythonContractDistinguishesHistoricalEvidenceWithoutChangingSnapshotSchema(t *testing.T) {
	steps := []contract.StepResult{}
	for _, label := range domain.StepOrder() {
		if label != "Python script tests" {
			steps = append(steps, contract.StepResult{Label: label, OK: true})
		}
	}
	oldInput := augment.SelfAugmentResult{OK: true, Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: steps}}}
	incomplete := SummarizeSelfVerification(oldInput, 95)
	if incomplete.TerminationEligible || len(incomplete.CoverageGaps) != 1 || !strings.Contains(incomplete.CoverageGaps[0], "Python script tests") {
		t.Fatal(incomplete)
	}
	olderContract := domain.ContractValue()
	olderContract.Version = 4
	olderContract.Hash = ""
	bytes, err := json.Marshal(olderContract)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(bytes)
	olderContract.Hash = hex.EncodeToString(hash[:])
	current := domain.ContractValue()
	if current.Version != 9 || current.Hash == olderContract.Hash || !reflect.DeepEqual(current.RequiredFields, olderContract.RequiredFields) {
		t.Fatal(current, olderContract)
	}
	old := augment.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: true, Summary: incomplete}
	old.Summary.Contract = olderContract
	old.Summary.TerminationEligible = true
	if err := history.ValidateSummarySnapshot("old", old); err != nil {
		t.Fatalf("older-contract snapshots must stay comparable: %v", err)
	}
	steps = append(steps, contract.StepResult{Label: "Python script tests", OK: true})
	newInput := augment.SelfAugmentResult{OK: true, Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: steps}}}
	fresh := augment.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: true, Summary: SummarizeSelfVerification(newInput, 95)}
	comparison := history.CompareSnapshots("v4", "v5", 100, old, fresh, "fixture")
	if comparison.Regressed || !reflect.DeepEqual(comparison.AddedStepLabels, []string{"Python script tests"}) || comparison.TotalStepsDelta != 1 {
		t.Fatal(comparison)
	}
	reverse := history.CompareSnapshots("v5", "v4", 100, fresh, old, "fixture")
	if !reverse.Regressed || !reflect.DeepEqual(reverse.MissingStepLabels, []string{"Python script tests"}) {
		t.Fatal(reverse)
	}
	t.Logf("v4 hash=%s; v5 hash=%s; added=%v; reverse regression=%v", olderContract.Hash, current.Hash, comparison.AddedStepLabels, reverse.Regressions)
}

// legacySummarySnapshot is a v8-era saved self-verification summary: no
// planned_steps/not_run_steps, and goal/coverage entries without not_run.
const legacySummarySnapshot = `{
  "schema_version": 1, "kind": "self_verification_summary", "loop_kind": "self_verification", "ok": false,
  "iterations": 1, "base_seed": 100, "target_score": 95, "elapsed_ms": 1000, "generated_at": "2026-10-01T00:00:00Z",
  "summary": {
    "total_runs": 1, "total_steps": 3, "passed_steps": 2, "failed_steps": 1, "target_score": 95,
    "contract": {"name": "self_verification_summary", "version": 8, "hash": "legacy", "required_fields": [], "goal_names": [], "coverage_claims": []},
    "minimum_goal_score": 0, "termination_eligible": false,
    "goal_scores": [{"name": "build_release", "korean_name": "빌드 산출물", "score": 0, "target_score": 95, "passed": false, "evidence_labels": ["go build"], "passed_checks": 0, "total_checks": 1}],
    "coverage": [{"claim": "release build artifact", "evidence_labels": ["go build"], "covered": false, "missing_labels": ["go build"]}],
    "coverage_gaps": ["release build artifact: missing go build"],
    "failed_iteration": 1, "failed_seed": 100, "failed_step": "Python script tests",
    "step_labels": ["harness invariants", "gofmt", "Python script tests"],
    "slowest_steps": [], "step_duration_stats": [],
    "failure_cause": "unknown", "failure_cause_reason": "x", "failure_cause_evidence": []
  }
}`

func TestSavedSummariesWithoutNotRunFieldsStillDecodeAndCompare(t *testing.T) {
	var old augment.SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(legacySummarySnapshot), &old); err != nil {
		t.Fatalf("legacy summary must decode: %v", err)
	}
	if err := history.ValidateSummarySnapshot("legacy", old); err != nil {
		t.Fatal(err)
	}
	if old.Summary.PlannedSteps != 0 || old.Summary.NotRunSteps != nil || old.Summary.GoalScores[0].NotRun || old.Summary.Coverage[0].NotRun {
		t.Fatalf("absent fields must decode to zero values: %+v", old.Summary)
	}

	// A fresh fail-fast summary compares against it without error or spurious step-label regression.
	order := domain.StepOrder()
	fresh := augment.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", Summary: SummarizeSelfVerification(augment.SelfAugmentResult{Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: []contract.StepResult{
		{Label: order[0], OK: true}, {Label: order[1], OK: true}, {Label: order[2], OK: false},
	}}}}, 95)}
	comparison := history.CompareSnapshots("legacy", "fresh", 100, old, fresh, "fixture")
	if !comparison.OK || len(comparison.MissingStepLabels) != 0 || len(comparison.AddedStepLabels) != 0 || !strings.Contains(strings.Join(comparison.Warnings, ","), "step_duration_contract_mismatch") {
		t.Fatalf("comparison=%+v", comparison)
	}
	if comparison.CandidateSummary.PlannedSteps != len(order) || len(comparison.CandidateSummary.NotRunSteps) != len(order)-3 {
		t.Fatalf("candidate summary lost not-run evidence: %+v", comparison.CandidateSummary)
	}
}
