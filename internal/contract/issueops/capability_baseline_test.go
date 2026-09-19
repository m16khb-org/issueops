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
		if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "not-run") {
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
	cell.Status = StatusSupported
	cell.Evidence.RuntimeVerified = observation(ClaimMock, true)

	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "runtime_verified") {
		t.Fatalf("mock runtime support error=%v", err)
	}
}

func TestValidateRequiresLiveObservationIdentity(t *testing.T) {
	baseline := minimalBaseline()
	baseline.Cells[0].Evidence.Connected = Observation{Claim: ClaimLive, Observed: true, Version: "1.0.0", ExecutablePath: "/bin/tool", ObservedAt: "2026-09-20T00:00:00Z", AttemptID: "attempt"}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "runtime_identity") {
		t.Fatalf("missing live identity error=%v", err)
	}
}

func TestValidateRejectsCrossHostNativeSessionMigration(t *testing.T) {
	baseline := minimalBaseline()
	baseline.HandOffs = []HandOff{{
		FromHost: HostCodex, ToHost: HostClaude,
		Semantics: HandOffNativeSessionMigration,
	}}
	if err := Validate(baseline); err == nil || !strings.Contains(err.Error(), "material transfer") {
		t.Fatalf("cross-host migration error=%v", err)
	}
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
					Installed:       observation(ClaimInstalled, true),
					Connected:       observation(ClaimLive, launcher != LauncherCmux),
					Capable:         observation(ClaimInstalled, launcher != LauncherCmux),
					RuntimeVerified: observation(ClaimNotRun, false),
				},
			})
		}
	}
	return Baseline{
		SchemaVersion: 1,
		Kind:          BaselineKindDeterministicFixture,
		GeneratedAt:   "2026-09-20T00:00:00Z",
		AttemptID:     "h0-test-attempt",
		Machine:       "fixture-machine",
		Cells:         cells,
		HandOffs: []HandOff{{
			FromHost: HostCodex, ToHost: HostClaude,
			Semantics: HandOffMaterialTransfer,
		}},
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

func observation(claim Claim, observed bool) Observation {
	return Observation{
		Claim:           claim,
		Observed:        observed,
		Version:         "fixture-version",
		ExecutablePath:  "/fixture/bin/tool",
		RuntimeIdentity: "fixture-runtime",
		ObservedAt:      "2026-09-20T00:00:00Z",
		AttemptID:       "attempt-1",
	}
}
