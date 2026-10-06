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
	legacyContract := domain.ContractValue()
	legacyContract.Version = 4
	legacyContract.Hash = ""
	bytes, err := json.Marshal(legacyContract)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(bytes)
	legacyContract.Hash = hex.EncodeToString(hash[:])
	current := domain.ContractValue()
	if current.Version != 8 || current.Hash == legacyContract.Hash || !reflect.DeepEqual(current.RequiredFields, legacyContract.RequiredFields) {
		t.Fatal(current, legacyContract)
	}
	old := augment.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: true, Summary: incomplete}
	old.Summary.Contract = legacyContract
	old.Summary.TerminationEligible = true
	if err := history.ValidateSummarySnapshot("old", old); err != nil {
		t.Fatalf("historical snapshots must remain readable: %v", err)
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
	t.Logf("v4 hash=%s; v5 hash=%s; added=%v; reverse regression=%v", legacyContract.Hash, current.Hash, comparison.AddedStepLabels, reverse.Regressions)
}
