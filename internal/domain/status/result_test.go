package status

import (
	"errors"
	"reflect"
	"testing"
)

func TestEvaluatePreservesFailureAndWarningOrder(t *testing.T) {
	facts := Observation{Doctor: Outcome{OK: true}, State: Outcome{OK: true}, Workers: Outcome{OK: true}}
	healthy := Evaluate(facts)
	if !healthy.OK || healthy.Warnings == nil || len(healthy.Warnings) != 0 || healthy.SelfVerify.Found {
		t.Fatalf("unexpected empty result: %+v", healthy)
	}
	for _, missing := range []string{"doctor", "state", "workers"} {
		f := facts
		switch missing {
		case "doctor":
			f.Doctor.OK = false
		case "state":
			f.State.OK = false
		case "workers":
			f.Workers.OK = false
		}
		if Evaluate(f).OK {
			t.Fatalf("accepted failed %s", missing)
		}
	}
	facts.Doctor.Err = errors.New("")
	facts.State.Err = errors.New("cannot read state")
	facts.Workers.Err = errors.New("cannot read workers")
	got := Evaluate(facts)
	want := []string{"doctor: ", "state: cannot read state", "workers: cannot read workers"}
	if got.OK || !reflect.DeepEqual(got.Warnings, want) {
		t.Fatalf("result: %+v, want warnings %v", got, want)
	}
}

func TestEvaluateProjectsValidatedSummaryAndSeparatesDiagnosticsFromReadFailure(t *testing.T) {
	record := Record{Key: "custom-run", UpdatedAt: "2026", Bytes: 22}
	facts := Observation{Doctor: Outcome{OK: true}, State: Outcome{OK: true}, Workers: Outcome{OK: true}, LatestSelfVerify: &record, SelfVerifyWarnings: []string{"invalid_generated_at:older"}}
	before := facts
	got := Evaluate(facts)
	if !got.OK || !got.SelfVerify.Found || got.SelfVerify.LatestKey != record.Key || got.SelfVerify.UpdatedAt != record.UpdatedAt || got.SelfVerify.Bytes != record.Bytes || !reflect.DeepEqual(got.Warnings, []string{"selfverify: invalid_generated_at:older"}) {
		t.Fatalf("%+v", got)
	}
	if !reflect.DeepEqual(facts, before) || record != (Record{Key: "custom-run", UpdatedAt: "2026", Bytes: 22}) {
		t.Fatal("input mutated")
	}
	facts.SelfVerifyReadFailed = true
	if got := Evaluate(facts); got.OK || !got.SelfVerify.Found {
		t.Fatalf("%+v", got)
	}
}
