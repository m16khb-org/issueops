package selfaugment

import (
	"encoding/json"
	"testing"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	verify "issueops/internal/domain/selfverify"
)

func TestPlannerRejectsHistoricalPythonIncompleteEvidence(t *testing.T) {
	for _, mode := range []string{"current", "v4", "hash-mismatch", "ineligible", "failed"} {
		t.Run(mode, func(t *testing.T) {
			snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: true, Summary: contract.SelfAugmentSummary{TerminationEligible: true, Contract: verify.ContractValue()}}
			switch mode {
			case "v4":
				snapshot.Summary.Contract.Version = 4
			case "hash-mismatch":
				snapshot.Summary.Contract.Hash = "old"
			case "ineligible":
				snapshot.Summary.TerminationEligible = false
			case "failed":
				snapshot.OK = false
			}
			NormalizeSnapshotFailureCause(&snapshot)
			bytes, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			planner := Planner{StateRead: func(string) (state.StateResult, error) {
				return state.StateResult{Record: state.RecordEnvelope{Content: string(bytes)}}, nil
			}}
			if got := planner.SelfVerificationPassed(); got != (mode == "current") {
				t.Fatalf("%s admitted=%v", mode, got)
			}
		})
	}
}
