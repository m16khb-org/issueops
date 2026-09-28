package issueopspreparation

import (
	"encoding/json"
	"reflect"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func ValidateReconcileSnapshot(current, expected preparationcontract.Record) error {
	if current.Execution == nil || current.Execution.Pending == nil {
		return contractError("orca_intent_invalid", "Orca pending intent is missing")
	}
	if current.ID != expected.ID || !reflect.DeepEqual(current.Execution, expected.Execution) ||
		current.IssueURL != expected.IssueURL || !reconcileBranchEvidenceEqual(current.BranchPrepare, expected.BranchPrepare) {
		return contractError("orca_intent_authority_changed", "Orca intent authority changed before canonicalization")
	}
	return nil
}

func reconcileBranchEvidenceEqual(left, right json.RawMessage) bool {
	if len(left) == 0 {
		left = []byte("null")
	}
	if len(right) == 0 {
		right = []byte("null")
	}
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func ValidateReconcileIntentIssueIdentity(record preparationcontract.Record, intent preparationcontract.Intent) error {
	issue, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare))
	if err != nil {
		return err
	}
	if record.ID != intent.LifecycleID || intent.Probe.Provider != issue.Provider || intent.Probe.Issue != issue.Issue {
		return contractError("intent_identity_mismatch", "Orca intent issue identity changed before persistence")
	}
	return nil
}
