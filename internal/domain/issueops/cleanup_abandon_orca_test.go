package issueops

import "testing"

func TestAbandonIntentRequiresAuthoritativeAbsence(t *testing.T) {
	for _, tc := range []struct {
		count int
		zero  bool
		allow bool
	}{{0, false, false}, {1, true, false}, {0, true, true}} {
		err := ValidateCleanupAbandonIntentObservation("task_create", tc.count, tc.zero, nil)
		if (err == nil) != tc.allow {
			t.Fatalf("count=%d authoritative=%t: %v", tc.count, tc.zero, err)
		}
	}
}

func TestAbandonAllowsOnlyReachableTerminalResidue(t *testing.T) {
	for _, tc := range []struct {
		task, terminal, reachable, allow bool
	}{{false, true, false, false}, {false, true, true, true}, {true, true, true, false}, {false, false, false, true}} {
		err := ValidateCleanupAbandonOrcaObservation(CleanupAbandonOrcaObservation{InspectorAvailable: true, TaskLive: tc.task, TerminalLive: tc.terminal, TerminalsReachable: tc.reachable})
		if (err == nil) != tc.allow {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	if err := ValidateCleanupAbandonOrcaObservation(CleanupAbandonOrcaObservation{}); err == nil {
		t.Fatal("missing inspector authorized")
	}
}
