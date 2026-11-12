package issueops

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeCycleOutcomeRejectsForeignSchemaOrCycle(t *testing.T) {
	outcome := CycleOutcome{
		SchemaVersion: CycleOutcomeSchemaVersion,
		ID:            "io-codec01",
		Repo:          "repo",
		Outcome:       CycleOutcomePruned,
		RetainedAt:    "2026-11-12T00:00:00Z",
		Metrics:       ReviewMetricsCycle{ID: "io-codec01", Outcome: CycleOutcomePruned},
		Timeline:      &CycleTimeline{EventsTotal: 1},
	}

	got, err := EncodeCycleOutcome("io-codec01", outcome)
	if err != nil {
		t.Fatalf("EncodeCycleOutcome() error = %v", err)
	}
	want, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeCycleOutcome() = %s, want %s", got, want)
	}

	foreignSchema := outcome
	foreignSchema.SchemaVersion = CycleOutcomeSchemaVersion + 1
	if data, err := EncodeCycleOutcome("io-codec01", foreignSchema); err == nil {
		t.Fatalf("schema %d encoded to %s, want an error", foreignSchema.SchemaVersion, data)
	}

	if data, err := EncodeCycleOutcome("io-other", outcome); err == nil {
		t.Fatalf("outcome of io-codec01 encoded under io-other as %s, want an error", data)
	}
}
