package issueops

const (
	IssueOpsHandoffDeliverySchemaVersion = 1

	IssueOpsHandoffDeliveryStateNotObserved = "not_observed"
	IssueOpsHandoffDeliveryStateObserved    = "observed"

	IssueOpsHandoffDeliveryLauncherDirect = "direct"
	IssueOpsHandoffDeliveryLauncherOrca   = "orca"
	IssueOpsHandoffDeliveryLauncherHerdr  = "herdr"
	IssueOpsHandoffDeliveryLauncherCmux   = "cmux"

	IssueOpsHandoffDeliveryEvidenceLauncherReceipt      = "launcher_receipt"
	IssueOpsHandoffDeliveryEvidenceNativeReceipt        = "native_receipt"
	IssueOpsHandoffDeliveryEvidenceIssueOpsClaim        = "issueops_claim"
	IssueOpsHandoffDeliveryEvidenceLauncherAccepted     = "launcher_accepted"
	IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost = "accepted_response_lost"
	IssueOpsHandoffDeliveryEvidenceOrcaDispatch         = "orca_dispatch"
	IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt  = "orca_dispatch_receipt"
	IssueOpsHandoffDeliveryEvidenceOmoSendFailed        = "omo_send_failed"
	IssueOpsHandoffDeliveryEvidenceOmoSendAccepted      = "omo_send_accepted"
	IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost  = "omo_send_response_lost"
	IssueOpsHandoffDeliveryEvidenceRawInput             = "raw_input"
	IssueOpsHandoffDeliveryEvidenceHerdrWaitState       = "herdr_wait_state"
	IssueOpsHandoffDeliveryEvidenceTimeout              = "timeout"
	IssueOpsHandoffDeliveryEvidenceAgentPromptStalled   = "agent_prompt_stalled"
	IssueOpsHandoffDeliveryEvidenceExternalCallStaged   = "external_call_staged"
)

type IssueOpsHandoffDeliveryObservation struct {
	SchemaVersion      int                               `json:"schema_version"`
	AttemptID          string                            `json:"attempt_id"`
	LineageID          string                            `json:"lineage_id"`
	LifecycleID        string                            `json:"lifecycle_id"`
	PromptSHA256       string                            `json:"prompt_sha256"`
	MaterialSHA256     string                            `json:"material_sha256"`
	Request            IssueOpsHandoffDeliveryRequest    `json:"request"`
	Launcher           IssueOpsHandoffDeliveryLauncher   `json:"launcher"`
	Target             IssueOpsHandoffDeliveryTarget     `json:"target"`
	ExpectedOwnerHost  string                            `json:"expected_owner_host,omitempty"`
	OwnerActor         *NativeActor                      `json:"owner_actor,omitempty"`
	SourceGeneration   uint64                            `json:"source_generation"`
	CreatedAt          string                            `json:"created_at"`
	UpdatedAt          string                            `json:"updated_at"`
	Receipt            IssueOpsHandoffDeliveryReceipt    `json:"receipt"`
	CallStaged         IssueOpsHandoffDeliveryState      `json:"call_staged,omitempty"`
	InputAccepted      IssueOpsHandoffDeliveryState      `json:"input_accepted"`
	NativeTurnObserved IssueOpsHandoffDeliveryState      `json:"native_turn_observed"`
	OwnerClaimed       IssueOpsHandoffDeliveryState      `json:"owner_claimed"`
	Ambiguous          IssueOpsHandoffDeliveryState      `json:"ambiguous"`
	OwnerClaim         IssueOpsHandoffDeliveryOwnerClaim `json:"owner_claim,omitempty"`
	Timing             *IssueOpsHandoffDeliveryTiming    `json:"timing,omitempty"`
}

type IssueOpsHandoffDeliveryRequest struct {
	DurableID string `json:"durable_id"`
}

type IssueOpsHandoffDeliveryLauncher struct {
	Name                string                                      `json:"name"`
	Version             string                                      `json:"version"`
	Path                string                                      `json:"path"`
	RuntimeID           string                                      `json:"runtime_id,omitempty"`
	MachineID           string                                      `json:"machine_id,omitempty"`
	ServerID            string                                      `json:"server_id,omitempty"`
	EndpointIncarnation *IssueOpsHandoffDeliveryEndpointIncarnation `json:"endpoint_incarnation,omitempty"`
}

type IssueOpsHandoffDeliveryEndpointIncarnation struct {
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

type IssueOpsHandoffDeliveryTarget struct {
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

type IssueOpsHandoffDeliveryTiming struct {
	PreflightMS       uint64 `json:"preflight_ms,omitempty"`
	WorkspaceCreateMS uint64 `json:"workspace_create_ms,omitempty"`
	TargetResolveMS   uint64 `json:"target_resolve_ms,omitempty"`
	InputSendMS       uint64 `json:"input_send_ms,omitempty"`
	ReceiverReceiptMS uint64 `json:"receiver_receipt_ms,omitempty"`
}

type IssueOpsHandoffDeliveryReceipt struct {
	Location string `json:"location"`
	Digest   string `json:"digest"`
}

type IssueOpsHandoffDeliveryState struct {
	Status     string `json:"status"`
	ObservedAt string `json:"observed_at,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
}

type IssueOpsHandoffDeliveryOwnerClaim struct {
	Claimed    bool        `json:"claimed,omitempty"`
	Generation uint64      `json:"generation,omitempty"`
	Actor      NativeActor `json:"actor,omitempty"`
	ClaimedAt  string      `json:"claimed_at,omitempty"`
}

type IssueOpsHandoffDeliveryDecision struct {
	Accepted        bool     `json:"accepted"`
	OwnerAuthorized bool     `json:"owner_authorized"`
	RetryAuthorized bool     `json:"retry_authorized"`
	RejectReasons   []string `json:"reject_reasons,omitempty"`
}
