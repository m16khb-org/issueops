// 실행 액션의 요청과 핸들러 계약이다. port는 이제 계약 어휘로 말하므로 이
// 선언들은 인터페이스를 물지 않는 한 계약 계층이 소유한다.

package issueops

import (
	"context"

	executionissue "issueops/internal/contract/executionissue"
)

// RemotePullRequestCreateHandler 타입은 원격 PR/MR 생성 요청을 provider에 전달하는
// 핸들러 시그니처다. port 인터페이스를 필드나 시그니처로 갖는 선언은 계약이 아니라
// 어댑터가 소유한다. contract 계층은 port를 참조할 수 없다.
type RemotePullRequestCreateHandler func(context.Context, string, RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error)

// ExecutionActionRequest is the single transport request for execution actions
// (prepare, claim, release, replace, reseed, complete and so on). Action selects
// the operation; each action reads only the fields it needs.
type ExecutionActionRequest struct {
	Action                 string      `json:"action"`
	ID                     string      `json:"id"`
	Actor                  NativeActor `json:"actor"`
	CWD                    string      `json:"cwd,omitempty"`
	Generation             uint64      `json:"generation,omitempty"`
	ExpectedGeneration     uint64      `json:"expected_generation,omitempty"`
	CompletionGeneration   uint64      `json:"completion_generation,omitempty"`
	TokenFile              string      `json:"claim_token_file,omitempty"`
	ClaimCurrentToken      bool        `json:"claim_current_token,omitempty"`
	ReplaceAction          string      `json:"replace_action,omitempty"`
	InventoryFingerprint   string      `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint  string      `json:"quiescence_fingerprint,omitempty"`
	Reason                 string      `json:"reason,omitempty"`
	Preview                bool        `json:"preview,omitempty"`
	Confirm                bool        `json:"confirm,omitempty"`
	FinalHead              string      `json:"final_head,omitempty"`
	VerificationReportPath string      `json:"verification_report_path,omitempty"`
	Verification           []string    `json:"verification,omitempty"`
	RemoteArtifactURL      string      `json:"remote_artifact_url,omitempty"`
}

// RemotePublicationHandlers groups the handlers used to publish a PR/MR:
// Create opens it and Reconcile settles an interrupted create.
type RemotePublicationHandlers struct {
	Create    RemotePullRequestCreateHandler
	Reconcile RemotePullRequestReconcileHandler
}

// ExecutionReseedRequest asks to reseed an execution workspace from the current
// lease at ExpectedGeneration. Confirm applies it; without Confirm it previews
// the InventoryFingerprint that approval must echo.
type ExecutionReseedRequest struct {
	ID                   string      `json:"id"`
	ExpectedGeneration   uint64      `json:"expected_generation"`
	CompletionGeneration uint64      `json:"completion_generation,omitempty"`
	InventoryFingerprint string      `json:"inventory_fingerprint,omitempty"`
	Reason               string      `json:"reason,omitempty"`
	Actor                NativeActor `json:"actor"`
	CWD                  string      `json:"cwd"`
	Confirm              bool        `json:"confirm,omitempty"`
}

// ExecutionPrepareHandler runs execution prepare for the cycle with the given ID.
type ExecutionPrepareHandler func(context.Context, string, ExecutionPrepareRequest) (ExecutionPrepareResult, error)

// ExecutionClaimHandler runs execution claim for the cycle with the given ID.
type ExecutionClaimHandler func(context.Context, string, ExecutionClaimRequest) (ExecutionResult, error)

// ExecutionReseedHandler runs execution reseed for the cycle with the given ID.
type ExecutionReseedHandler func(context.Context, string, ExecutionReseedRequest) (ExecutionReplaceResult, error)
