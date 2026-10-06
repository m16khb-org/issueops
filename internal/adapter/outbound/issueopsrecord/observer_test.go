package issueopsrecord

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestJSONLineObserverEmitsOnlyActionableRedactedSpanEvents(t *testing.T) {
	var output bytes.Buffer
	observer := NewJSONLineObserver(&output, 100*time.Millisecond)

	observer.Observe(SpanObservation{
		Operation: "routing.update",
		Outcome:   "success",
		WaitMS:    1,
		HoldMS:    2,
	})
	if output.Len() != 0 {
		t.Fatalf("fast uncontended span emitted noise: %q", output.String())
	}

	observer.Observe(SpanObservation{
		Operation: "routing.update",
		Outcome:   "success",
		Contended: true,
		WaitMS:    4,
		HoldMS:    2,
	})
	line := strings.TrimSpace(output.String())
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatal(err)
	}
	if event["event"] != "issueops_record_span" ||
		event["operation"] != "routing.update" ||
		event["contended"] != true {
		t.Fatalf("unexpected span event: %v", event)
	}
	for _, forbidden := range []string{"root", "id", "bucket", "payload"} {
		if _, found := event[forbidden]; found {
			t.Fatalf("span event leaked %s: %v", forbidden, event)
		}
	}
}

func TestStoreScopesSpanObservationByCapability(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-observe01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}

	var observations []SpanObservation
	store := Store{
		Scope: "decision",
		Observer: ObserverFunc(func(observation SpanObservation) {
			observations = append(observations, observation)
		}),
	}
	if _, err := store.Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
			record.Branch = "observed"
			return record, true, nil
		},
	); err != nil {
		t.Fatal(err)
	}

	if len(observations) != 1 || observations[0].Operation != "decision.update" {
		t.Fatalf("scoped observations = %+v", observations)
	}
}

func seedObservedRecord(t testing.TB, id string) string {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, id, data); err != nil {
		t.Fatal(err)
	}
	return stateRoot
}

func TestStoreWritesCommitInsideObservedSpan(t *testing.T) {
	id := "io-commit01"
	stateRoot := seedObservedRecord(t, id)
	var observations []SpanObservation
	store := Store{Scope: "routing", Observer: ObserverFunc(func(observation SpanObservation) {
		observations = append(observations, observation)
	})}
	ctx := context.Background()
	if _, err := store.Update(ctx, stateRoot, id, func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		record.Branch = "observed"
		return record, true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateRelated(ctx, stateRoot, id, "artifact_stage_v1", func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
		return []byte(`{"stage":1}`), false, nil
	}); err != nil {
		t.Fatal(err)
	}
	if data, found, err := store.ReadRelated(ctx, stateRoot, id, "artifact_stage_v1"); err != nil || !found || string(data) != `{"stage":1}` {
		t.Fatalf("related put data=%q found=%v err=%v", data, found, err)
	}
	if _, err := store.UpdateRelated(ctx, stateRoot, id, "artifact_stage_v1", func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
		return nil, true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.ReadRelated(ctx, stateRoot, id, "artifact_stage_v1"); err != nil || found {
		t.Fatalf("related delete found=%v err=%v", found, err)
	}
	if record, err := store.Read(ctx, stateRoot, id); err != nil || record.Branch != "observed" {
		t.Fatalf("update not visible after span: record=%+v err=%v", record, err)
	}
	wantOperations := []string{"routing.update", "routing.related_update", "routing.related_update"}
	if len(observations) != len(wantOperations) {
		t.Fatalf("observations=%+v", observations)
	}
	for index, observation := range observations {
		if observation.Operation != wantOperations[index] || !observation.Acquired ||
			observation.CommitCount != 1 || observation.CommitCoverage != sqlstore.SpanCommitCoverageComplete ||
			observation.CommitMS == nil || observation.CallbackMS == nil {
			t.Fatalf("observation %d=%+v", index, observation)
		}
	}
}

func TestJSONLineObserverEmitsStageFieldsFromSQLiteSpan(t *testing.T) {
	id := "io-stage01"
	stateRoot := seedObservedRecord(t, id)
	var output bytes.Buffer
	store := Store{Scope: "routing", Observer: NewJSONLineObserver(&output, time.Nanosecond)}
	if _, err := store.Update(context.Background(), stateRoot, id, func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		record.Branch = "stage-fields"
		return record, true, nil
	}); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(output.String())
	t.Logf("sqlite span event: %s", line)
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatal(err)
	}
	if event["acquired"] != true || event["commit_count"] != float64(1) ||
		event["commit_coverage"] != sqlstore.SpanCommitCoverageComplete || event["outcome"] != "success" {
		t.Fatalf("stage event=%v", event)
	}
	values := map[string]float64{}
	for _, key := range []string{"wait_ms", "hold_ms", "callback_ms", "commit_ms", "total_ms"} {
		value, ok := event[key].(float64)
		if !ok || value < 0 {
			t.Fatalf("%s=%v is not a measured duration in %v", key, event[key], event)
		}
		values[key] = value
	}
	if values["total_ms"] < values["wait_ms"] || values["total_ms"] < values["hold_ms"] ||
		values["hold_ms"] < values["callback_ms"] || values["callback_ms"] < values["commit_ms"] {
		t.Fatalf("stage containment violated: %v", values)
	}
}

func TestJSONLineObserverKeepsUnknownStagesNull(t *testing.T) {
	var output bytes.Buffer
	NewJSONLineObserver(&output, 100*time.Millisecond).Observe(SpanObservation{
		Operation:      "routing.update",
		Outcome:        sqlstore.SpanOutcomeError,
		CommitCoverage: sqlstore.SpanCommitCoverageUnknown,
	})
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"callback_ms", "commit_ms"} {
		if value, found := event[key]; !found || value != nil {
			t.Fatalf("%s must be an explicit null, event=%v", key, event)
		}
	}
	if event["commit_coverage"] != sqlstore.SpanCommitCoverageUnknown {
		t.Fatalf("event=%v", event)
	}
}
