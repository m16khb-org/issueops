package toolconformance

import (
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func TestDecideGatePreservesCompletionAndDriftPriority(t *testing.T) {
	selected := []SelectedPair{{Host: "codex", FixtureID: "fixture"}}
	report := contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex", Cases: []contract.EpisodeReport{
		{FixtureID: "fixture", Status: contract.EpisodeCompleted, Classification: contract.UnknownKey, DiagnosticSignature: "drift"},
	}}}}
	if got := DecideGate(report, selected, 2); got.Decision != contract.GateInconclusive {
		t.Fatalf("incomplete decision=%+v", got)
	}
	if got := DecideGate(report, selected, 1); got.Decision != contract.GateNeedsReproduction || got.NextReproductionTarget != "codex:fixture" {
		t.Fatalf("single drift decision=%+v", got)
	}
	report.Hosts[0].Cases = append(report.Hosts[0].Cases, report.Hosts[0].Cases[0])
	if got := DecideGate(report, selected, 2); got.Decision != contract.GateAuthorizeHardening || got.ConfirmedSignature != "drift" || got.ConfirmedCount != 2 {
		t.Fatalf("repeated drift decision=%+v", got)
	}
	report.Hosts[0].Cases[1].Status = contract.EpisodeIncomplete
	if got := DecideGate(report, selected, 1); got.Decision != contract.GateNeedsReproduction {
		t.Fatalf("incomplete episode must not confirm drift: %+v", got)
	}
}

func TestDecideGateRequiresReproductionOnlyForDrift(t *testing.T) {
	selected := []SelectedPair{{Host: "codex", FixtureID: "fixture"}}
	episode := contract.EpisodeReport{FixtureID: "fixture", Status: contract.EpisodeCompleted, Classification: contract.ExactValid}
	report := contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex", Cases: []contract.EpisodeReport{episode}}}}
	if got := DecideGate(report, selected, 1); got.Decision != contract.GateDeferHardening {
		t.Fatalf("valid observations should defer hardening: %+v", got)
	}
	episode.Classification = contract.EnumMismatch
	episode.DiagnosticSignature = "drift"
	report.Hosts[0].Cases = make([]contract.EpisodeReport, 20)
	for i := range report.Hosts[0].Cases {
		report.Hosts[0].Cases[i] = episode
		report.Hosts[0].Cases[i].DiagnosticSignature = string(rune('a' + i))
	}
	if got := DecideGate(report, selected, 20); got.Decision != contract.GateUnreproducedObservation {
		t.Fatalf("twenty unique drifts should be unreproduced: %+v", got)
	}
}
