package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestValidateReconcileSnapshotPreservesAuthority(t *testing.T) {
	current := preparationcontract.Record{
		ID: "io-1", IssueURL: "https://github.com/example/repo/issues/193",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/193"}`),
		Execution:     &leasecontract.Execution{Mode: "orca", Pending: &leasecontract.ExternalIntent{OperationID: "op"}},
	}
	if err := ValidateReconcileSnapshot(current, current); err != nil {
		t.Fatal(err)
	}
	formatted := current
	formatted.BranchPrepare = []byte(`{ "issue_url": "https://github.com/example/repo/issues/193", "provider": "github" }`)
	if err := ValidateReconcileSnapshot(current, formatted); err != nil {
		t.Fatalf("equivalent branch evidence rejected: %v", err)
	}
	stale := current
	stale.Execution = &leasecontract.Execution{Mode: "direct"}
	if err := ValidateReconcileSnapshot(current, stale); err == nil || !strings.Contains(err.Error(), "orca_intent_authority_changed") {
		t.Fatalf("stale authority error=%v", err)
	}
}

func TestValidateReconcileIntentIssueIdentity(t *testing.T) {
	record := preparationcontract.Record{
		ID: "io-1", IssueURL: "https://github.com/example/repo/issues/193",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/193","link_verified":true}`),
	}
	intent := preparationcontract.Intent{LifecycleID: record.ID, Probe: preparationcontract.ProbeRequest{Provider: "github", Issue: 193}}
	if err := ValidateReconcileIntentIssueIdentity(record, intent); err != nil {
		t.Fatal(err)
	}
	intent.Probe.Issue = 194
	if err := ValidateReconcileIntentIssueIdentity(record, intent); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
		t.Fatalf("mismatched issue error=%v", err)
	}
}
