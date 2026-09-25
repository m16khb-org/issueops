package issueops

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
	SchemaVersion   int                `json:"schema_version"`
	Kind            BaselineKind       `json:"kind"`
	GeneratedAt     string             `json:"generated_at"`
	AttemptID       string             `json:"attempt_id"`
	Machine         string             `json:"machine"`
	RuntimeIdentity string             `json:"runtime_identity"`
	Cells           []Cell             `json:"cells"`
	HandOffs        []HandOff          `json:"handoffs"`
	Recovery        RecoveryInvariants `json:"recovery"`
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
	Claim           Claim             `json:"claim"`
	Result          ObservationResult `json:"result"`
	Observed        bool              `json:"observed"`
	Version         string            `json:"version,omitempty"`
	ExecutablePath  string            `json:"executable_path,omitempty"`
	RuntimeIdentity string            `json:"runtime_identity,omitempty"`
	ObservedAt      string            `json:"observed_at,omitempty"`
	AttemptID       string            `json:"attempt_id,omitempty"`
	Detail          string            `json:"detail,omitempty"`
}

type ObservationResult string

const (
	ObservationResultPositive ObservationResult = "positive"
	ObservationResultNegative ObservationResult = "negative"
	ObservationResultNotRun   ObservationResult = "not-run"
)

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
