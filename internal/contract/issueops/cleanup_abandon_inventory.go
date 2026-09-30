package issueops

// CleanupAbandonInventory는 fingerprint 입력이 되는 현재 관측 상태다.
// cleanupFinishInventory와 달리 원격 completion 권한을 다루지 않고, 레코드가
// 지목한 로컬 worktree와 branch의 삭제 권한만 봉인한다.
type CleanupAbandonInventory struct {
	ID                 string `json:"id"`
	Repo               string `json:"repo"`
	Branch             string `json:"branch"`
	WorktreeRoot       string `json:"worktree_root"`
	WorktreePresent    bool   `json:"worktree_present"`
	WorktreeCanonical  bool   `json:"worktree_canonical"`
	WorktreeBranch     string `json:"worktree_branch"`
	WorktreeClean      bool   `json:"worktree_clean"`
	WorktreeHead       string `json:"worktree_head"`
	BranchOID          string `json:"branch_oid"`
	BranchCheckoutPath string `json:"branch_checkout_path"`
	RecordSHA          string `json:"record_sha"`
	Phase              string `json:"phase"`
	LeaseStatus        string `json:"lease_status"`
	PendingOperationID string `json:"pending_operation_id"`
	// WorkspaceProcesses와 OrcaTerminals는 apply ①′가 종료할 집합이며 fingerprint
	// 입력이다(#477).
	WorkspaceProcesses []NativeProcessReceipt `json:"workspace_processes,omitempty"`
	OrcaTerminals      []string               `json:"orca_terminals,omitempty"`
	// OrcaAppPID는 ①′가 시그널에서 제외할 Orca 앱 pid다(finish와 같은 계약).
	OrcaAppPID       int  `json:"orca_app_pid,omitempty"`
	OrcaRuntimeReady bool `json:"orca_runtime_ready,omitempty"`
	// 아래 넷은 원격 효과가 요청됐을 때만 채워진다. 전부 omitempty이므로
	// 플래그 없는 폐기의 fingerprint는 이 확장 전과 같은 값이다.
	//
	// 관측한 원격 *상태*는 여기 넣지 않는다(ArtifactUnmerged와 같은 규율).
	// RemoteBranchOID만 예외인데, 삭제가 force-with-lease CAS로 그 값을
	// 사용하므로 승인 대상의 일부이기 때문이다.
	ClosePR            bool   `json:"close_pr,omitempty"`
	CloseIssue         bool   `json:"close_issue,omitempty"`
	DeleteRemoteBranch bool   `json:"delete_remote_branch,omitempty"`
	RemoteBranchOID    string `json:"remote_branch_oid,omitempty"`
}
