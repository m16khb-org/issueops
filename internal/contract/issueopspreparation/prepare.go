package issueopspreparation

import (
	"encoding/json"

	leasecontract "issueops/internal/contract/issueopslease"
)

const (
	ModeAuto   = "auto"
	ModeDirect = "direct"
	ModeOrca   = "orca"

	// IssueOps implementer(하위 세션 execution owner)의 host별 기본 모델.
	// execution prepare가 --owner-model/--owner-effort 미지정 호출에 적용한다.
	ImplementerModelCodex  = "gpt-5.6-terra"
	ImplementerEffortCodex = "xhigh"
	// Claude Code 자동 체인은 Opus 5 planner가 계획·리뷰하고 Sonnet 5
	// implementer가 실행한다. Fable 5는 명시적 수동 지정에만 사용한다.
	ImplementerModelClaude = "claude-sonnet-5"
	// claude CLI의 --effort <level> 플래그 실지원을 확인함(2026-07-24).
	// 플래그가 제거되면 ownerAgentCommand(adapter/orca/client.go)의 claude
	// 분기에서 effort 인자를 조건부 생략으로 되돌린다.
	ImplementerEffortClaude = "high"
	ImplementerModelOmo     = "openai-codex/gpt-5.6-sol"
	ImplementerEffortOmo    = "max"
)

type Actor = leasecontract.Actor
type Record = leasecontract.Record
type Execution = leasecontract.Execution
type Workspace = leasecontract.Workspace

func ImplementerDefaults(host string) (model string, effort string, ok bool) {
	switch host {
	case "codex":
		return ImplementerModelCodex, ImplementerEffortCodex, true
	case "claude":
		return ImplementerModelClaude, ImplementerEffortClaude, true
	case "omo":
		return ImplementerModelOmo, ImplementerEffortOmo, true
	default:
		return "", "", false
	}
}

type Command struct {
	ID                           string `json:"id"`
	Mode                         string `json:"mode"`
	Actor                        Actor  `json:"actor"`
	CWD                          string `json:"cwd"`
	OwnerHost                    string `json:"owner_host,omitempty"`
	OwnerModel                   string `json:"owner_model,omitempty"`
	OwnerEffort                  string `json:"owner_effort,omitempty"`
	IssueSnapshotFile            string `json:"issue_snapshot_file,omitempty"`
	DirectReason                 string `json:"direct_reason,omitempty"`
	ExpectedReadinessFingerprint string `json:"expected_readiness_fingerprint,omitempty"`
	Confirm                      bool   `json:"confirm,omitempty"`
}

func (command Command) Clone() Command {
	cloned := command
	cloned.Actor = cloneActor(command.Actor)
	return cloned
}

type Result struct {
	OK                   bool                     `json:"ok"`
	ID                   string                   `json:"id"`
	Preview              bool                     `json:"preview,omitempty"`
	RequestedMode        string                   `json:"requested_mode"`
	ResolvedMode         string                   `json:"resolved_mode"`
	FallbackCode         string                   `json:"fallback_code,omitempty"`
	ProbeAttempted       bool                     `json:"probe_attempted"`
	ProbeAvailable       bool                     `json:"probe_available"`
	ProbeReady           bool                     `json:"probe_ready"`
	ProbeCode            string                   `json:"probe_code,omitempty"`
	ReadinessFingerprint string                   `json:"readiness_fingerprint,omitempty"`
	ExplicitDirectReason string                   `json:"explicit_direct_reason,omitempty"`
	Workspace            Workspace                `json:"workspace"`
	Execution            *leasecontract.Execution `json:"execution,omitempty"`
	ClaimTokenPath       string                   `json:"claim_token_path,omitempty"`
	IssueBodySHA256      string                   `json:"issue_body_sha256,omitempty"`
	ContextPacketPath    string                   `json:"context_packet_path,omitempty"`
	ContextPacketSHA256  string                   `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath      string                   `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256    string                   `json:"owner_prompt_sha256,omitempty"`
	IssueSnapshotSource  string                   `json:"issue_snapshot_source,omitempty"`
	NextCommand          string                   `json:"next_command,omitempty"`
}

func (result Result) Clone() Result {
	cloned := result
	cloned.Execution = cloneExecutionPointer(result.Execution)
	return cloned
}

type RootClaim struct {
	LifecycleID string
	Branch      string
	Root        string
}

type Snapshot struct {
	Record         leasecontract.Record
	RecordRaw      []byte
	CanonicalRoot  string
	ClaimTokenPath string
	RootConflict   *RootClaim
}

type AccessResult struct {
	Allowed         bool   `json:"allowed"`
	Code            string `json:"code,omitempty"`
	RelaunchCommand string `json:"relaunch_command,omitempty"`
}

type ProbeResult struct {
	Available bool   `json:"available"`
	Ready     bool   `json:"ready"`
	Code      string `json:"code,omitempty"`
}

type OwnerEvidence struct {
	IssueURL   string
	IssueBody  string
	BodySHA256 string
	Source     string
	Provider   string
	Issue      int
}

type OwnerArtifacts struct {
	PlanPath            string
	ClaimTokenPath      string
	ClaimTokenSHA256    string
	ContextPacketPath   string
	ContextPacketSHA256 string
	OwnerPromptPath     string
	OwnerPromptSHA256   string
}

func (snapshot Snapshot) Clone() Snapshot {
	cloned := snapshot
	cloned.Record = cloneRecord(snapshot.Record)
	cloned.RecordRaw = cloneBytes(snapshot.RecordRaw)
	if snapshot.RootConflict != nil {
		claim := *snapshot.RootConflict
		cloned.RootConflict = &claim
	}
	return cloned
}

func cloneRecord(record leasecontract.Record) leasecontract.Record {
	cloned := record
	cloned.Intent = cloneRaw(record.Intent)
	cloned.DesignReview = cloneRaw(record.DesignReview)
	cloned.DomainReview = cloneRaw(record.DomainReview)
	cloned.IssueLinks = cloneRaw(record.IssueLinks)
	cloned.BranchPrepare = cloneRaw(record.BranchPrepare)
	cloned.RemoteArtifact = cloneRaw(record.RemoteArtifact)
	cloned.Decisions = cloneRaw(record.Decisions)
	cloned.PlanPrep = cloneRaw(record.PlanPrep)
	cloned.CompatibilityReview = cloneRaw(record.CompatibilityReview)
	cloned.DevilsAdvocateReview = cloneRaw(record.DevilsAdvocateReview)
	cloned.Feedback = cloneRaw(record.Feedback)
	cloned.RegressEvents = cloneRaw(record.RegressEvents)
	cloned.Delegation = cloneRaw(record.Delegation)
	cloned.ChildCycles = cloneRaw(record.ChildCycles)
	cloned.RemoteCompletion = cloneRaw(record.RemoteCompletion)
	cloned.CleanupFinishFailure = cloneRaw(record.CleanupFinishFailure)
	cloned.ImplementationReview = cloneRaw(record.ImplementationReview)
	cloned.RoutingTrace = cloneRaw(record.RoutingTrace)
	cloned.AISlopCleanCategories = cloneRaw(record.AISlopCleanCategories)
	cloned.AISlopCleanVerification = cloneRaw(record.AISlopCleanVerification)
	cloned.PhaseLedger = cloneRaw(record.PhaseLedger)
	cloned.Execution = cloneExecutionPointer(record.Execution)
	return cloned
}

func cloneExecutionPointer(execution *leasecontract.Execution) *leasecontract.Execution {
	if execution == nil {
		return nil
	}
	cloned := *execution
	if execution.Selection != nil {
		selection := *execution.Selection
		cloned.Selection = &selection
	}
	cloned.Lease.Holder = cloneActorPointer(execution.Lease.Holder)
	if execution.Orca != nil {
		binding := *execution.Orca
		cloned.Orca = &binding
	}
	if execution.Pending != nil {
		pending := *execution.Pending
		cloned.Pending = &pending
	}
	if execution.Completion != nil {
		completion := *execution.Completion
		completion.Verification = cloneStrings(execution.Completion.Verification)
		cloned.Completion = &completion
	}
	if execution.CompletionHistory != nil {
		cloned.CompletionHistory = append([]leasecontract.CompletionHistoryEntry(nil), execution.CompletionHistory...)
		for index := range cloned.CompletionHistory {
			cloned.CompletionHistory[index].Completion.Verification = cloneStrings(execution.CompletionHistory[index].Completion.Verification)
		}
	}
	if execution.Failure != nil {
		failure := *execution.Failure
		cloned.Failure = &failure
	}
	if execution.SyncBaseResolution != nil {
		resolution := *execution.SyncBaseResolution
		resolution.Actor = cloneActor(execution.SyncBaseResolution.Actor)
		resolution.ConflictFiles = cloneStrings(execution.SyncBaseResolution.ConflictFiles)
		cloned.SyncBaseResolution = &resolution
	}
	if execution.SyncBaseEvents != nil {
		cloned.SyncBaseEvents = append([]leasecontract.SyncBaseEvent{}, execution.SyncBaseEvents...)
	}
	return &cloned
}

func cloneActor(actor leasecontract.Actor) leasecontract.Actor {
	cloned := actor
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		cloned.SessionProcess = &process
	}
	return cloned
}

func cloneActorPointer(actor *leasecontract.Actor) *leasecontract.Actor {
	if actor == nil {
		return nil
	}
	cloned := cloneActor(*actor)
	return &cloned
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage{}, value...)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
