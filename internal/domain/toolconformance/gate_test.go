package toolconformance

import (
	"slices"
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

func gateDriftReport(hosts map[string][]contract.EpisodeReport, order ...string) contract.BenchmarkReport {
	report := contract.BenchmarkReport{}
	for _, host := range order {
		report.Hosts = append(report.Hosts, contract.HostReport{Host: host, Cases: hosts[host]})
	}
	return report
}

func driftEpisode(fixture, signature string) contract.EpisodeReport {
	return contract.EpisodeReport{FixtureID: fixture, Status: contract.EpisodeCompleted, Classification: contract.UnknownKey, DiagnosticSignature: signature}
}

func TestDecideGateIsIndependentOfReportOrdering(t *testing.T) {
	selected := []SelectedPair{{Host: "alpha", FixtureID: "f1"}, {Host: "alpha", FixtureID: "f2"}, {Host: "beta", FixtureID: "f1"}}
	build := func(hostOrder []string, alphaFirst bool) contract.BenchmarkReport {
		alpha := []contract.EpisodeReport{driftEpisode("f1", "s"), driftEpisode("f2", "s")}
		if !alphaFirst {
			alpha[0], alpha[1] = alpha[1], alpha[0]
		}
		return gateDriftReport(map[string][]contract.EpisodeReport{
			"alpha": alpha,
			"beta":  {driftEpisode("f1", "s")},
		}, hostOrder...)
	}
	want := DecideGate(build([]string{"alpha", "beta"}, true), selected, 1)
	if want.Decision != contract.GateNeedsReproduction {
		t.Fatalf("decision=%+v", want)
	}
	candidates := []string{"alpha:f1", "alpha:f2", "beta:f1"}
	if !slices.Contains(candidates, want.NextReproductionTarget) {
		t.Fatalf("target %q is not a drift candidate", want.NextReproductionTarget)
	}
	for _, order := range [][]string{{"beta", "alpha"}, {"alpha", "beta"}} {
		for _, alphaFirst := range []bool{true, false} {
			if got := DecideGate(build(order, alphaFirst), selected, 1); got != want {
				t.Fatalf("order=%v alphaFirst=%v got=%+v want=%+v", order, alphaFirst, got, want)
			}
		}
	}
}

func TestDecideGateConfirmsARepeatedSignatureDeterministically(t *testing.T) {
	selected := []SelectedPair{{Host: "codex", FixtureID: "f"}}
	build := func(signatures ...string) contract.BenchmarkReport {
		cases := make([]contract.EpisodeReport, 0, len(signatures))
		for _, signature := range signatures {
			cases = append(cases, driftEpisode("f", signature))
		}
		return gateDriftReport(map[string][]contract.EpisodeReport{"codex": cases}, "codex")
	}
	want := DecideGate(build("y", "x", "y", "x"), selected, 1)
	if want.Decision != contract.GateAuthorizeHardening || want.ConfirmedCount != 2 || (want.ConfirmedSignature != "x" && want.ConfirmedSignature != "y") {
		t.Fatalf("decision=%+v", want)
	}
	if got := DecideGate(build("x", "y", "x", "y"), selected, 1); got != want {
		t.Fatalf("reordered cases got=%+v want=%+v", got, want)
	}
}
