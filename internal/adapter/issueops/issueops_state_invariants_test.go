package issueops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

func TestReadIssueOpsAcceptsValidRecord(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsSchemaVersion,
		ID:            "io-valid",
		Repo:          "/repo",
		Branch:        "state-read",
		Phase:         issueops.IssueOpsPhaseProblem,
		CreatedAt:     "2026-10-01T00:00:00Z",
		UpdatedAt:     "2026-10-01T01:00:00Z",
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueOpsBucket, want.ID, raw); err != nil {
		t.Fatal(err)
	}
	want.OK = true
	for _, reader := range []struct {
		name string
		read func(string, string) (issueops.IssueOpsRecord, error)
	}{
		{name: "read", read: ReadIssueOps},
		{name: "existing", read: ReadIssueOpsExisting},
	} {
		t.Run(reader.name, func(t *testing.T) {
			got, err := reader.read(stateRoot, " "+want.ID+" ")
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("record = %+v, error = %v, want %+v", got, err, want)
			}
		})
	}
}

func TestReadIssueOpsMissingRecordPreservesErrorIdentity(t *testing.T) {
	stateRoot := t.TempDir()
	if _, err := sqlstore.Open(stateRoot); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(string, string) (issueops.IssueOpsRecord, error){ReadIssueOps, ReadIssueOpsExisting} {
		got, err := read(stateRoot, "io-missing")
		want := issueops.IssueOpsRecord{ID: "io-missing"}
		if !errors.Is(err, fs.ErrNotExist) || err.Error() != "issueops record io-missing: file does not exist" ||
			!reflect.DeepEqual(got, want) {
			t.Fatalf("record = %+v, error = %v, want %+v and not-exist error", got, err, want)
		}
	}
}

func TestReadIssueOpsRejectsRecordInvariantViolations(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body string
	}{
		{name: "empty_phase", body: `"phase":""`},
		{name: "unknown_phase", body: `"phase":"unknown"`},
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
			raw := fmt.Sprintf(`{"schema_version":1,"id":%q,%s}`, id, test.body)
			if err := database.Put(issueOpsBucket, id, []byte(raw)); err != nil {
				t.Fatal(err)
			}

			for _, read := range []func(string, string) (issueops.IssueOpsRecord, error){ReadIssueOps, ReadIssueOpsExisting} {
				got, err := read(stateRoot, id)
				want := issueops.IssueOpsRecord{ID: id, Invalid: true, InvalidReason: statecontract.ErrInvalidState.Error()}
				if !errors.Is(err, statecontract.ErrInvalidState) ||
					!reflect.DeepEqual(got, want) {
					t.Fatalf("record = %+v, error = %v, want %+v and invalid state", got, err, want)
				}
			}
		})
	}
}

func TestReadIssueOpsRejectsInvalidStateMatrix(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		id   string
		raw  string
	}{
		{name: "missing_schema", id: "io-missing", raw: `{"id":"io-missing","phase":"problem"}`},
		{name: "zero_schema", id: "io-zero", raw: `{"schema_version":0,"id":"io-zero","phase":"problem"}`},
		{name: "future_schema", id: "io-future", raw: `{"schema_version":2,"id":"io-future","phase":"problem"}`},
		{name: "unsupported_schema", id: "io-unsupported", raw: `{"schema_version":-1,"id":"io-unsupported","phase":"problem"}`},
		{name: "malformed_json", id: "io-malformed", raw: `{`},
		{name: "trailing_json", id: "io-trailing", raw: `{"schema_version":1,"id":"io-trailing","phase":"problem"} {}`},
		{name: "trailing_garbage", id: "io-garbage", raw: `{"schema_version":1,"id":"io-garbage","phase":"problem"} x`},
		{name: "unknown_field", id: "io-unknown", raw: `{"schema_version":1,"id":"io-unknown","phase":"problem","unknown":true}`},
		{name: "id_mismatch", id: "io-requested", raw: `{"schema_version":1,"id":"io-other","phase":"problem"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := database.Put(issueOpsBucket, test.id, []byte(test.raw)); err != nil {
				t.Fatal(err)
			}

			for _, read := range []func(string, string) (issueops.IssueOpsRecord, error){ReadIssueOps, ReadIssueOpsExisting} {
				got, err := read(stateRoot, test.id)
				want := issueops.IssueOpsRecord{ID: test.id, Invalid: true, InvalidReason: statecontract.ErrInvalidState.Error()}
				if !errors.Is(err, statecontract.ErrInvalidState) ||
					err.Error() != statecontract.ErrInvalidState.Error() || !reflect.DeepEqual(got, want) {
					t.Fatalf("record = %+v, error = %v, want %+v and invalid state", got, err, want)
				}
			}
		})
	}
}
