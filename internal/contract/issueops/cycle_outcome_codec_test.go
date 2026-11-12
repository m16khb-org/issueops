package issueops

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
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

func TestDecodeCycleOutcomeIsStrictAboutSchemaAndCycle(t *testing.T) {
	outcome := CycleOutcome{
		SchemaVersion: CycleOutcomeSchemaVersion,
		ID:            "io-codec02",
		Repo:          "repo",
		Outcome:       CycleOutcomeAbandoned,
		RetainedAt:    "2026-11-12T00:00:00Z",
		Metrics:       ReviewMetricsCycle{ID: "io-codec02", Outcome: CycleOutcomeAbandoned, RegressCount: 1},
		Timeline:      &CycleTimeline{EventsTotal: 1, EventsShown: 1, Events: []TimelineEvent{{At: "2026-11-11T00:00:00Z", Kind: TimelineKindRegressed, Phase: "plan"}}},
	}
	data, err := EncodeCycleOutcome("io-codec02", outcome)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("round trip", func(t *testing.T) {
		got, err := DecodeCycleOutcome("io-codec02", data)
		if err != nil || !reflect.DeepEqual(got, outcome) {
			t.Fatalf("DecodeCycleOutcome() = %+v, %v; want %+v", got, err, outcome)
		}
	})
	t.Run("unknown field is ignored", func(t *testing.T) {
		got, err := DecodeCycleOutcome("io-codec02", append(bytes.TrimSuffix(data, []byte("}")), []byte(`,"added_later":{"x":1}}`)...))
		if err != nil || !reflect.DeepEqual(got, outcome) {
			t.Fatalf("DecodeCycleOutcome() = %+v, %v; want %+v", got, err, outcome)
		}
	})
	for _, tc := range []struct {
		name    string
		id      string
		data    []byte
		wantErr string
	}{
		{"bad JSON", "io-codec02", []byte(`{"schema_version":1,`), "decode cycle outcome io-codec02: "},
		{"other schema", "io-codec02", []byte(`{"schema_version":2,"id":"io-codec02","outcome":"abandoned","retained_at":"2026-11-12T00:00:00Z"}`), "cycle outcome io-codec02 has schema 2, want 1"},
		{"missing schema", "io-codec02", []byte(`{"id":"io-codec02","outcome":"abandoned","retained_at":"2026-11-12T00:00:00Z"}`), "cycle outcome io-codec02 has schema 0, want 1"},
		{"other cycle", "io-codec02", []byte(`{"schema_version":1,"id":"io-other","outcome":"abandoned","retained_at":"2026-11-12T00:00:00Z"}`), `cycle outcome row io-codec02 holds outcome for "io-other"`},
		{"stored under another id", "io-other", data, `cycle outcome row io-other holds outcome for "io-codec02"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeCycleOutcome(tc.id, tc.data)
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) || !reflect.DeepEqual(got, CycleOutcome{}) {
				t.Fatalf("DecodeCycleOutcome() = %+v, %v; want zero value and error %q", got, err, tc.wantErr)
			}
		})
	}
}
