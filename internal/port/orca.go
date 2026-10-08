package port

import (
	"context"
	"fmt"
	"strings"

	deliverycontract "issueops/internal/contract/issueops"
)

const OrcaMaxBaselineIDs = 512

type OrcaError struct {
	Code    string `json:"code"`
	Detail  string `json:"detail,omitempty"`
	Invoked bool   `json:"invoked,omitempty"`
	Timeout bool   `json:"timeout,omitempty"`
	// OrchestrationRequestID is the response value observed for diagnostics.
	// Callers must validate it against sealed state before using it as retry identity.
	OrchestrationRequestID string `json:"orchestration_request_id,omitempty"`
	DispatchRequestID      string `json:"dispatch_request_id,omitempty"`
	CallPhase              string `json:"call_phase,omitempty"`
}

func (e *OrcaError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

type OrcaProbeRequest struct {
	Repo     string `json:"repo"`
	Agent    string `json:"agent"`
	Provider string `json:"provider,omitempty"`
}

type OrcaProbeResult struct {
	Available        bool   `json:"available"`
	Ready            bool   `json:"ready"`
	Code             string `json:"code,omitempty"`
	Detail           string `json:"detail,omitempty"`
	RuntimeID        string `json:"runtime_id,omitempty"`
	RepoID           string `json:"repo_id,omitempty"`
	RepoPath         string `json:"repo_path,omitempty"`
	RepoRemoteName   string `json:"repo_remote_name,omitempty"`
	WorktreeBasePath string `json:"worktree_base_path,omitempty"`
	Agent            string `json:"agent,omitempty"`
	Provider         string `json:"provider,omitempty"`
}

type OrcaStatus struct {
	RuntimeID string `json:"runtime_id,omitempty"`
	Version   string `json:"version,omitempty"`
	// TargetIdentity is read directly from target.serverId, target.id, or
	// target.kind, in that order. Local runtimes therefore use "local" without
	// presenting it as a server identifier.
	TargetIdentity   string `json:"target_identity,omitempty"`
	RuntimeReachable bool   `json:"runtime_reachable"`
	RuntimeState     string `json:"runtime_state,omitempty"`
	GraphState       string `json:"graph_state,omitempty"`
	// AppPID는 Orca 데스크톱 앱 프로세스다. cleanup은 이 pid를 종료 대상에서
	// 항상 제외한다(#477).
	AppPID int `json:"app_pid,omitempty"`
}

// CleanupOrcaTerminals는 cleanup finish/abandon이 워크트리에 매인 Orca 터미널을
// 관측하고 닫는 좁은 표면이다. 기존 OwnerInspector fake를 건드리지 않도록 별도
// 인터페이스로 둔다(#477).
type CleanupOrcaTerminals interface {
	Status(ctx context.Context) (OrcaStatus, error)
	// ListAllTerminals는 요청자 터미널을 ORCA_PANE_KEY/ORCA_TERMINAL_HANDLE과
	// join해 확정하기 위한 전체 인벤토리다.
	ListAllTerminals(ctx context.Context) ([]OrcaTerminal, error)
	// ListWorktreeTerminalsByPath는 등록되지 않은 워크트리(selector_not_found)를
	// 빈 목록으로 돌려주고, 그 밖의 오류만 오류로 돌려준다.
	ListWorktreeTerminalsByPath(ctx context.Context, path string) ([]OrcaTerminal, error)
	// CloseTerminal은 preview fingerprint가 승인한 exact handle 하나를 닫고,
	// PTY 종료까지 확인되지 않으면 오류를 돌려준다.
	CloseTerminal(ctx context.Context, handle string) error
}

type OrcaRepo struct {
	RuntimeID        string `json:"-"`
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name,omitempty"`
	RemoteName       string `json:"remote_name,omitempty"`
	WorktreeBasePath string `json:"worktree_base_path,omitempty"`
}

type OrcaRun struct {
	RuntimeID string `json:"-"`
	ID        string `json:"id"`
	Objective string `json:"objective"`
}

type OrcaCreateRunRequest struct {
	Objective string `json:"objective"`
}

type OrcaWorktree struct {
	RuntimeID         string `json:"runtime_id,omitempty"`
	ID                string `json:"id"`
	InstanceID        string `json:"instance_id,omitempty"`
	RepoID            string `json:"repo_id,omitempty"`
	Path              string `json:"path"`
	Head              string `json:"head,omitempty"`
	Branch            string `json:"branch,omitempty"`
	Name              string `json:"name,omitempty"`
	Comment           string `json:"comment,omitempty"`
	BaseRef           string `json:"base_ref,omitempty"`
	Issue             int    `json:"issue,omitempty"`
	GitLabIssue       *int   `json:"gitlab_issue,omitempty"`
	ParentWorktreeID  string `json:"parent_worktree_id,omitempty"`
	LineageSource     string `json:"lineage_source,omitempty"`
	LineageConfidence string `json:"lineage_confidence,omitempty"`
}

type OrcaCreateWorktreeRequest struct {
	Repo           string `json:"repo"`
	Name           string `json:"name"`
	BaseBranch     string `json:"base_branch"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	UpstreamBranch string `json:"upstream_branch,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Issue          int    `json:"issue,omitempty"`
	Comment        string `json:"comment"`
}

type OrcaTerminal struct {
	RuntimeID      string `json:"runtime_id,omitempty"`
	Handle         string `json:"handle"`
	PTYID          string `json:"pty_id"`
	WorktreeID     string `json:"worktree_id"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	TabID          string `json:"tab_id,omitempty"`
	LeafID         string `json:"leaf_id,omitempty"`
	StableTabTitle string `json:"stable_tab_title,omitempty"`
	Title          string `json:"title,omitempty"`
	Connected      bool   `json:"connected"`
	Writable       bool   `json:"writable"`
}

type OrcaCreateTerminalRequest struct {
	WorktreeID                string `json:"worktree_id"`
	Agent                     string `json:"agent"`
	Model                     string `json:"model,omitempty"`
	ReasoningEffort           string `json:"reasoning_effort,omitempty"`
	Title                     string `json:"title,omitempty"`
	AllowCodexHookTrustBypass bool   `json:"allow_codex_hook_trust_bypass,omitempty"`
	// ExtraArgs are role-agent launch arguments appended to the owner command.
	ExtraArgs []string `json:"extra_args,omitempty"`
}

type OrcaTask struct {
	RuntimeID   string `json:"-"`
	RunID       string `json:"run_id,omitempty"`
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	HasResult   bool   `json:"has_result,omitempty"`
}

type OrcaGate struct {
	RuntimeID string `json:"-"`
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
}

type OrcaInboxPresence struct {
	RuntimeID       string `json:"-"`
	Count           int    `json:"count"`
	RowCount        int    `json:"row_count"`
	CompleteAbsence bool   `json:"complete_absence"`
}

type OrcaCreateTaskRequest struct {
	RunID       string `json:"run_id"`
	Spec        string `json:"spec"`
	Title       string `json:"title"`
	DisplayName string `json:"display_name"`
}

type OrcaDispatchRequest struct {
	RunID          string `json:"run_id"`
	TaskID         string `json:"task_id"`
	ToHandle       string `json:"to_handle"`
	Inject         bool   `json:"inject"`
	ReturnPreamble bool   `json:"return_preamble"`
	RetryRequestID string `json:"retry_request_id,omitempty"`
}

type OrcaDispatch struct {
	RuntimeID      string `json:"-"`
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	AssigneeHandle string `json:"assignee_handle,omitempty"`
	Status         string `json:"status,omitempty"`
	Injected       bool   `json:"injected,omitempty"`
	Preamble       string `json:"preamble,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
}

type OrcaRequestObservation struct {
	RuntimeID string `json:"-"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Method    string `json:"method,omitempty"`
}

type OrcaPromptReceipt struct {
	RequestID               string   `json:"request_id"`
	Stages                  []string `json:"stages,omitempty"`
	Provider                string   `json:"provider,omitempty"`
	Observation             string   `json:"observation,omitempty"`
	ProcessIncarnation      string   `json:"process_incarnation,omitempty"`
	Generation              uint64   `json:"generation,omitempty"`
	BaselineWorkingSequence *uint64  `json:"baseline_working_sequence,omitempty"`
}

type OrcaDeliveryReceiptExpectation struct {
	Host                     string
	TaskID                   string
	TerminalPTYID            string
	TerminalHandle           string
	DispatchRequestID        string
	PromptRequestID          string
	PromptProcessIncarnation string
}

func ValidateOrcaDurableRequestID(actual, retry string) error {
	actual = strings.TrimSpace(actual)
	retry = strings.TrimSpace(retry)
	if err := deliverycontract.ValidateOrcaRequestID(actual); err != nil {
		return fmt.Errorf("Orca response is missing a durable request UUID")
	}
	if retry != "" {
		if err := deliverycontract.ValidateOrcaRequestID(retry); err != nil {
			return fmt.Errorf("Orca retry request UUID is invalid")
		}
	}
	if retry != "" && actual != retry {
		return fmt.Errorf("Orca response request UUID does not match the requested retry UUID")
	}
	return nil
}

func ValidateOrcaPromptReceipt(receipt OrcaPromptReceipt, retryID, expectedProcess string) error {
	if err := ValidateOrcaDurableRequestID(receipt.RequestID, retryID); err != nil {
		return err
	}
	if strings.TrimSpace(receipt.Provider) != "omo" || strings.TrimSpace(receipt.ProcessIncarnation) == "" || receipt.Generation == 0 || receipt.BaselineWorkingSequence == nil {
		return fmt.Errorf("Orca Omo prompt receipt is incomplete")
	}
	accepted := false
	for _, stage := range receipt.Stages {
		if strings.TrimSpace(stage) == "input_accepted" {
			accepted = true
			break
		}
	}
	if !accepted {
		return fmt.Errorf("Orca Omo prompt receipt has no input_accepted stage")
	}
	if expectedProcess = strings.TrimSpace(expectedProcess); expectedProcess != "" && strings.TrimSpace(receipt.ProcessIncarnation) != expectedProcess {
		return fmt.Errorf("Orca Omo prompt receipt belongs to a different process incarnation")
	}
	return nil
}

func ValidateExecutionOrcaDeliveryReceipt(receipt ExecutionOrcaIntentReceipt, expected OrcaDeliveryReceiptExpectation) error {
	if strings.TrimSpace(receipt.TaskID) == "" || strings.TrimSpace(receipt.TaskID) != strings.TrimSpace(expected.TaskID) ||
		strings.TrimSpace(receipt.DispatchID) == "" || strings.TrimSpace(receipt.TerminalPTYID) == "" || strings.TrimSpace(receipt.TerminalHandle) == "" {
		return fmt.Errorf("Orca dispatch receipt is incomplete")
	}
	if terminalPTYID := strings.TrimSpace(expected.TerminalPTYID); terminalPTYID != "" && strings.TrimSpace(receipt.TerminalPTYID) != terminalPTYID {
		return fmt.Errorf("Orca dispatch receipt belongs to a different terminal PTY")
	}
	if terminalHandle := strings.TrimSpace(expected.TerminalHandle); terminalHandle != "" && strings.TrimSpace(receipt.TerminalHandle) != terminalHandle {
		return fmt.Errorf("Orca dispatch receipt belongs to a different current terminal handle")
	}
	if err := ValidateOrcaDurableRequestID(receipt.RequestID, expected.DispatchRequestID); err != nil {
		return err
	}
	switch strings.TrimSpace(expected.Host) {
	case "codex", "claude":
		if receipt.PromptReceipt != nil {
			return fmt.Errorf("Orca injected dispatch unexpectedly carried a prompt receipt")
		}
	case "omo":
		if receipt.PromptReceipt == nil {
			return fmt.Errorf("Orca Omo delivery is missing its prompt receipt")
		}
		if err := ValidateOrcaPromptReceipt(*receipt.PromptReceipt, expected.PromptRequestID, expected.PromptProcessIncarnation); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Orca delivery receipt host is invalid")
	}
	return nil
}
