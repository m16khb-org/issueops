package issueops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH0BaselineFixtureCoversEveryLauncherHostCell(t *testing.T) {
	var baseline Baseline
	readFixture(t, "h0-baseline.json", &baseline)

	if err := Validate(baseline); err != nil {
		t.Fatalf("fixture must be valid: %v", err)
	}

	want := map[string]bool{}
	for _, launcher := range []Launcher{LauncherOrca, LauncherHerdr, LauncherCmux, LauncherDirect} {
		for _, host := range []Host{HostCodex, HostClaude, HostOmo} {
			want[string(launcher)+"/"+string(host)] = false
		}
	}
	for _, cell := range baseline.Cells {
		key := string(cell.Launcher) + "/" + string(cell.Host)
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected cell %s", key)
		}
		want[key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("missing matrix cell %s", key)
		}
	}
}

func TestValidateRejectsBlankOrUnknownStatus(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Cells[0].Status = ""
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("blank status error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.Cells[0].Status = "maybe"
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("unknown status error=%v", err)
	}
}

func TestValidateRejectsBlankOrUnknownBaselineKind(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Kind = ""
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("blank kind error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.Kind = "cached-live-state"
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("unknown kind error=%v", err)
	}
}

func TestValidateStatusEvidenceRelationships(t *testing.T) {
	t.Run("unavailable requires disconnected or incapable evidence", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[6]
		cell.Status = StatusUnavailable
		cell.Evidence.Connected = observation(ClaimLive, true)
		cell.Evidence.Capable = observation(ClaimLive, true)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("unavailable relationship error=%v", err)
		}
	})

	t.Run("unsupported requires incapable evidence", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusUnsupported
		cell.Evidence.Capable = observation(ClaimLive, true)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("unsupported relationship error=%v", err)
		}
	})

	t.Run("not-run requires runtime not-run evidence", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusNotRun
		cell.Evidence.RuntimeVerified = observation(ClaimLive, true)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "non-supported") {
			t.Fatalf("not-run relationship error=%v", err)
		}
	})
}

func TestValidateRejectsSilentFallback(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Cells[0].SelectedLauncher = LauncherOrca
	baseline.Cells[0].ObservedLauncher = LauncherHerdr
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "silent fallback") {
		t.Fatalf("fallback error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.Cells[0].FallbackLauncher = LauncherDirect
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("fallback launcher error=%v", err)
	}
}

func TestValidateSeparatesInstalledCapableFromRuntimeSupported(t *testing.T) {
	baseline := minimalBaseline()
	cell := &baseline.Cells[0]
	baseline.Kind = BaselineKindLiveObservation
	cell.Status = StatusSupported
	cell.Evidence.Installed = observation(ClaimInstalled, true)
	cell.Evidence.Connected = observation(ClaimLive, true)
	cell.Evidence.Capable = observation(ClaimInstalled, true)
	cell.Evidence.RuntimeVerified = observation(ClaimNotRun, false)

	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "runtime_verified") {
		t.Fatalf("installed-only supported error=%v", err)
	}
}

func TestValidateSeparatesMockFromLiveRuntimeSupport(t *testing.T) {
	baseline := minimalBaseline()
	cell := &baseline.Cells[0]
	baseline.Kind = BaselineKindLiveObservation
	cell.Status = StatusSupported
	cell.Evidence.RuntimeVerified = observation(ClaimMock, true)

	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "runtime_verified") {
		t.Fatalf("mock runtime support error=%v", err)
	}
}

func TestValidateRequiresLiveObservationIdentity(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Cells[0].Evidence.Connected = Observation{Claim: ClaimLive, Result: ObservationResultPositive, Observed: true, Version: "1.0.0", ExecutablePath: "/bin/tool", ObservedAt: "2026-09-20T00:00:00Z", AttemptID: "attempt"}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "runtime_identity") {
		t.Fatalf("missing live identity error=%v", err)
	}
}

func TestValidateRequiresExactlySixDirectedCrossHostHandoffs(t *testing.T) {
	baseline := minimalBaseline()
	if err := Validate(baseline); err != nil {
		t.Fatalf("complete handoff fixture rejected: %v", err)
	}

	baseline = minimalBaseline()
	baseline.HandOffs = baseline.HandOffs[:5]
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "six directed cross-host") {
		t.Fatalf("missing handoff error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.HandOffs[5] = baseline.HandOffs[0]
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "duplicate handoff") {
		t.Fatalf("duplicate handoff error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.HandOffs[0] = HandOff{FromHost: HostCodex, ToHost: HostCodex, Semantics: HandOffMaterialTransfer}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "self handoff") {
		t.Fatalf("self handoff error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.HandOffs[0].Semantics = HandOffNativeSessionMigration
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "material transfer") {
		t.Fatalf("native migration error=%v", err)
	}
}

func TestValidateRejectsCrossHostNativeSessionMigration(t *testing.T) {
	baseline := minimalBaseline()
	baseline.HandOffs[0].Semantics = HandOffNativeSessionMigration
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "material transfer") {
		t.Fatalf("cross-host migration error=%v", err)
	}
}

func TestValidateAcceptsLiveSupportedWhenRuntimeBindsToBaseline(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Kind = BaselineKindLiveObservation
	cell := &baseline.Cells[0]
	cell.Status = StatusSupported
	cell.Evidence.RuntimeVerified = observationResult(ClaimLive, ObservationResultPositive)
	cell.Evidence.RuntimeVerified.AttemptID = baseline.AttemptID
	cell.Evidence.RuntimeVerified.RuntimeIdentity = baseline.RuntimeIdentity

	if err := Validate(baseline); err != nil {
		t.Fatalf("live supported baseline rejected: %v", err)
	}
}

func TestValidateEvidenceResultsAreExplicit(t *testing.T) {
	t.Run("deterministic supported is rejected", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusSupported
		cell.Evidence.RuntimeVerified = observationResult(ClaimLive, ObservationResultPositive)
		cell.Evidence.RuntimeVerified.AttemptID = baseline.AttemptID
		cell.Evidence.RuntimeVerified.RuntimeIdentity = baseline.RuntimeIdentity
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "live-observation") {
			t.Fatalf("deterministic supported error=%v", err)
		}
	})

	t.Run("supported requires baseline attempt binding", func(t *testing.T) {
		baseline := minimalBaseline()
		baseline.Kind = BaselineKindLiveObservation
		cell := &baseline.Cells[0]
		cell.Status = StatusSupported
		cell.Evidence.RuntimeVerified = observationResult(ClaimLive, ObservationResultPositive)
		cell.Evidence.RuntimeVerified.RuntimeIdentity = baseline.RuntimeIdentity
		cell.Evidence.RuntimeVerified.AttemptID = "other-attempt"
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "baseline attempt") {
			t.Fatalf("supported attempt binding error=%v", err)
		}
	})

	t.Run("supported requires baseline runtime binding", func(t *testing.T) {
		baseline := minimalBaseline()
		baseline.Kind = BaselineKindLiveObservation
		cell := &baseline.Cells[0]
		cell.Status = StatusSupported
		cell.Evidence.RuntimeVerified = observationResult(ClaimLive, ObservationResultPositive)
		cell.Evidence.RuntimeVerified.AttemptID = baseline.AttemptID
		cell.Evidence.RuntimeVerified.RuntimeIdentity = "other-runtime"
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "baseline runtime") {
			t.Fatalf("supported runtime binding error=%v", err)
		}
	})

	t.Run("unsupported requires explicit negative capable evidence", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusUnsupported
		cell.Evidence.Capable = observationResult(ClaimNotRun, ObservationResultNotRun)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "explicit negative capable") {
			t.Fatalf("unsupported explicit negative error=%v", err)
		}
	})

	t.Run("unavailable requires explicit negative connected or capable evidence", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[6]
		cell.Status = StatusUnavailable
		cell.Evidence.Connected = observationResult(ClaimNotRun, ObservationResultNotRun)
		cell.Evidence.Capable = observationResult(ClaimNotRun, ObservationResultNotRun)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "explicit negative connected or capable") {
			t.Fatalf("unavailable explicit negative error=%v", err)
		}
	})

	t.Run("non-supported status rejects live runtime verified", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusNotRun
		cell.Evidence.RuntimeVerified = observationResult(ClaimLive, ObservationResultPositive)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "non-supported") {
			t.Fatalf("non-supported live runtime error=%v", err)
		}
	})

	t.Run("not-run requires explicit not-run runtime result", func(t *testing.T) {
		baseline := minimalBaseline()
		cell := &baseline.Cells[0]
		cell.Status = StatusNotRun
		cell.Evidence.RuntimeVerified = observationResult(ClaimNotRun, ObservationResultNegative)
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "not-run") {
			t.Fatalf("not-run explicit result error=%v", err)
		}
	})
}

func TestValidateRecoveryModeInvariants(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Recovery = RecoveryInvariants{
		Direct: RecoveryChain{
			Mode: RecoveryModeDirect,
			Steps: []RecoveryStep{
				RecoveryDirectReleased,
				RecoveryReplacePreview,
				RecoveryReturnedRecoveryChain,
				RecoveryClaimable,
				RecoveryClaim,
			},
			FailureCases: []RecoveryFailure{
				RecoveryFailureReleasedImmediateClaim,
				RecoveryFailureStaleGeneration,
			},
		},
		Orca: RecoveryChain{
			Mode: RecoveryModeOrca,
			Steps: []RecoveryStep{
				RecoveryOrcaReleased,
				RecoveryReplace,
				RecoveryReseed,
				RecoveryResume,
				RecoverySealedOwnerClaim,
			},
			FailureCases: []RecoveryFailure{
				RecoveryFailureReleasedImmediateClaim,
				RecoveryFailureStaleGeneration,
				RecoveryFailureDuplicateOrcaOwner,
				RecoveryFailureManualOrcaOwner,
			},
		},
	}
	if err := Validate(baseline); err != nil {
		t.Fatalf("valid recovery invariants rejected: %v", err)
	}

	baseline.Recovery.Direct.Steps = []RecoveryStep{RecoveryDirectReleased, RecoveryClaim}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "direct recovery") {
		t.Fatalf("bad direct chain error=%v", err)
	}

	baseline = minimalBaseline()
	baseline.Recovery.Orca.Steps = []RecoveryStep{RecoveryOrcaReleased, RecoveryReplace, RecoveryReseed, RecoveryResume, RecoveryClaim}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "sealed owner") {
		t.Fatalf("bad orca chain error=%v", err)
	}
}

func TestValidateRecoveryFailureCasesAreExactSets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Baseline)
	}{
		{
			name: "direct extra failure",
			mutate: func(b *Baseline) {
				b.Recovery.Direct.FailureCases = append(b.Recovery.Direct.FailureCases, RecoveryFailureDuplicateOrcaOwner)
			},
		},
		{
			name: "direct duplicate failure",
			mutate: func(b *Baseline) {
				b.Recovery.Direct.FailureCases = append(b.Recovery.Direct.FailureCases, RecoveryFailureReleasedImmediateClaim)
			},
		},
		{
			name: "direct unknown failure",
			mutate: func(b *Baseline) {
				b.Recovery.Direct.FailureCases[0] = RecoveryFailure("future-direct-failure")
			},
		},
		{
			name: "orca extra failure",
			mutate: func(b *Baseline) {
				b.Recovery.Orca.FailureCases = append(b.Recovery.Orca.FailureCases, RecoveryFailure("future-orca-failure"))
			},
		},
		{
			name: "orca duplicate failure",
			mutate: func(b *Baseline) {
				b.Recovery.Orca.FailureCases = append(b.Recovery.Orca.FailureCases, RecoveryFailureManualOrcaOwner)
			},
		},
		{
			name: "orca unknown failure",
			mutate: func(b *Baseline) {
				b.Recovery.Orca.FailureCases[0] = RecoveryFailure("future-orca-failure")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := minimalBaseline()
			tt.mutate(&baseline)
			if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "failure cases") {
				t.Fatalf("failure cases accepted or wrong error: %v", err)
			}
		})
	}
}

func readFixture(t *testing.T, name string, target any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func minimalBaseline() Baseline {
	cells := make([]Cell, 0, 12)
	for _, launcher := range []Launcher{LauncherOrca, LauncherHerdr, LauncherCmux, LauncherDirect} {
		for _, host := range []Host{HostCodex, HostClaude, HostOmo} {
			status := StatusNotRun
			if launcher == LauncherCmux {
				status = StatusUnavailable
			}
			cells = append(cells, Cell{
				Launcher:         launcher,
				Host:             host,
				Status:           status,
				SelectedLauncher: launcher,
				ObservedLauncher: launcher,
				Evidence: Evidence{
					Installed:       observationResult(ClaimInstalled, ObservationResultPositive),
					Connected:       connectionObservation(launcher),
					Capable:         capabilityObservation(launcher),
					RuntimeVerified: observationResult(ClaimNotRun, ObservationResultNotRun),
				},
			})
		}
	}
	return Baseline{
		SchemaVersion:   1,
		Kind:            BaselineKindDeterministicFixture,
		GeneratedAt:     "2026-09-20T00:00:00Z",
		AttemptID:       "h0-test-attempt",
		Machine:         "fixture-machine",
		RuntimeIdentity: "fixture-runtime",
		Cells:           cells,
		HandOffs:        allDirectedCrossHostHandoffs(),
		Recovery: RecoveryInvariants{
			Direct: RecoveryChain{
				Mode: RecoveryModeDirect,
				Steps: []RecoveryStep{
					RecoveryDirectReleased,
					RecoveryReplacePreview,
					RecoveryReturnedRecoveryChain,
					RecoveryClaimable,
					RecoveryClaim,
				},
				FailureCases: []RecoveryFailure{
					RecoveryFailureReleasedImmediateClaim,
					RecoveryFailureStaleGeneration,
				},
			},
			Orca: RecoveryChain{
				Mode: RecoveryModeOrca,
				Steps: []RecoveryStep{
					RecoveryOrcaReleased,
					RecoveryReplace,
					RecoveryReseed,
					RecoveryResume,
					RecoverySealedOwnerClaim,
				},
				FailureCases: []RecoveryFailure{
					RecoveryFailureReleasedImmediateClaim,
					RecoveryFailureStaleGeneration,
					RecoveryFailureDuplicateOrcaOwner,
					RecoveryFailureManualOrcaOwner,
				},
			},
		},
	}
}
func allDirectedCrossHostHandoffs() []HandOff {
	hosts := []Host{HostCodex, HostClaude, HostOmo}
	handoffs := make([]HandOff, 0, 6)
	for _, fromHost := range hosts {
		for _, toHost := range hosts {
			if fromHost == toHost {
				continue
			}
			handoffs = append(handoffs, HandOff{FromHost: fromHost, ToHost: toHost, Semantics: HandOffMaterialTransfer})
		}
	}
	return handoffs
}

func observation(claim Claim, observed bool) Observation {
	if observed {
		return observationResult(claim, ObservationResultPositive)
	}
	return observationResult(claim, ObservationResultNotRun)
}

func observationResult(claim Claim, result ObservationResult) Observation {
	observed := result != ObservationResultNotRun
	return Observation{
		Claim:           claim,
		Result:          result,
		Observed:        observed,
		Version:         "fixture-version",
		ExecutablePath:  "/fixture/bin/tool",
		RuntimeIdentity: "fixture-runtime",
		ObservedAt:      "2026-09-20T00:00:00Z",
		AttemptID:       "attempt-1",
	}
}

func connectionObservation(launcher Launcher) Observation {
	if launcher == LauncherCmux {
		return observationResult(ClaimInstalled, ObservationResultNegative)
	}
	return observationResult(ClaimLive, ObservationResultPositive)
}

func capabilityObservation(launcher Launcher) Observation {
	if launcher == LauncherCmux {
		return observationResult(ClaimInstalled, ObservationResultNegative)
	}
	return observationResult(ClaimInstalled, ObservationResultPositive)
}
