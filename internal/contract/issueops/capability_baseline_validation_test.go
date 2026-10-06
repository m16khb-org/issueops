package issueops_test

import (
	"fmt"
	model "issueops/internal/contract/issueops"
)

func validateCapabilityBaseline(baseline model.Baseline) error {
	if baseline.SchemaVersion != model.CapabilityBaselineSchemaVersion {
		return fmt.Errorf("schema_version must be %d", model.CapabilityBaselineSchemaVersion)
	}
	if baseline.Kind != model.BaselineKindDeterministicFixture && baseline.Kind != model.BaselineKindLiveObservation {
		return fmt.Errorf("baseline kind must be deterministic-fixture or live-observation")
	}
	if baseline.GeneratedAt == "" || baseline.AttemptID == "" || baseline.Machine == "" || baseline.RuntimeIdentity == "" {
		return fmt.Errorf("baseline identity requires generated_at, attempt_id, machine, and runtime_identity")
	}
	if err := validateCells(baseline.Kind, baseline.AttemptID, baseline.RuntimeIdentity, baseline.Cells); err != nil {
		return err
	}
	if err := validateHandOffs(baseline.HandOffs); err != nil {
		return err
	}
	if err := validateRecovery(baseline.Recovery); err != nil {
		return err
	}
	return nil
}

func validateCells(kind model.BaselineKind, baselineAttemptID, baselineRuntimeIdentity string, cells []model.Cell) error {
	if len(cells) != 12 {
		return fmt.Errorf("baseline must contain 12 launcher/host cells, got %d", len(cells))
	}
	seen := map[string]bool{}
	for i, cell := range cells {
		if !validLauncher(cell.Launcher) {
			return fmt.Errorf("cell %d launcher %q is invalid", i, cell.Launcher)
		}
		if !validHost(cell.Host) {
			return fmt.Errorf("cell %d host %q is invalid", i, cell.Host)
		}
		if !validStatus(cell.Status) {
			return fmt.Errorf("cell %s/%s status %q is invalid", cell.Launcher, cell.Host, cell.Status)
		}
		key := string(cell.Launcher) + "/" + string(cell.Host)
		if seen[key] {
			return fmt.Errorf("duplicate cell %s", key)
		}
		seen[key] = true
		if cell.SelectedLauncher != "" && cell.SelectedLauncher != cell.Launcher {
			return fmt.Errorf("cell %s selected launcher must match launcher", key)
		}
		if cell.ObservedLauncher != "" && cell.ObservedLauncher != cell.Launcher {
			return fmt.Errorf("cell %s silent fallback from %s to %s is forbidden", key, cell.SelectedLauncher, cell.ObservedLauncher)
		}
		if cell.FallbackLauncher != "" {
			return fmt.Errorf("cell %s fallback launcher %s is forbidden", key, cell.FallbackLauncher)
		}
		if err := validateEvidence(kind, baselineAttemptID, baselineRuntimeIdentity, key, cell.Status, cell.Evidence); err != nil {
			return err
		}
	}
	for _, launcher := range []model.Launcher{model.LauncherOrca, model.LauncherHerdr, model.LauncherCmux, model.LauncherDirect} {
		for _, host := range []model.Host{model.HostCodex, model.HostClaude, model.HostOmo} {
			key := string(launcher) + "/" + string(host)
			if !seen[key] {
				return fmt.Errorf("missing cell %s", key)
			}
		}
	}
	return nil
}

func validateEvidence(kind model.BaselineKind, baselineAttemptID, baselineRuntimeIdentity, key string, status model.Status, evidence model.Evidence) error {
	observations := map[string]model.Observation{
		"installed":        evidence.Installed,
		"connected":        evidence.Connected,
		"capable":          evidence.Capable,
		"runtime_verified": evidence.RuntimeVerified,
	}
	for name, observation := range observations {
		if err := validateObservation(key, name, observation); err != nil {
			return err
		}
	}
	runtime := evidence.RuntimeVerified
	if status != model.StatusSupported && runtime.Result != model.ObservationResultNotRun {
		return fmt.Errorf("cell %s non-supported status must not contain live runtime_verified evidence", key)
	}
	if status == model.StatusSupported {
		if kind != model.BaselineKindLiveObservation {
			return fmt.Errorf("cell %s status supported requires live-observation baseline kind", key)
		}
		if runtime.Result != model.ObservationResultPositive || !runtime.Observed || runtime.Claim != model.ClaimLive {
			return fmt.Errorf("cell %s status supported requires positive live runtime_verified evidence", key)
		}
		if runtime.AttemptID != baselineAttemptID {
			return fmt.Errorf("cell %s status supported runtime_verified must bind to baseline attempt", key)
		}
		if runtime.RuntimeIdentity != baselineRuntimeIdentity {
			return fmt.Errorf("cell %s status supported runtime_verified must bind to baseline runtime", key)
		}
	}
	if status == model.StatusUnavailable && evidence.Connected.Result != model.ObservationResultNegative && evidence.Capable.Result != model.ObservationResultNegative {
		return fmt.Errorf("cell %s status unavailable requires explicit negative connected or capable evidence", key)
	}
	if status == model.StatusUnsupported && evidence.Capable.Result != model.ObservationResultNegative {
		return fmt.Errorf("cell %s status unsupported requires explicit negative capable evidence", key)
	}
	if status == model.StatusNotRun {
		if runtime.Result != model.ObservationResultNotRun || runtime.Observed || runtime.Claim != model.ClaimNotRun {
			return fmt.Errorf("cell %s status not-run requires runtime_verified not-run evidence", key)
		}
	}
	return nil
}

func validateObservation(key, name string, observation model.Observation) error {
	if !validClaim(observation.Claim) {
		return fmt.Errorf("cell %s evidence %s claim %q is invalid", key, name, observation.Claim)
	}
	if !validObservationResult(observation.Result) {
		return fmt.Errorf("cell %s evidence %s result %q is invalid", key, name, observation.Result)
	}
	if observation.Result == model.ObservationResultNotRun {
		if observation.Observed || observation.Claim != model.ClaimNotRun {
			return fmt.Errorf("cell %s evidence %s not-run result requires not-run claim and observed=false", key, name)
		}
		return nil
	}
	if !observation.Observed {
		return fmt.Errorf("cell %s evidence %s explicit %s result requires observed=true", key, name, observation.Result)
	}
	if observation.Claim == model.ClaimNotRun {
		return fmt.Errorf("cell %s evidence %s explicit %s result cannot use not-run claim", key, name, observation.Result)
	}
	if observation.Version == "" || observation.ExecutablePath == "" || observation.RuntimeIdentity == "" || observation.ObservedAt == "" || observation.AttemptID == "" {
		return fmt.Errorf("cell %s evidence %s explicit observation requires version, executable_path, runtime_identity, observed_at, and attempt_id", key, name)
	}
	return nil
}

func validateHandOffs(handoffs []model.HandOff) error {
	if len(handoffs) != 6 {
		return fmt.Errorf("handoffs must contain exactly six directed cross-host material-transfer pairs, got %d", len(handoffs))
	}
	seen := map[string]bool{}
	for i, handoff := range handoffs {
		if !validHost(handoff.FromHost) || !validHost(handoff.ToHost) {
			return fmt.Errorf("handoff %d host must be codex, claude, or omo", i)
		}
		if handoff.FromHost == handoff.ToHost {
			return fmt.Errorf("self handoff %s->%s is forbidden", handoff.FromHost, handoff.ToHost)
		}
		if handoff.Semantics != model.HandOffMaterialTransfer {
			return fmt.Errorf("cross-host handoff %s->%s must be material transfer, not native session migration", handoff.FromHost, handoff.ToHost)
		}
		key := string(handoff.FromHost) + "->" + string(handoff.ToHost)
		if seen[key] {
			return fmt.Errorf("duplicate handoff %s", key)
		}
		seen[key] = true
	}
	for _, fromHost := range []model.Host{model.HostCodex, model.HostClaude, model.HostOmo} {
		for _, toHost := range []model.Host{model.HostCodex, model.HostClaude, model.HostOmo} {
			if fromHost == toHost {
				continue
			}
			key := string(fromHost) + "->" + string(toHost)
			if !seen[key] {
				return fmt.Errorf("missing directed cross-host handoff %s", key)
			}
		}
	}
	return nil
}

func validateRecovery(recovery model.RecoveryInvariants) error {
	direct := []model.RecoveryStep{model.RecoveryDirectReleased, model.RecoveryReplacePreview, model.RecoveryReturnedRecoveryChain, model.RecoveryClaimable, model.RecoveryClaim}
	if recovery.Direct.Mode != model.RecoveryModeDirect || !sameSteps(recovery.Direct.Steps, direct) {
		return fmt.Errorf("direct recovery must be released -> replace preview -> returned recovery chain -> claimable -> claim")
	}
	if !sameFailures(recovery.Direct.FailureCases, []model.RecoveryFailure{model.RecoveryFailureReleasedImmediateClaim, model.RecoveryFailureStaleGeneration}) {
		return fmt.Errorf("direct recovery failure cases must exactly match released-immediate claim and stale generation")
	}
	orca := []model.RecoveryStep{model.RecoveryOrcaReleased, model.RecoveryReplace, model.RecoveryReseed, model.RecoveryResume, model.RecoverySealedOwnerClaim}
	if recovery.Orca.Mode != model.RecoveryModeOrca || !sameSteps(recovery.Orca.Steps, orca) {
		return fmt.Errorf("orca recovery must be released -> replace -> reseed -> resume -> sealed owner claim")
	}
	if !sameFailures(recovery.Orca.FailureCases, []model.RecoveryFailure{model.RecoveryFailureReleasedImmediateClaim, model.RecoveryFailureStaleGeneration, model.RecoveryFailureDuplicateOrcaOwner, model.RecoveryFailureManualOrcaOwner}) {
		return fmt.Errorf("orca recovery failure cases must exactly match released-immediate claim, stale generation, duplicate owner, and manual owner")
	}
	return nil
}

func sameSteps(got, want []model.RecoveryStep) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func sameFailures(got, want []model.RecoveryFailure) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[model.RecoveryFailure]bool{}
	for _, failure := range want {
		set[failure] = true
	}
	seen := map[model.RecoveryFailure]bool{}
	for _, failure := range got {
		if !set[failure] || seen[failure] {
			return false
		}
		seen[failure] = true
	}
	return len(seen) == len(set)
}

func validLauncher(launcher model.Launcher) bool {
	switch launcher {
	case model.LauncherOrca, model.LauncherHerdr, model.LauncherCmux, model.LauncherDirect:
		return true
	default:
		return false
	}
}

func validHost(host model.Host) bool {
	switch host {
	case model.HostCodex, model.HostClaude, model.HostOmo:
		return true
	default:
		return false
	}
}

func validStatus(status model.Status) bool {
	switch status {
	case model.StatusSupported, model.StatusUnsupported, model.StatusUnavailable, model.StatusNotRun:
		return true
	default:
		return false
	}
}

func validObservationResult(result model.ObservationResult) bool {
	switch result {
	case model.ObservationResultPositive, model.ObservationResultNegative, model.ObservationResultNotRun:
		return true
	default:
		return false
	}
}

func validClaim(claim model.Claim) bool {
	switch claim {
	case model.ClaimMock, model.ClaimInstalled, model.ClaimLive, model.ClaimNotRun:
		return true
	default:
		return false
	}
}
