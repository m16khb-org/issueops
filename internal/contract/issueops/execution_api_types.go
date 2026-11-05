// issueops 실행 API의 요청·결과·핸들러 계약이다. 값을 만들어내는 쪽은 I/O를
// 하지만 읽고 전달하는 쪽은 그 구현을 알 필요가 없다.

package issueops

import (
	"context"
)

// ExecutionReleaseHandler runs execution release for the cycle with the given ID.
type ExecutionReleaseHandler func(context.Context, string, ExecutionReleaseRequest) (ExecutionResult, error)

// ExecutionCompleteHandler runs execution complete for the cycle with the given ID.
type ExecutionCompleteHandler func(context.Context, string, ExecutionCompleteRequest) (ExecutionResult, error)

// RemotePullRequestReconcileHandler settles an interrupted PR/MR create for the
// cycle with the given ID.
type RemotePullRequestReconcileHandler func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error)

// ExecutionCompleteRequest is the transport DTO consumed by the current
// completion inbound adapter.
type ExecutionCompleteRequest struct {
	ID                     string      `json:"id"`
	Generation             uint64      `json:"generation"`
	Actor                  NativeActor `json:"actor"`
	CWD                    string      `json:"cwd"`
	FinalHead              string      `json:"final_head"`
	VerificationReportPath string      `json:"verification_report_path"`
	Verification           []string    `json:"verification"`
	RemoteArtifactURL      string      `json:"remote_artifact_url"`
	Confirm                bool        `json:"confirm"`
}

// ExecutionResult is the common answer of an execution action: the resulting
// Execution state and the command to run next.
type ExecutionResult struct {
	OK          bool      `json:"ok"`
	ID          string    `json:"id"`
	Execution   Execution `json:"execution"`
	NextCommand string    `json:"next_command,omitempty"`
}

// ExecutionClaimRequest asks the actor to claim the write lease at Generation,
// authenticating with the claim token file or the current token.
type ExecutionClaimRequest struct {
	ID                string      `json:"id"`
	Generation        uint64      `json:"generation"`
	Actor             NativeActor `json:"actor"`
	CWD               string      `json:"cwd"`
	TokenFile         string      `json:"claim_token_file,omitempty"`
	ClaimCurrentToken bool        `json:"claim_current_token,omitempty"`
}

// ExecutionReleaseRequest asks the lease holder at Generation to release it.
type ExecutionReleaseRequest struct {
	ID         string      `json:"id"`
	Generation uint64      `json:"generation"`
	Actor      NativeActor `json:"actor"`
	CWD        string      `json:"cwd"`
}

// ExecutionReplaceResult is the answer of a lease replace: the resulting
// Execution, the fingerprints the approval was bound to, and where the new claim
// token was written.
type ExecutionReplaceResult struct {
	OK                    bool      `json:"ok"`
	ID                    string    `json:"id"`
	Action                string    `json:"action"`
	Execution             Execution `json:"execution"`
	InventoryFingerprint  string    `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string    `json:"quiescence_fingerprint,omitempty"`
	ClaimTokenPath        string    `json:"claim_token_path,omitempty"`
	NextCommand           string    `json:"next_command,omitempty"`
}

// ExecutionPrepareRequest asks to create (or, without Confirm, preview) the
// isolated execution workspace for the cycle.
type ExecutionPrepareRequest struct {
	ID      string      `json:"id"`
	Actor   NativeActor `json:"actor"`
	CWD     string      `json:"cwd"`
	Confirm bool        `json:"confirm,omitempty"`
}

// ExecutionPrepareResult is the answer of execution prepare: the Workspace and,
// once applied, the Execution and the path of the written claim token.
type ExecutionPrepareResult struct {
	OK             bool       `json:"ok"`
	ID             string     `json:"id"`
	Preview        bool       `json:"preview,omitempty"`
	Workspace      Workspace  `json:"workspace"`
	Execution      *Execution `json:"execution,omitempty"`
	ClaimTokenPath string     `json:"claim_token_path,omitempty"`
	NextCommand    string     `json:"next_command,omitempty"`
}

// ExecutionReconcileRequest asks to settle a pending ExternalIntent. Snapshot is
// the in-process record the caller already loaded and is never serialized.
type ExecutionReconcileRequest struct {
	ID       string      `json:"id"`
	Preview  bool        `json:"preview,omitempty"`
	Confirm  bool        `json:"confirm,omitempty"`
	Actor    NativeActor `json:"actor"`
	CWD      string      `json:"cwd"`
	Snapshot *Record     `json:"-"`
}

// ExecutionReconcileResult is the answer of reconciling a pending intent: the
// outcome Code, the Execution afterwards, and whether external state was read.
type ExecutionReconcileResult struct {
	OK         bool            `json:"ok"`
	ID         string          `json:"id"`
	Preview    bool            `json:"preview,omitempty"`
	Reconciled bool            `json:"reconciled"`
	Code       string          `json:"code"`
	Execution  Execution       `json:"execution"`
	Pending    *ExternalIntent `json:"pending,omitempty"`
	// ExternalStateInspected는 이 결과가 외부 자원을 실제로 조회하고 나온
	// 것인지 밝힌다. preview는 pending kind만 보고 상수 코드를 돌려주므로
	// false다. 그 구분이 없으면 preview 출력이 외부 상태를 관측한 증거로
	// 오독된다(이슈 #154).
	//
	// omitempty를 쓰지 않는다. "조회하지 않았다"가 이 필드의 핵심 정보이므로
	// false가 출력에서 사라지면 목적 자체가 무너진다.
	ExternalStateInspected bool `json:"external_state_inspected"`
	// NextCommand is the --confirm step a preview with a pending intent hands
	// back.
	NextCommand string `json:"next_command,omitempty"`
}

// RemotePullRequestRequest carries everything needed to create the cycle's PR/MR
// (title, body, head, base, labels, assignees) bound to ExpectedGeneration.
type RemotePullRequestRequest struct {
	ID                 string      `json:"id"`
	Provider           string      `json:"provider"`
	Title              string      `json:"title"`
	Body               string      `json:"body"`
	Head               string      `json:"head"`
	Base               string      `json:"base"`
	Labels             []string    `json:"labels"`
	Assignees          []string    `json:"assignees"`
	ExpectedGeneration uint64      `json:"expected_generation"`
	Actor              NativeActor `json:"actor"`
	CWD                string      `json:"cwd"`
	Confirm            bool        `json:"confirm"`
}

// ExecutionSyncBaseRequest asks to sync the cycle's branch with its base.
// Mode selects apply, finalize or abort; Fingerprint binds a confirm to the
// preview it approves.
type ExecutionSyncBaseRequest struct {
	ID                   string      `json:"id"`
	Mode                 string      `json:"mode"`
	CompletionGeneration uint64      `json:"completion_generation,omitempty"`
	Actor                NativeActor `json:"actor"`
	CWD                  string      `json:"cwd"`
	Confirm              bool        `json:"confirm,omitempty"`
	Fingerprint          string      `json:"fingerprint,omitempty"`
}

// ExecutionSyncBaseDeps 타입은 Git 표면 하나만 주입점으로 연다. fetch·merge-tree·
// merge·commit·push가 전부 이 함수를 지나므로 테스트가 순서와 인자를 전수
// 검증할 수 있고, 비대화 env 계약과 timeout도 한 곳에서 강제된다.
type ExecutionSyncBaseDeps struct {
	Git func(ctx context.Context, dir string, args ...string) (int, string)
}

// ExecutionSyncBaseResult is the answer of a sync-base call: the observed branch
// and base state, what was merged or pushed, and the command to retry or abort.
type ExecutionSyncBaseResult struct {
	OK                  bool     `json:"ok"`
	ID                  string   `json:"id"`
	Mode                string   `json:"mode"`
	LeaseGeneration     uint64   `json:"lease_generation,omitempty"`
	Missing             []string `json:"missing,omitempty"`
	Fingerprint         string   `json:"fingerprint,omitempty"`
	Branch              string   `json:"branch,omitempty"`
	BaseBranch          string   `json:"base_branch,omitempty"`
	BaseOID             string   `json:"base_oid,omitempty"`
	WorkOID             string   `json:"work_oid,omitempty"`
	RemoteBranchPresent bool     `json:"remote_branch_present"`
	MergeInProgress     bool     `json:"merge_in_progress"`
	MergeNeeded         bool     `json:"merge_needed"`
	ConflictFiles       []string `json:"conflict_files,omitempty"`
	UntrackedWarnings   []string `json:"untracked_warnings,omitempty"`
	Merged              bool     `json:"merged,omitempty"`
	MergeCommit         string   `json:"merge_commit,omitempty"`
	Pushed              bool     `json:"pushed,omitempty"`
	PushRetryRequired   bool     `json:"push_retry_required,omitempty"`
	Aborted             bool     `json:"aborted,omitempty"`
	FailedStep          string   `json:"failed_step,omitempty"`
	NextCommand         string   `json:"next_command,omitempty"`
	AbortCommand        string   `json:"abort_command,omitempty"`
}
