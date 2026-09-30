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

func TestEvaluateSelectsFirstSelfVerifyPrefixInSourceOrder(t *testing.T) {
	facts := Observation{Records: []Record{
		{Key: "unrelated", UpdatedAt: "2030", Bytes: 99},
		{Key: "self-verify-older", UpdatedAt: "2020", Bytes: 11},
		{Key: "self-verify-latest", UpdatedAt: "2026", Bytes: 22},
	}}
	got := Evaluate(facts).SelfVerify
	if !got.Found || got.LatestKey != "self-verify-older" || got.UpdatedAt != "2020" || got.Bytes != 11 {
		t.Fatalf("selection reordered: %+v", got)
	}
	facts.Records = []Record{{Key: "self-verifyanything", Bytes: 7}}
	if got := Evaluate(facts).SelfVerify; !got.Found || got.LatestKey != "self-verifyanything" {
		t.Fatalf("prefix semantics changed: %+v", got)
	}
}
