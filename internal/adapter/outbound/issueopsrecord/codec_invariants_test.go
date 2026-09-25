package issueopsrecord

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

func TestCodecRejectsCompletedIssueURLMismatch(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-url-mismatch",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
		IssueURL:      "https://example.test/issues/2",
		IssueCreateIntent: &issueopscontract.IssueOpsIssueCreateIntent{
			OperationID:      "0123456789abcdef0123456789abcdef",
			Marker:           "<!-- issueops:issue-create:0123456789abcdef0123456789abcdef -->",
			Provider:         "github",
			ProjectAuthority: "example/repo",
			Title:            "example",
			BodySHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Status:           issueopscontract.IssueCreateIntentCompleted,
			Attempt:          1,
			CanonicalURL:     "https://example.test/issues/1",
			StartedAt:        "2026-09-24T00:00:00Z",
			UpdatedAt:        "2026-09-24T00:00:00Z",
		},
	}
	if err := issueopscontract.ValidateRecord(record); err != nil {
		t.Fatalf("record shape is invalid: %v", err)
	}
	if _, err := Encode(record); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("Encode error = %v, want invalid state", err)
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(record.ID, raw); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("Decode error = %v, want invalid state", err)
	}
	record.IssueURL = ""
	record.IssueCreateIntent.CanonicalURL = ""
	if err := issueopscontract.ValidateRecord(record); err != nil {
		t.Fatalf("record shape is invalid: %v", err)
	}
	if _, err := Encode(record); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("Encode missing completed URL error = %v, want invalid state", err)
	}
	raw, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(record.ID, raw); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("Decode missing completed URL error = %v, want invalid state", err)
	}
}

func TestDecodeRejectsRecordInvariantViolations(t *testing.T) {
	tests := []struct {
		name string
		id   string
		body string
	}{
		{name: "empty_phase", body: `"phase":""`},
		{name: "unknown_phase", body: `"phase":"unknown"`},
		{name: "invalid_id", id: "invalid", body: `"phase":"problem"`},
		{
			name: "phase_ledger_identity_mismatch",
			body: `"phase":"problem","phase_ledger":{"problem":{"phase":"plan"}}`,
		},
		{
			name: "unknown_plan_prep_status",
			body: `"phase":"problem","plan_prep":{"prior_decisions":{"status":"unknown"}}`,
		},
		{
			name: "unknown_cleanup_failure_step",
			body: `"phase":"done","cleanup_finish_failure":{"step":"secret-like-arbitrary-value"}`,
		},
		{name: "unknown_intent_class", body: `"phase":"problem","intent":{"intent_class":"unknown"}`},
		{name: "unknown_feedback_classification", body: `"phase":"feedback","feedback":[{"classification":"unknown"}]`},
		{name: "unknown_feedback_resolution", body: `"phase":"feedback","feedback":[{"resolution":"unknown"}]`},
		{name: "unknown_regress_phase", body: `"phase":"plan","regress_events":[{"from_phase":"unknown"}]`},
		{name: "unknown_routing_phase", body: `"phase":"plan","routing_trace":[{"phase":"unknown"}]`},
		{name: "unknown_branch_provider", body: `"phase":"plan","branch_prepare":{"provider":"bitbucket"}}`},
		{name: "unknown_remote_artifact_kind", body: `"phase":"pr","remote_artifact":{"provider":"github","kind":"mr"}}`},
		{name: "unknown_child_verdict", body: `"phase":"implement","child_cycles":[{"validation_verdict":"unknown"}]`},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id := fmt.Sprintf("io-invalid%02d", index)
			if test.id != "" {
				id = test.id
			}
			raw := fmt.Sprintf(`{"schema_version":1,"id":%q,%s}`, id, test.body)

			_, err := Decode(id, []byte(raw))

			if !errors.Is(err, statecontract.ErrInvalidState) {
				t.Fatalf("Decode error = %v, want invalid state", err)
			}
		})
	}
}

func TestEncodeRejectsRecordInvariantViolationsAsInvalidState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*issueopscontract.IssueOpsRecord)
	}{
		{
			name: "unknown_phase",
			mutate: func(record *issueopscontract.IssueOpsRecord) {
				record.Phase = "unknown"
			},
		},
		{
			name: "phase_ledger_identity_mismatch",
			mutate: func(record *issueopscontract.IssueOpsRecord) {
				record.PhaseLedger = issueopscontract.IssueOpsPhaseLedger{
					issueopscontract.IssueOpsPhaseProblem: {
						Phase: issueopscontract.IssueOpsPhasePlan,
					},
				}
			},
		},
		{
			name: "unknown_plan_prep_status",
			mutate: func(record *issueopscontract.IssueOpsRecord) {
				record.PlanPrep = &issueopscontract.IssueOpsPlanPrep{
					PriorDecisions: issueopscontract.IssueOpsPlanPrepItem{Status: "unknown"},
				}
			},
		},
		{
			name: "unknown_cleanup_failure_step",
			mutate: func(record *issueopscontract.IssueOpsRecord) {
				record.CleanupFinishFailure = &issueopscontract.IssueOpsCleanupFinishFailure{
					Step: "unknown",
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := issueopscontract.IssueOpsRecord{
				SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
				ID:            "io-invalid",
				Phase:         issueopscontract.IssueOpsPhaseProblem,
			}
			test.mutate(&record)

			_, err := Encode(record)

			if !errors.Is(err, statecontract.ErrInvalidState) ||
				err.Error() != statecontract.ErrInvalidState.Error() {
				t.Fatalf("Encode error = %v, want invalid state", err)
			}
		})
	}
}

func TestDecodeRejectsInvalidStateMatrix(t *testing.T) {
	tests := []struct {
		name string
		id   string
		raw  string
	}{
		{name: "missing_schema", id: "io-missing", raw: `{"id":"io-missing","phase":"problem"}`},
		{name: "zero_schema", id: "io-zero", raw: `{"schema_version":0,"id":"io-zero","phase":"problem"}`},
		{name: "future_schema", id: "io-future", raw: `{"schema_version":2,"id":"io-future","phase":"problem"}`},
		{name: "malformed_json", id: "io-malformed", raw: `{`},
		{name: "unknown_field", id: "io-unknown", raw: `{"schema_version":1,"id":"io-unknown","phase":"problem","unknown":true}`},
		{name: "id_mismatch", id: "io-requested", raw: `{"schema_version":1,"id":"io-other","phase":"problem"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(test.id, []byte(test.raw))

			if !errors.Is(err, statecontract.ErrInvalidState) ||
				err.Error() != statecontract.ErrInvalidState.Error() {
				t.Fatalf("Decode error = %v, want invalid state", err)
			}
		})
	}
}
