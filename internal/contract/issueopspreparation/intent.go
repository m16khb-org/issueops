// Package issueopspreparation defines the stable execution-preparation
// contract shared by prepare, resume, and reconcile.
package issueopspreparation

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	leasecontract "issueops/internal/contract/issueopslease"
)

const (
	PurposePrepare = "prepare"
	PurposeResume  = "resume"

	InvocationNotInvoked  = "not_invoked_proven"
	InvocationUnknown     = "unknown"
	MaxInvocationAttempts = 2

	markerPrefix = "issueops-v1"
)

var (
	sha256Pattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[0-9a-f]{64}$`) })
	uuidPattern   = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	})
)

type LaunchIdentity struct {
	PromptPath          string `json:"prompt_path"`
	PromptSHA256        string `json:"prompt_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
}

type IntentStage string

const (
	IntentStageWorktree IntentStage = "worktree_create"
	IntentStageTerminal IntentStage = "terminal_create"
	IntentStageRun      IntentStage = "run_create"
	IntentStageRunBind  IntentStage = "run_bind"
	IntentStageTask     IntentStage = "task_create"
	IntentStageDispatch IntentStage = "dispatch"
)

type WorkspaceRequest struct {
	LifecycleID    string `json:"lifecycle_id"`
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"base_branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
	CWD            string `json:"-"`
}

type WorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	Exists         bool   `json:"exists,omitempty"`
}

type ProbeRequest struct {
	Repo      string           `json:"repo"`
	Host      string           `json:"host"`
	Model     string           `json:"model"`
	Effort    string           `json:"effort,omitempty"`
	Provider  string           `json:"provider,omitempty"`
	Issue     int              `json:"issue,omitempty"`
	Marker    string           `json:"marker"`
	Workspace WorkspaceRequest `json:"-"`
}

type OrcaWorkspaceReceipt struct {
	Workspace          WorkspaceReceipt `json:"workspace"`
	RuntimeID          string           `json:"runtime_id"`
	RepoID             string           `json:"repo_id"`
	WorktreeID         string           `json:"worktree_id"`
	WorktreeInstanceID string           `json:"worktree_instance_id,omitempty"`
}

type LaunchRequest struct {
	Prompt              string `json:"prompt"`
	PromptPath          string `json:"prompt_path"`
	PromptSHA256        string `json:"prompt_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
}

type IntentRequest struct {
	Stage               IntentStage           `json:"stage"`
	OperationID         string                `json:"operation_id,omitempty"`
	Generation          uint64                `json:"generation,omitempty"`
	OrcaRequestID       string                `json:"orca_request_id,omitempty"`
	OrcaPromptRequestID string                `json:"orca_prompt_request_id,omitempty"`
	Marker              string                `json:"marker"`
	Workspace           WorkspaceRequest      `json:"workspace"`
	Probe               ProbeRequest          `json:"probe"`
	Prepared            *OrcaWorkspaceReceipt `json:"prepared,omitempty"`
	Launch              *LaunchRequest        `json:"launch,omitempty"`
	TerminalPTYID       string                `json:"terminal_pty_id,omitempty"`
	RunID               string                `json:"run_id,omitempty"`
	RunBound            bool                  `json:"run_bound,omitempty"`
	TaskID              string                `json:"task_id,omitempty"`
	// RoleAgentArgs are the role-agent launch arguments for the terminal
	// stage. They are resolved before MarkInvoking and never sealed.
	RoleAgentArgs []string `json:"-"`
}

type IntentReceipt struct {
	Workspace      *OrcaWorkspaceReceipt `json:"workspace,omitempty"`
	TerminalPTYID  string                `json:"terminal_pty_id,omitempty"`
	TerminalHandle string                `json:"terminal_handle,omitempty"`
	RunID          string                `json:"run_id,omitempty"`
	RunBound       bool                  `json:"run_bound,omitempty"`
	TaskID         string                `json:"task_id,omitempty"`
	DispatchID     string                `json:"dispatch_id,omitempty"`
	RequestID      string                `json:"request_id,omitempty"`
	PromptReceipt  *PromptReceipt        `json:"prompt_receipt,omitempty"`
}

type PromptReceipt struct {
	RequestID               string   `json:"request_id"`
	Stages                  []string `json:"stages,omitempty"`
	Provider                string   `json:"provider,omitempty"`
	Observation             string   `json:"observation,omitempty"`
	ProcessIncarnation      string   `json:"process_incarnation,omitempty"`
	Generation              uint64   `json:"generation,omitempty"`
	BaselineWorkingSequence *uint64  `json:"baseline_working_sequence,omitempty"`
}

type IntentInventory struct {
	Candidates        []IntentReceipt `json:"candidates"`
	AuthoritativeZero bool            `json:"authoritative_zero,omitempty"`
	ExactReplay       bool            `json:"exact_replay,omitempty"`
}

type InvocationError struct {
	State string
	Cause error
}

func (err *InvocationError) Error() string { return err.Cause.Error() }
func (err *InvocationError) Unwrap() error { return err.Cause }

// Intent is the only persisted JSON shape for Orca prepare and resume work.
type Intent struct {
	SchemaVersion       int                   `json:"schema_version"`
	Purpose             string                `json:"purpose,omitempty"`
	OperationID         string                `json:"operation_id"`
	LifecycleID         string                `json:"lifecycle_id"`
	Generation          uint64                `json:"generation"`
	Stage               IntentStage           `json:"stage"`
	Marker              string                `json:"marker"`
	StartedAt           string                `json:"started_at"`
	InvocationState     string                `json:"invocation_state"`
	InvocationAttempts  int                   `json:"invocation_attempts"`
	OrcaRequestID       string                `json:"orca_request_id,omitempty"`
	OrcaPromptRequestID string                `json:"orca_prompt_request_id,omitempty"`
	Workspace           WorkspaceRequest      `json:"workspace"`
	Probe               ProbeRequest          `json:"probe"`
	Prepared            *OrcaWorkspaceReceipt `json:"prepared,omitempty"`
	Launch              *LaunchIdentity       `json:"launch,omitempty"`
	IssueBodySHA256     string                `json:"issue_body_sha256"`
	ClaimTokenSHA256    string                `json:"claim_token_sha256,omitempty"`
	TerminalPTYID       string                `json:"terminal_pty_id,omitempty"`
	RunID               string                `json:"run_id,omitempty"`
	RunBound            bool                  `json:"run_bound,omitempty"`
	TaskID              string                `json:"task_id,omitempty"`
	PriorBinding        *ResumeBinding        `json:"prior_binding,omitempty"`
	ResumeLease         *leasecontract.Lease  `json:"resume_lease,omitempty"`
}

// ResumeBinding preserves the stable resume-intent byte order. Persisted
// issueops_v1 records use leasecontract.OrcaBinding with a different stable
// byte order, so the two wire contracts cannot share one struct.
type ResumeBinding struct {
	RuntimeID          string `json:"runtime_id"`
	RepoID             string `json:"repo_id"`
	WorktreeID         string `json:"worktree_id"`
	WorktreeInstanceID string `json:"worktree_instance_id,omitempty"`
	LeaseGeneration    uint64 `json:"lease_generation,omitempty"`
	OwnerHost          string `json:"owner_host"`
	OwnerModel         string `json:"owner_model"`
	OwnerEffort        string `json:"owner_effort,omitempty"`
	RunID              string `json:"run_id,omitempty"`
	TaskID             string `json:"task_id"`
	DispatchID         string `json:"dispatch_id"`
	TerminalPTYID      string `json:"terminal_pty_id,omitempty"`
}

type IssueIdentity struct {
	Provider string
	Issue    int
}

type IssueLinkEvidence struct {
	Provider     string `json:"provider"`
	IssueURL     string `json:"issue_url"`
	LinkVerified bool   `json:"link_verified"`
}

func DecodeIssueLinkEvidence(raw json.RawMessage) *IssueLinkEvidence {
	var evidence IssueLinkEvidence
	if len(raw) == 0 || json.Unmarshal(raw, &evidence) != nil {
		return nil
	}
	return &evidence
}

type IntentCodec struct{}

type IntentError struct {
	Code   string
	Detail string
}

func (e *IntentError) Error() string {
	if strings.TrimSpace(e.Detail) == "" {
		return e.Code
	}
	return e.Code + ": " + strings.TrimSpace(e.Detail)
}

func (e *IntentError) IssueOpsErrorFields() map[string]any {
	return map[string]any{"code": e.Code}
}

func (IntentCodec) Decode(operationID string, raw []byte) (Intent, error) {
	intent, err := decodeShape(raw)
	if err != nil {
		return Intent{}, err
	}
	if err := validateIntent(intent, operationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

// DecodeSelfIdentified validates the payload against its own sealed operation ID.
func (IntentCodec) DecodeSelfIdentified(raw []byte) (Intent, error) {
	intent, err := decodeShape(raw)
	if err != nil {
		return Intent{}, err
	}
	if err := validateIntent(intent, intent.OperationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

func (IntentCodec) Validate(intent Intent, operationID string) error {
	return validateIntent(intent, operationID)
}

func (IntentCodec) Encode(intent Intent) ([]byte, error) {
	if err := validateIntent(intent, intent.OperationID); err != nil {
		return nil, err
	}
	return json.Marshal(intent)
}

func decodeShape(raw []byte) (Intent, error) {
	var intent Intent
	if err := json.Unmarshal(raw, &intent); err != nil {
		return Intent{}, fmt.Errorf("decode Orca external intent payload: %w", err)
	}
	if err := validateShape(intent, intent.OperationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

func validateIntent(intent Intent, operationID string) error {
	if err := validateShape(intent, operationID); err != nil {
		return err
	}
	identity, err := parseMarker(intent.Marker)
	if err != nil {
		return err
	}
	if identity.Purpose != intent.Purpose || identity.LifecycleID != intent.LifecycleID ||
		identity.Generation != intent.Generation || identity.OperationID != intent.OperationID ||
		identity.Provider != intent.Probe.Provider || identity.Issue != intent.Probe.Issue {
		return contractError("intent_identity_mismatch", "Orca intent marker does not match the sealed payload identity")
	}
	return nil
}

func validateShape(intent Intent, operationID string) error {
	if !intentEnvelopeValid(intent, operationID) || !intentProbeValid(intent) || !intentInvocationValid(intent) {
		return fmt.Errorf("Orca external intent payload is invalid")
	}
	if err := validatePurposeShape(intent); err != nil {
		return err
	}
	return validateStageShape(intent)
}

func intentEnvelopeValid(intent Intent, operationID string) bool {
	return intent.SchemaVersion == leasecontract.SchemaVersion && intent.OperationID == operationID &&
		intent.LifecycleID != "" && intent.Generation != 0 && intent.Marker != "" && intent.StartedAt != "" &&
		intent.Workspace.LifecycleID == intent.LifecycleID
}

func intentProbeValid(intent Intent) bool {
	host := intent.Probe.Host
	return intent.Probe.Marker == intent.Marker &&
		samePath(intent.Probe.Repo, intent.Workspace.SourceRoot) && strings.TrimSpace(intent.Probe.Model) != "" &&
		(host == "codex" || host == "claude" || host == "omo" || host == "omp")
}

func intentInvocationValid(intent Intent) bool {
	return (intent.InvocationState == InvocationNotInvoked || intent.InvocationState == InvocationUnknown) &&
		intent.InvocationAttempts >= 0 && intent.InvocationAttempts <= MaxInvocationAttempts &&
		sha256Pattern().MatchString(intent.IssueBodySHA256) &&
		(intent.OrcaRequestID == "" || uuidPattern().MatchString(intent.OrcaRequestID)) &&
		(intent.OrcaPromptRequestID == "" || uuidPattern().MatchString(intent.OrcaPromptRequestID))
}

func validatePurposeShape(intent Intent) error {
	switch intent.Purpose {
	case PurposePrepare:
		if intent.Generation != 1 || intent.PriorBinding != nil || intent.ResumeLease != nil {
			return fmt.Errorf("Orca prepare intent payload is invalid")
		}
	case PurposeResume:
		if !resumeShapeValid(intent) {
			return fmt.Errorf("Orca resume intent payload is invalid")
		}
	default:
		return fmt.Errorf("unsupported Orca external intent purpose %q", intent.Purpose)
	}
	return nil
}

func resumeShapeValid(intent Intent) bool {
	lease := intent.ResumeLease
	if intent.Stage == IntentStageWorktree || intent.PriorBinding == nil || lease == nil {
		return false
	}
	return lease.Generation == intent.Generation && lease.Status == "claimable" &&
		lease.Holder == nil && lease.ClaimTokenSHA256 == intent.ClaimTokenSHA256 &&
		sha256Pattern().MatchString(lease.ClaimTokenSHA256)
}

func validateStageShape(intent Intent) error {
	switch intent.Stage {
	case IntentStageWorktree:
		if intent.Prepared != nil || intent.Launch != nil || intent.ClaimTokenSHA256 != "" || intent.TerminalPTYID != "" || intent.RunID != "" || intent.RunBound || intent.TaskID != "" {
			return fmt.Errorf("Orca worktree intent payload contains later-stage receipts")
		}
	case IntentStageTerminal, IntentStageRun, IntentStageRunBind, IntentStageTask, IntentStageDispatch:
		if !ownerLaunchReceiptsValid(intent) {
			return fmt.Errorf("Orca owner intent payload is missing sealed launch receipts")
		}
		return validateOwnerStageProgress(intent)
	default:
		return fmt.Errorf("unsupported Orca external intent stage %q", intent.Stage)
	}
	return nil
}

func ownerLaunchReceiptsValid(intent Intent) bool {
	if intent.Prepared == nil || intent.Launch == nil || !sha256Pattern().MatchString(intent.ClaimTokenSHA256) {
		return false
	}
	launch := intent.Launch
	return sha256Pattern().MatchString(launch.PromptSHA256) && sha256Pattern().MatchString(launch.ContextPacketSHA256) &&
		strings.TrimSpace(launch.PromptPath) != "" && strings.TrimSpace(launch.ContextPacketPath) != "" &&
		validateWorkspaceReceipt(intent.Workspace, *intent.Prepared) == nil
}

type ownerStageProgress struct{ pty, run, bound, task bool }

var ownerStageProgressRules = map[IntentStage]struct {
	want    ownerStageProgress
	message string
}{
	IntentStageTerminal: {ownerStageProgress{}, "Orca terminal intent payload contains later-stage receipts"},
	IntentStageRun:      {ownerStageProgress{pty: true}, "Orca Run intent payload is incomplete"},
	IntentStageRunBind:  {ownerStageProgress{pty: true, run: true}, "Orca Run bind intent payload is incomplete"},
	IntentStageTask:     {ownerStageProgress{pty: true, run: true, bound: true}, "Orca task intent payload is incomplete"},
	IntentStageDispatch: {ownerStageProgress{pty: true, run: true, bound: true, task: true}, "Orca dispatch intent payload is incomplete"},
}

func validateOwnerStageProgress(intent Intent) error {
	rule, ok := ownerStageProgressRules[intent.Stage]
	if !ok {
		return nil
	}
	got := ownerStageProgress{pty: intent.TerminalPTYID != "", run: intent.RunID != "", bound: intent.RunBound, task: intent.TaskID != ""}
	if got != rule.want {
		return fmt.Errorf("%s", rule.message)
	}
	return nil
}

func validateWorkspaceReceipt(request WorkspaceRequest, receipt OrcaWorkspaceReceipt) error {
	workspace := receipt.Workspace
	if !samePath(workspace.SourceRoot, request.SourceRoot) || !samePath(workspace.Root, request.Root) ||
		workspace.Branch != request.Branch || workspace.BaseHead != request.BaseHead ||
		!sameOptionalPath(workspace.ParentWorktree, request.ParentWorktree) || workspace.Driver != "orca" ||
		strings.TrimSpace(receipt.RuntimeID) == "" || strings.TrimSpace(receipt.RepoID) == "" || strings.TrimSpace(receipt.WorktreeID) == "" {
		return fmt.Errorf("Orca workspace receipt does not match the sealed request")
	}
	return nil
}

type MarkerIdentity struct {
	Purpose, LifecycleID, OperationID, Provider string
	Generation                                  uint64
	Issue                                       int
}

func (codec IntentCodec) RenderMarker(identity MarkerIdentity) (string, error) {
	return renderMarker(identity)
}

func (IntentCodec) RenderReadinessMarker(lifecycleID string, issue IssueIdentity) (string, error) {
	if !validToken(lifecycleID) || !validProvider(issue.Provider) || issue.Issue <= 0 {
		return "", contractError("intent_marker_invalid", "Orca readiness marker identity is invalid")
	}
	return strings.Join([]string{
		markerPrefix, "lifecycle=" + lifecycleID,
		"provider=" + issue.Provider, "issue=" + strconv.Itoa(issue.Issue),
	}, " "), nil
}

func renderMarker(identity MarkerIdentity) (string, error) {
	if (identity.Purpose != PurposePrepare && identity.Purpose != PurposeResume) ||
		!validToken(identity.LifecycleID) || !validToken(identity.OperationID) || !validProvider(identity.Provider) || identity.Issue <= 0 ||
		(identity.Purpose == PurposePrepare && identity.Generation != 1) || (identity.Purpose == PurposeResume && identity.Generation == 0) {
		return "", contractError("intent_marker_invalid", "Orca intent marker identity is invalid")
	}
	fields := []string{markerPrefix}
	if identity.Purpose == PurposeResume {
		fields = append(fields, "resume")
	}
	fields = append(fields, "lifecycle="+identity.LifecycleID)
	if identity.Purpose == PurposeResume {
		fields = append(fields, "generation="+strconv.FormatUint(identity.Generation, 10))
	}
	fields = append(fields, "operation="+identity.OperationID, "provider="+identity.Provider, "issue="+strconv.Itoa(identity.Issue))
	return strings.Join(fields, " "), nil
}

func parseMarker(marker string) (MarkerIdentity, error) {
	fields := strings.Fields(marker)
	identity := MarkerIdentity{}
	var offset int
	switch {
	case len(fields) == 5 && fields[0] == "issueops-v1":
		identity.Purpose, identity.Generation, offset = PurposePrepare, 1, 1
	case len(fields) == 7 && fields[0] == "issueops-v1" && fields[1] == "resume":
		identity.Purpose, offset = PurposeResume, 2
	default:
		return MarkerIdentity{}, invalidMarker()
	}
	var err error
	if identity.LifecycleID, err = markerField(fields[offset], "lifecycle"); err != nil {
		return MarkerIdentity{}, err
	}
	if identity.Purpose == PurposeResume {
		raw, fieldErr := markerField(fields[offset+1], "generation")
		if fieldErr != nil {
			return MarkerIdentity{}, fieldErr
		}
		identity.Generation, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return MarkerIdentity{}, invalidMarker()
		}
		offset++
	}
	if err := parseMarkerTail(&identity, fields[offset+1:]); err != nil {
		return MarkerIdentity{}, err
	}
	rendered, err := renderMarker(identity)
	if err != nil || rendered != marker {
		return MarkerIdentity{}, invalidMarker()
	}
	return identity, nil
}

// parseMarkerTail은 operation/provider/issue 세 필드를 순서대로 읽는다.
func parseMarkerTail(identity *MarkerIdentity, fields []string) error {
	var err error
	if identity.OperationID, err = markerField(fields[0], "operation"); err != nil {
		return err
	}
	if identity.Provider, err = markerField(fields[1], "provider"); err != nil {
		return err
	}
	rawIssue, err := markerField(fields[2], "issue")
	if err != nil {
		return err
	}
	identity.Issue, err = strconv.Atoi(rawIssue)
	if err != nil {
		return invalidMarker()
	}
	return nil
}

func markerField(field, name string) (string, error) {
	prefix := name + "="
	if !strings.HasPrefix(field, prefix) || !validToken(strings.TrimPrefix(field, prefix)) {
		return "", invalidMarker()
	}
	return strings.TrimPrefix(field, prefix), nil
}

func samePath(left, right string) bool {
	a, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(a); resolveErr == nil {
		a = resolved
	}
	b, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(b); resolveErr == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func sameOptionalPath(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	return samePath(left, right)
}

func validToken(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && !strings.ContainsRune(value, '=') &&
		!strings.ContainsFunc(value, unicode.IsSpace)
}
func validProvider(value string) bool         { return value == "github" || value == "gitlab" }
func contractError(code, detail string) error { return &IntentError{Code: code, Detail: detail} }
func invalidMarker() error {
	return contractError("intent_marker_invalid", "Orca intent marker is not canonical")
}
