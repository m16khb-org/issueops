package issueopslease

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// lease vertical은 execution만 typed로 다루고 나머지 sidecar는 원문 그대로 다시 쓴다.
// execution 하위에서 lease 타입이 모르는 field가 있으면 Encode가 그 field를 조용히
// 버리므로, production execution의 모든 field가 lease 타입에 있어야 한다.
func TestLeaseExecutionShapeCoversEveryPersistedExecutionField(t *testing.T) {
	assertJSONShape(t, reflect.TypeOf(model.Execution{}), reflect.TypeOf(Execution{}), "Execution")
}

// lease Record에 없는 최상위 field는 Decode 뒤 Encode에서 사라진다. production
// record의 모든 최상위 field가 lease Record에 있어야 한다. sidecar는
// json.RawMessage로 받아 원문 그대로 다시 쓴다.
func TestLeaseRecordCarriesEveryPersistedTopLevelField(t *testing.T) {
	lease := jsonTaggedFields(reflect.TypeOf(Record{}))
	for _, field := range jsonTaggedFields(reflect.TypeOf(model.IssueOpsRecord{})) {
		if _, ok := lease[field.tag]; !ok {
			t.Errorf("persisted field %q is absent from the lease Record and would be dropped on re-encode", field.tag)
		}
	}
}

func TestDecodeRejectsFieldsThePersistedRecordDoesNotDefine(t *testing.T) {
	valid := `{"ok":true,"schema_version":1,"id":"io-strict","repo":"/repo","phase":"problem","created_at":"2026-09-23T00:00:00Z","updated_at":"2026-09-23T00:00:00Z"}`
	if _, err := Decode("io-strict", []byte(valid)); err != nil {
		t.Fatalf("a record with only defined fields must decode: %v", err)
	}
	for name, data := range map[string]string{
		"top-level":      strings.Replace(valid, `"ok":true`, `"ok":true,"future_field":1`, 1),
		"nested sidecar": strings.Replace(valid, `"ok":true`, `"ok":true,"intent":{"raw_request":"x","future_field":1}`, 1),
		"execution":      strings.Replace(valid, `"ok":true`, `"ok":true,"execution":{"mode":"direct","future_field":1}`, 1),
	} {
		if _, err := Decode("io-strict", []byte(data)); err == nil {
			t.Fatalf("%s unknown field must be rejected so a re-encode cannot drop it", name)
		}
	}
}

// Decode와 Encode를 거쳐도 lease 밖의 sidecar는 의미가 바뀌지 않아야 한다.
func TestDecodeEncodePreservesEverySidecar(t *testing.T) {
	record := model.IssueOpsRecord{
		OK: true, SchemaVersion: 1, ID: "io-preserve", Repo: "/repo", Branch: "12-preserve",
		Phase:    model.IssueOpsPhaseImplement,
		IssueURL: "https://github.com/example/issueops/issues/12",
		Intent: &model.IssueOpsIntentContract{
			RawRequest: "keep every sidecar", InterpretedIntent: "preserve", SuccessCriteria: []string{"round trip"},
			RecordedAt: "2026-09-23T00:00:00Z",
		},
		PlanPath:     "/repo.worktrees/12-preserve/plan.md",
		WorktreePath: "/repo.worktrees/12-preserve",
		Decisions: []model.IssueOpsDecision{{
			Title: "keep", Body: "keep the sidecar", Kind: "architecture", CreatedAt: "2026-09-23T00:00:00Z",
		}},
		ImplementationReview: &model.IssueOpsImplementationReview{
			Verdict: "pass", Findings: []string{"none"}, Evidence: []string{"go test"},
			ReviewedFingerprint: strings.Repeat("a", 64), RecordedAt: "2026-09-23T00:00:00Z",
		},
		SchemaEvidence: &model.IssueOpsSchemaEvidence{
			Measurements: []string{"rows=1"}, Sources: []string{"psql"}, RecordedAt: "2026-09-23T00:00:00Z",
		},
		LinkedBranchCleanup: &model.IssueOpsLinkedBranchCleanup{
			State: "deleted", LinkedBranchID: "LB_1", LinkedCount: 1, Deleted: true, ObservedAt: "2026-09-23T00:00:00Z",
		},
		BodySyncs: []model.IssueOpsRemoteBodySync{{
			Kind: "issue", URL: "https://github.com/example/issueops/issues/12", ToSHA256: strings.Repeat("b", 64), SyncedAt: "2026-09-23T00:00:00Z",
		}},
		AISlopCleanCategories: []string{"dead-code"},
		CreatedAt:             "2026-09-23T00:00:00Z",
		UpdatedAt:             "2026-09-23T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(record.ID, data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	encoded, err := Encode(decoded)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var before, after model.IssueOpsRecord
	if err := json.Unmarshal(data, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("decode/encode changed the record:\nbefore=%s\nafter=%s", data, encoded)
	}
}
