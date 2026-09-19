package issueops

const (
	IssueOpsHandoffSchemaVersion         = 1
	IssueOpsHandoffDeliverySchemaVersion = 1

	IssueOpsHandoffPhaseReleaseReadiness = "release-readiness"
	IssueOpsHandoffPhaseReceiveReadiness = "receive-readiness"

	IssueOpsHandoffTaskReader = "reader"
	IssueOpsHandoffTaskWriter = "writer"

	IssueOpsHandoffTaskKindBuild     = "build"
	IssueOpsHandoffTaskKindGolden    = "golden"
	IssueOpsHandoffTaskKindGenerator = "generator"
	IssueOpsHandoffTaskKindFormatter = "formatter"
	IssueOpsHandoffTaskKindFixture   = "fixture"
	IssueOpsHandoffTaskKindUnknown   = "unknown"

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

type IssueOpsHandoffSnapshot struct {
	SchemaVersion int                          `json:"schema_version"`
	Phase         string                       `json:"phase"`
	Sealed        IssueOpsHandoffSealed        `json:"sealed"`
	Current       IssueOpsHandoffCurrent       `json:"current"`
	Sender        IssueOpsHandoffSession       `json:"sender"`
	Receiver      IssueOpsHandoffSession       `json:"receiver"`
	Execution     IssueOpsHandoffExecution     `json:"execution"`
	UserDirective IssueOpsHandoffUserDirective `json:"user_directive"`
	Tasks         []IssueOpsHandoffTask        `json:"tasks,omitempty"`
	LateResults   []IssueOpsHandoffLateResult  `json:"late_results,omitempty"`
}

type IssueOpsHandoffSealed struct {
	BaseHead         string                    `json:"base_head"`
	FullHead         string                    `json:"full_head"`
	PlanDigest       string                    `json:"plan_digest"`
	MaterialDigest   string                    `json:"material_digest"`
	Material         IssueOpsHandoffMaterial   `json:"material"`
	RequiredEvidence []IssueOpsHandoffEvidence `json:"required_evidence,omitempty"`
}

type IssueOpsHandoffCurrent struct {
	FullHead             string                    `json:"full_head"`
	PlanDigest           string                    `json:"plan_digest"`
	MaterialDigest       string                    `json:"material_digest"`
	Evidence             []IssueOpsHandoffEvidence `json:"evidence,omitempty"`
	SharedStateRechecked bool                      `json:"shared_state_rechecked"`
	ReleaseCompleted     bool                      `json:"release_completed,omitempty"`
}

type IssueOpsHandoffEvidence struct {
	ID             string `json:"id"`
	Digest         string `json:"digest"`
	Head           string `json:"head,omitempty"`
	PlanDigest     string `json:"plan_digest,omitempty"`
	MaterialDigest string `json:"material_digest,omitempty"`
	InputRevision  string `json:"input_revision,omitempty"`
}

type IssueOpsHandoffMaterial struct {
	Purpose           string                        `json:"purpose"`
	NonGoals          []string                      `json:"non_goals,omitempty"`
	ApprovedEndpoint  string                        `json:"approved_endpoint"`
	SourceRoot        string                        `json:"source_root"`
	CanonicalWorktree string                        `json:"canonical_worktree"`
	PlanPath          string                        `json:"plan_path"`
	DiffDigest        string                        `json:"diff_digest"`
	Verification      []IssueOpsHandoffVerification `json:"verification,omitempty"`
	LifecycleState    string                        `json:"lifecycle_state"`
	ResumeCommands    []string                      `json:"resume_commands,omitempty"`
	ReadOnlyCommands  []string                      `json:"read_only_commands,omitempty"`
}

type IssueOpsHandoffVerification struct {
	ID             string `json:"id"`
	Input          string `json:"input"`
	Command        string `json:"command"`
	Timestamp      string `json:"timestamp"`
	Environment    string `json:"environment"`
	Failure        string `json:"failure,omitempty"`
	ResultLocation string `json:"result_location"`
}

type IssueOpsHandoffSession struct {
	Host           string `json:"host"`
	SessionID      string `json:"session_id"`
	ProcessReceipt string `json:"process_receipt"`
}

type IssueOpsHandoffExecution struct {
	Mode               ExecutionMode `json:"mode"`
	StatusValidated    bool          `json:"status_validated"`
	OrcaPacketPresent  bool          `json:"orca_packet_present"`
	OrcaRecoveryAction string        `json:"orca_recovery_action,omitempty"`
}

type IssueOpsHandoffUserDirective struct {
	MaterialVersion    int    `json:"material_version"`
	LatestVersion      int    `json:"latest_version"`
	CurrentInstruction string `json:"current_instruction"`
	Cancelled          bool   `json:"cancelled,omitempty"`
	ScopeChanged       bool   `json:"scope_changed,omitempty"`
}

type IssueOpsHandoffTask struct {
	ID                    string `json:"id"`
	Kind                  string `json:"kind,omitempty"`
	Classification        string `json:"classification"`
	Owner                 string `json:"owner"`
	ExecutionHandle       string `json:"execution_handle"`
	InputRevision         string `json:"input_revision"`
	WriteScope            string `json:"write_scope,omitempty"`
	ResultLocation        string `json:"result_location"`
	Live                  bool   `json:"live,omitempty"`
	DescendantsLive       bool   `json:"descendants_live,omitempty"`
	Terminated            bool   `json:"terminated,omitempty"`
	CancellationRequested bool   `json:"cancellation_requested,omitempty"`
}

type IssueOpsHandoffLateResult struct {
	ID                   string `json:"id"`
	SourceSessionID      string `json:"source_session_id"`
	InputRevision        string `json:"input_revision"`
	ResultLocation       string `json:"result_location"`
	ArrivedAfterRelease  bool   `json:"arrived_after_release"`
	AttemptsSourceChange bool   `json:"attempts_source_change,omitempty"`
}

type IssueOpsHandoffDecision struct {
	Phase            string   `json:"phase"`
	AllowRelease     bool     `json:"allow_release"`
	AllowReceive     bool     `json:"allow_receive"`
	ReusableEvidence []string `json:"reusable_evidence,omitempty"`
	SelectiveRecheck []string `json:"selective_recheck,omitempty"`
	Quarantine       []string `json:"quarantine,omitempty"`
	RejectReasons    []string `json:"reject_reasons,omitempty"`
}

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
}

type IssueOpsHandoffDeliveryRequest struct {
	DurableID string `json:"durable_id"`
}

type IssueOpsHandoffDeliveryLauncher struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	RuntimeID string `json:"runtime_id"`
	MachineID string `json:"machine_id"`
	ServerID  string `json:"server_id"`
}

type IssueOpsHandoffDeliveryTarget struct {
	TerminalID         string                `json:"terminal_id,omitempty"`
	PaneID             string                `json:"pane_id,omitempty"`
	ProcessIncarnation string                `json:"process_incarnation,omitempty"`
	Process            *NativeProcessReceipt `json:"process,omitempty"`
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
