package looprun

import (
	"errors"
	"reflect"
	"testing"

	contract "issueops/internal/contract/looprun"
)

func TestRepoGateClassifiesMatchingLoopsAndFailsClosedOnUnreadableRecords(t *testing.T) {
	observation := RepoObservation{Repo: "/repo", Loops: []LoopObservation{
		{ID: "z", Error: errors.New("invalid state")},
		{Loop: contract.LoopRun{ID: "loop-active", Repo: " /repo ", Status: " active "}},
		{Loop: contract.LoopRun{ID: "loop-exhausted", Repo: "/repo", Status: "exhausted"}},
		{Loop: contract.LoopRun{ID: "loop-done", Repo: "/repo", Status: "succeeded"}},
		{Loop: contract.LoopRun{ID: "loop-stopped", Repo: "/repo", Status: "stopped"}},
		{Loop: contract.LoopRun{ID: "loop-foreign", Repo: "/other", Status: "active"}},
	}}
	got := EvaluateRepoGate(observation)
	if !reflect.DeepEqual(got.Missing, []string{"loop_incomplete:loop-active", "loop_incomplete:loop-exhausted", "loop_incomplete:z"}) || !reflect.DeepEqual(got.Warnings, []string{"failed to inspect loop run z: invalid state"}) || got.Summary.Active != 1 || got.Summary.Exhausted != 1 {
		t.Fatalf("gate=%+v", got)
	}
}
func TestRepoGatePreservesUnavailableAndEmptyContracts(t *testing.T) {
	for _, test := range []struct {
		name              string
		observation       RepoObservation
		missing, warnings []string
	}{
		{name: "blank", observation: RepoObservation{Repo: " "}},
		{name: "empty store", observation: RepoObservation{Repo: "/repo"}, missing: []string{}, warnings: []string{}},
		{name: "resolve", observation: RepoObservation{Repo: "relative", ResolveError: errors.New("cwd lost")}, missing: []string{"loops_complete"}, warnings: []string{"failed to resolve loop repo: cwd lost"}},
		{name: "scan", observation: RepoObservation{Repo: "/repo", ListError: errors.New("permission denied")}, missing: []string{"loops_complete"}, warnings: []string{"failed to scan loop runs: permission denied"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := EvaluateRepoGate(test.observation)
			if !reflect.DeepEqual(got.Missing, test.missing) || !reflect.DeepEqual(got.Warnings, test.warnings) {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}
