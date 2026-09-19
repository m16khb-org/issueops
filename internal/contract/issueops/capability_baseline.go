package issueops

import (
	"fmt"
)

const CapabilityBaselineSchemaVersion = 1

type Launcher string

const (
	LauncherOrca   Launcher = "orca"
	LauncherHerdr  Launcher = "herdr"
	LauncherCmux   Launcher = "cmux"
	LauncherDirect Launcher = "direct"
)

type Host string

const (
	HostCodex  Host = "codex"
	HostClaude Host = "claude"
	HostOmo    Host = "omo"
)

type Status string

const (
	StatusSupported   Status = "supported"
	StatusUnsupported Status = "unsupported"
	StatusUnavailable Status = "unavailable"
	StatusNotRun      Status = "not-run"
)

type Claim string

const (
	ClaimMock      Claim = "mock"
	ClaimInstalled Claim = "installed"
	ClaimLive      Claim = "live"
	ClaimNotRun    Claim = "not-run"
)

type Baseline struct {
	SchemaVersion int                `json:"schema_version"`
	Kind          BaselineKind       `json:"kind"`
	GeneratedAt   string             `json:"generated_at"`
	AttemptID     string             `json:"attempt_id"`
	Machine       string             `json:"machine"`
	Cells         []Cell             `json:"cells"`
	HandOffs      []HandOff          `json:"handoffs"`
	Recovery      RecoveryInvariants `json:"recovery"`
}

type BaselineKind string

const (
	BaselineKindDeterministicFixture BaselineKind = "deterministic-fixture"
	BaselineKindLiveObservation      BaselineKind = "live-observation"
)

type Cell struct {
	Launcher         Launcher `json:"launcher"`
	Host             Host     `json:"host"`
	Status           Status   `json:"status"`
	SelectedLauncher Launcher `json:"selected_launcher"`
	ObservedLauncher Launcher `json:"observed_launcher"`
	FallbackLauncher Launcher `json:"fallback_launcher,omitempty"`
	Evidence         Evidence `json:"evidence"`
	Notes            []string `json:"notes,omitempty"`
}

type Evidence struct {
	Installed       Observation `json:"installed"`
	Connected       Observation `json:"connected"`
	Capable         Observation `json:"capable"`
	RuntimeVerified Observation `json:"runtime_verified"`
}

type Observation struct {
	Claim           Claim  `json:"claim"`
	Observed        bool   `json:"observed"`
	Version         string `json:"version,omitempty"`
	ExecutablePath  string `json:"executable_path,omitempty"`
	RuntimeIdentity string `json:"runtime_identity,omitempty"`
	ObservedAt      string `json:"observed_at,omitempty"`
	AttemptID       string `json:"attempt_id,omitempty"`
	Detail          string `json:"detail,omitempty"`
}

type HandOffSemantics string

const (
	HandOffMaterialTransfer       HandOffSemantics = "material-transfer"
	HandOffNativeSessionMigration HandOffSemantics = "native-session-migration"
)

type HandOff struct {
	FromHost  Host             `json:"from_host"`
	ToHost    Host             `json:"to_host"`
	Semantics HandOffSemantics `json:"semantics"`
}

type RecoveryMode string

const (
	RecoveryModeDirect RecoveryMode = "direct"
	RecoveryModeOrca   RecoveryMode = "orca"
)

type RecoveryStep string

const (
	RecoveryDirectReleased        RecoveryStep = "direct-released"
	RecoveryOrcaReleased          RecoveryStep = "orca-released"
	RecoveryReplacePreview        RecoveryStep = "replace-preview"
	RecoveryReturnedRecoveryChain RecoveryStep = "returned-recovery-chain"
	RecoveryReplace               RecoveryStep = "replace"
	RecoveryReseed                RecoveryStep = "reseed"
	RecoveryResume                RecoveryStep = "resume"
	RecoveryClaimable             RecoveryStep = "claimable"
	RecoveryClaim                 RecoveryStep = "claim"
	RecoverySealedOwnerClaim      RecoveryStep = "sealed-owner-claim"
)

type RecoveryFailure string

const (
	RecoveryFailureReleasedImmediateClaim RecoveryFailure = "released-immediate-claim"
	RecoveryFailureStaleGeneration        RecoveryFailure = "stale-generation"
	RecoveryFailureDuplicateOrcaOwner     RecoveryFailure = "duplicate-orca-owner"
	RecoveryFailureManualOrcaOwner        RecoveryFailure = "manual-orca-owner"
)

type RecoveryInvariants struct {
	Direct RecoveryChain `json:"direct"`
	Orca   RecoveryChain `json:"orca"`
}

type RecoveryChain struct {
	Mode         RecoveryMode      `json:"mode"`
	Steps        []RecoveryStep    `json:"steps"`
	FailureCases []RecoveryFailure `json:"failure_cases"`
}

func Validate(baseline Baseline) error {
	if baseline.SchemaVersion != CapabilityBaselineSchemaVersion {
		return fmt.Errorf("schema_version must be %d", CapabilityBaselineSchemaVersion)
	}
	if baseline.Kind != BaselineKindDeterministicFixture && baseline.Kind != BaselineKindLiveObservation {
		return fmt.Errorf("baseline kind must be deterministic-fixture or live-observation")
	}
	if baseline.GeneratedAt == "" || baseline.AttemptID == "" || baseline.Machine == "" {
		return fmt.Errorf("baseline identity requires generated_at, attempt_id, and machine")
	}
	if err := validateCells(baseline.Cells); err != nil {
		return err
	}
	for i, handoff := range baseline.HandOffs {
		if !validHost(handoff.FromHost) || !validHost(handoff.ToHost) {
			return fmt.Errorf("handoff %d host must be codex, claude, or omo", i)
		}
		if handoff.FromHost != handoff.ToHost && handoff.Semantics != HandOffMaterialTransfer {
			return fmt.Errorf("cross-host handoff %s->%s must be material transfer, not native session migration", handoff.FromHost, handoff.ToHost)
		}
	}
	if err := validateRecovery(baseline.Recovery); err != nil {
		return err
	}
	return nil
}

func validateCells(cells []Cell) error {
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
		if err := validateEvidence(key, cell.Status, cell.Evidence); err != nil {
			return err
		}
	}
	for _, launcher := range []Launcher{LauncherOrca, LauncherHerdr, LauncherCmux, LauncherDirect} {
		for _, host := range []Host{HostCodex, HostClaude, HostOmo} {
			key := string(launcher) + "/" + string(host)
			if !seen[key] {
				return fmt.Errorf("missing cell %s", key)
			}
		}
	}
	return nil
}

func validateEvidence(key string, status Status, evidence Evidence) error {
	observations := map[string]Observation{
		"installed":        evidence.Installed,
		"connected":        evidence.Connected,
		"capable":          evidence.Capable,
		"runtime_verified": evidence.RuntimeVerified,
	}
	for name, observation := range observations {
		if !validClaim(observation.Claim) {
			return fmt.Errorf("cell %s evidence %s claim %q is invalid", key, name, observation.Claim)
		}
		if observation.Observed && observation.Claim != ClaimNotRun {
			if observation.Version == "" || observation.ExecutablePath == "" || observation.RuntimeIdentity == "" || observation.ObservedAt == "" || observation.AttemptID == "" {
				return fmt.Errorf("cell %s evidence %s live or installed observation requires version, executable_path, runtime_identity, observed_at, and attempt_id", key, name)
			}
		}
	}
	if status == StatusSupported {
		runtime := evidence.RuntimeVerified
		if !runtime.Observed || runtime.Claim != ClaimLive {
			return fmt.Errorf("cell %s status supported requires live runtime_verified evidence", key)
		}
	}
	if status == StatusUnavailable && evidence.Connected.Observed && evidence.Capable.Observed {
		return fmt.Errorf("cell %s status unavailable requires connected or capable evidence to be false", key)
	}
	if status == StatusUnsupported && evidence.Capable.Observed {
		return fmt.Errorf("cell %s status unsupported requires capable evidence to be false", key)
	}
	if status == StatusNotRun {
		runtime := evidence.RuntimeVerified
		if runtime.Observed || runtime.Claim != ClaimNotRun {
			return fmt.Errorf("cell %s status not-run requires runtime_verified not-run evidence", key)
		}
	}
	return nil
}

func validateRecovery(recovery RecoveryInvariants) error {
	direct := []RecoveryStep{RecoveryDirectReleased, RecoveryReplacePreview, RecoveryReturnedRecoveryChain, RecoveryClaimable, RecoveryClaim}
	if recovery.Direct.Mode != RecoveryModeDirect || !sameSteps(recovery.Direct.Steps, direct) {
		return fmt.Errorf("direct recovery must be released -> replace preview -> returned recovery chain -> claimable -> claim")
	}
	if !containsFailures(recovery.Direct.FailureCases, []RecoveryFailure{RecoveryFailureReleasedImmediateClaim, RecoveryFailureStaleGeneration}) {
		return fmt.Errorf("direct recovery must reject released-immediate claim and stale generation")
	}
	orca := []RecoveryStep{RecoveryOrcaReleased, RecoveryReplace, RecoveryReseed, RecoveryResume, RecoverySealedOwnerClaim}
	if recovery.Orca.Mode != RecoveryModeOrca || !sameSteps(recovery.Orca.Steps, orca) {
		return fmt.Errorf("orca recovery must be released -> replace -> reseed -> resume -> sealed owner claim")
	}
	if !containsFailures(recovery.Orca.FailureCases, []RecoveryFailure{RecoveryFailureReleasedImmediateClaim, RecoveryFailureStaleGeneration, RecoveryFailureDuplicateOrcaOwner, RecoveryFailureManualOrcaOwner}) {
		return fmt.Errorf("orca recovery must reject released-immediate claim, stale generation, duplicate owner, and manual owner")
	}
	return nil
}

func sameSteps(got, want []RecoveryStep) bool {
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

func containsFailures(got, want []RecoveryFailure) bool {
	set := map[RecoveryFailure]bool{}
	for _, failure := range got {
		set[failure] = true
	}
	for _, failure := range want {
		if !set[failure] {
			return false
		}
	}
	return true
}

func validLauncher(launcher Launcher) bool {
	switch launcher {
	case LauncherOrca, LauncherHerdr, LauncherCmux, LauncherDirect:
		return true
	default:
		return false
	}
}

func validHost(host Host) bool {
	switch host {
	case HostCodex, HostClaude, HostOmo:
		return true
	default:
		return false
	}
}

func validStatus(status Status) bool {
	switch status {
	case StatusSupported, StatusUnsupported, StatusUnavailable, StatusNotRun:
		return true
	default:
		return false
	}
}

func validClaim(claim Claim) bool {
	switch claim {
	case ClaimMock, ClaimInstalled, ClaimLive, ClaimNotRun:
		return true
	default:
		return false
	}
}
