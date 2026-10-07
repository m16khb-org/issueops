package selfaugment

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
)

func TestEmptySummaryDoesNotGainPerfectScore(t *testing.T) {
	summary := SummarizeSteps(nil, 95)
	FinalizeSummary(&summary, false)
	if summary.TotalRuns != 0 || summary.MinimumGoalScore != 0 || summary.TerminationEligible {
		t.Fatalf("empty summary gained success: %+v", summary)
	}
}

func TestSummarySeparatesMeasuredDurationsFromReusedEvidence(t *testing.T) {
	// Given: reuse carries no timing sample, even if an old duration was copied.
	var runs []SummaryRun
	if err := json.Unmarshal([]byte(`[
		{"Iteration":1,"Seed":10,"Steps":[
			{"Label":"mixed","OK":true,"DurationMS":137},
			{"Label":"mixed","OK":true,"DurationMS":9000,"Reused":true},
			{"Label":"reuse-only","OK":true,"DurationMS":8000,"Reused":true},
			{"Label":"zero","OK":true,"DurationMS":0},
			{"Label":"failed","OK":false,"DurationMS":149}]},
		{"Iteration":2,"Seed":11,"Steps":[
			{"Label":"reuse-only","OK":true,"DurationMS":0,"Reused":true}]}
	]`), &runs); err != nil {
		t.Fatal(err)
	}

	// When
	summary := SummarizeSteps(runs, 95)

	// Then: all evidence still contributes to coverage and pass/fail accounting.
	if summary.TotalSteps != 6 || summary.PassedSteps != 5 || summary.FailedSteps != 1 || summary.FailedStep != "failed" {
		t.Fatalf("evidence accounting changed: %+v", summary)
	}
	if !reflect.DeepEqual(summary.StepLabels, []string{"mixed", "reuse-only", "zero", "failed"}) {
		t.Fatalf("labels = %v", summary.StepLabels)
	}
	wantSlow := []contract.SelfAugmentSlowStep{
		{Iteration: 1, Seed: 10, Label: "failed", DurationMS: 149},
		{Iteration: 1, Seed: 10, Label: "mixed", DurationMS: 137},
		{Iteration: 1, Seed: 10, Label: "zero"},
	}
	if !reflect.DeepEqual(summary.SlowestSteps, wantSlow) {
		t.Errorf("slowest steps = %+v, want %+v", summary.SlowestSteps, wantSlow)
	}
	serialized, err := json.Marshal(summary.StepDurationStats)
	if err != nil {
		t.Fatal(err)
	}
	var stats []struct {
		Label             string  `json:"label"`
		Count             int     `json:"count"`
		ReusedCount       int     `json:"reused_count"`
		MinDurationMS     int64   `json:"min_duration_ms"`
		MaxDurationMS     int64   `json:"max_duration_ms"`
		AverageDurationMS float64 `json:"average_duration_ms"`
		P95DurationMS     int64   `json:"p95_duration_ms"`
	}
	if err := json.Unmarshal(serialized, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats) != 4 {
		t.Fatalf("stats = %s", serialized)
	}
	for _, stat := range stats {
		wantDuration, wantCount, wantReused := int64(0), 1, 0
		switch stat.Label {
		case "mixed":
			wantDuration, wantReused = 137, 1
		case "reuse-only":
			wantCount, wantReused = 0, 2
		case "failed":
			wantDuration = 149
		case "zero":
		default:
			t.Fatalf("unexpected stat: %+v", stat)
		}
		if stat.Count != wantCount || stat.ReusedCount != wantReused || stat.MinDurationMS != wantDuration ||
			stat.MaxDurationMS != wantDuration || stat.P95DurationMS != wantDuration || stat.AverageDurationMS != float64(wantDuration) {
			t.Errorf("duration stat = %+v, want duration=%d count=%d reused=%d", stat, wantDuration, wantCount, wantReused)
		}
	}
}

func TestSummarySnapshotKeepsV1AndReadsOlderContractDurationEvidence(t *testing.T) {
	// Given
	var older contract.SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(`{"schema_version":1,"kind":"self_verification_summary","summary":{
		"contract":{"name":"self_verification_summary","version":6,"hash":"older"},
		"step_duration_stats":[{"label":"go test","count":2,"p95_duration_ms":137}]
	}}`), &older); err != nil {
		t.Fatal(err)
	}

	// When
	err := ValidateSummarySnapshot("older", older)
	current := NewSelfVerificationSummarySnapshot(contract.SelfAugmentResult{}, time.Time{})

	// Then: no inference of old reuse or migration of the envelope.
	if err != nil || current.SchemaVersion != 1 {
		t.Fatalf("snapshot compatibility: current=%+v err=%v", current, err)
	}
	stats := StepDurationStatsForCompare(older.Summary)
	if len(stats) != 1 || stats[0].Count != 2 || stats[0].P95DurationMS != 137 || older.Summary.Contract.Version != 6 {
		t.Fatalf("older evidence changed: %+v", older)
	}
}
