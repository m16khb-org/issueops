package issueops

// The HandoffDelivery* constants are the vocabulary of handoff delivery
// observations: the schema version, the not_observed/observed states, the
// launcher names, and the evidence kinds that justify an observed state.
const (
	HandoffDeliverySchemaVersion = 1

	HandoffDeliveryStateNotObserved = "not_observed"
	HandoffDeliveryStateObserved    = "observed"

	HandoffDeliveryLauncherDirect = "direct"
	HandoffDeliveryLauncherHerdr  = "herdr"
	HandoffDeliveryLauncherCmux   = "cmux"

	HandoffDeliveryEvidenceLauncherReceipt      = "launcher_receipt"
	HandoffDeliveryEvidenceNativeReceipt        = "native_receipt"
	HandoffDeliveryEvidenceIssueOpsClaim        = "issueops_claim"
	HandoffDeliveryEvidenceLauncherAccepted     = "launcher_accepted"
	HandoffDeliveryEvidenceAcceptedResponseLost = "accepted_response_lost"
	HandoffDeliveryEvidenceRawInput             = "raw_input"
	HandoffDeliveryEvidenceHerdrWaitState       = "herdr_wait_state"
	HandoffDeliveryEvidenceTimeout              = "timeout"
	HandoffDeliveryEvidenceAgentPromptStalled   = "agent_prompt_stalled"
	HandoffDeliveryEvidenceExternalCallStaged   = "external_call_staged"
)

// HandoffDeliveryObservation is the durable record of one attempt to hand a
// cycle to a new agent session. Each stage (call staged, input accepted, native
// turn observed, owner claimed) is its own HandoffDeliveryState, so a stage that
// was never seen is not mistaken for one that failed.
type HandoffDeliveryObservation struct {
	SchemaVersion      int                       `json:"schema_version"`
	AttemptID          string                    `json:"attempt_id"`
	LineageID          string                    `json:"lineage_id"`
	LifecycleID        string                    `json:"lifecycle_id"`
	PromptSHA256       string                    `json:"prompt_sha256"`
	MaterialSHA256     string                    `json:"material_sha256"`
	Request            HandoffDeliveryRequest    `json:"request"`
	Launcher           HandoffDeliveryLauncher   `json:"launcher"`
	Target             HandoffDeliveryTarget     `json:"target"`
	ExpectedOwnerHost  string                    `json:"expected_owner_host,omitempty"`
	OwnerActor         *NativeActor              `json:"owner_actor,omitempty"`
	SourceGeneration   uint64                    `json:"source_generation"`
	CreatedAt          string                    `json:"created_at"`
	UpdatedAt          string                    `json:"updated_at"`
	Receipt            HandoffDeliveryReceipt    `json:"receipt"`
	CallStaged         HandoffDeliveryState      `json:"call_staged"`
	InputAccepted      HandoffDeliveryState      `json:"input_accepted"`
	NativeTurnObserved HandoffDeliveryState      `json:"native_turn_observed"`
	OwnerClaimed       HandoffDeliveryState      `json:"owner_claimed"`
	Ambiguous          HandoffDeliveryState      `json:"ambiguous"`
	OwnerClaim         HandoffDeliveryOwnerClaim `json:"owner_claim"`
	Timing             *HandoffDeliveryTiming    `json:"timing,omitempty"`
}

// HandoffDeliveryRequest identifies the durable handoff the delivery belongs to.
type HandoffDeliveryRequest struct {
	DurableID string `json:"durable_id"`
}

// HandoffDeliveryLauncher identifies the terminal multiplexer that delivered the
// handoff and, when available, the server instance behind its endpoint.
type HandoffDeliveryLauncher struct {
	Name                string                              `json:"name"`
	Version             string                              `json:"version"`
	Path                string                              `json:"path"`
	RuntimeID           string                              `json:"runtime_id,omitempty"`
	MachineID           string                              `json:"machine_id,omitempty"`
	ServerID            string                              `json:"server_id,omitempty"`
	EndpointIncarnation *HandoffDeliveryEndpointIncarnation `json:"endpoint_incarnation,omitempty"`
}

// HandoffDeliveryEndpointIncarnation is the filesystem identity (device, inode,
// ctime, ownership, mode) of a launcher socket and its parent directory, so a
// replaced or hijacked endpoint can be told apart from the original.
type HandoffDeliveryEndpointIncarnation struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
	CTimeNS        int64  `json:"ctime_ns"`
	OwnerUID       uint32 `json:"owner_uid"`
	OwnerGID       uint32 `json:"owner_gid"`
	Mode           uint32 `json:"mode"`
	ParentPath     string `json:"parent_path"`
	ParentDevice   uint64 `json:"parent_device"`
	ParentInode    uint64 `json:"parent_inode"`
	ParentOwnerUID uint32 `json:"parent_owner_uid"`
	ParentOwnerGID uint32 `json:"parent_owner_gid"`
	ParentMode     uint32 `json:"parent_mode"`
}

// HandoffDeliveryTarget identifies where the handoff prompt was delivered:
// terminal, pane, window, workspace or surface, the working directory, and the
// agent process that received it.
type HandoffDeliveryTarget struct {
	TerminalID              string                `json:"terminal_id,omitempty"`
	PaneID                  string                `json:"pane_id,omitempty"`
	WindowID                string                `json:"window_id,omitempty"`
	WorkspaceID             string                `json:"workspace_id,omitempty"`
	SurfaceID               string                `json:"surface_id,omitempty"`
	CWD                     string                `json:"cwd,omitempty"`
	ProcessIncarnation      string                `json:"process_incarnation,omitempty"`
	PromptGeneration        *uint64               `json:"prompt_generation,omitempty"`
	BaselineWorkingSequence *uint64               `json:"baseline_working_sequence,omitempty"`
	Process                 *NativeProcessReceipt `json:"process,omitempty"`
}

// HandoffDeliveryTiming holds the elapsed milliseconds of each delivery stage.
type HandoffDeliveryTiming struct {
	PreflightMS       uint64 `json:"preflight_ms,omitempty"`
	WorkspaceCreateMS uint64 `json:"workspace_create_ms,omitempty"`
	TargetResolveMS   uint64 `json:"target_resolve_ms,omitempty"`
	InputSendMS       uint64 `json:"input_send_ms,omitempty"`
	ReceiverReceiptMS uint64 `json:"receiver_receipt_ms,omitempty"`
}

// HandoffDeliveryReceipt locates the receipt for a delivery and the digest that
// binds its contents.
type HandoffDeliveryReceipt struct {
	Location string `json:"location"`
	Digest   string `json:"digest"`
}

// HandoffDeliveryState is the observed status of one delivery stage, with the
// time it was observed and the evidence kind that supports it.
type HandoffDeliveryState struct {
	Status     string `json:"status"`
	ObservedAt string `json:"observed_at,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
}

// HandoffDeliveryOwnerClaim records whether the new session claimed ownership of
// the cycle, at which lease generation, and as which actor.
type HandoffDeliveryOwnerClaim struct {
	Claimed    bool        `json:"claimed,omitempty"`
	Generation uint64      `json:"generation,omitempty"`
	Actor      NativeActor `json:"actor"`
	ClaimedAt  string      `json:"claimed_at,omitempty"`
}

// HandoffDeliveryDecision is the verdict on a delivery observation: whether it
// was accepted, and the reasons it was rejected otherwise.
type HandoffDeliveryDecision struct {
	Accepted      bool     `json:"accepted"`
	RejectReasons []string `json:"reject_reasons,omitempty"`
}
