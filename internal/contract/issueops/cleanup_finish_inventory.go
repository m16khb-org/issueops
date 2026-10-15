package issueops

// CleanupFinishInventory binds a preview to the observed local cleanup targets.
type CleanupFinishInventory struct {
	ID              string `json:"id"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	WorktreeRoot    string `json:"worktree_root"`
	WorktreePresent bool   `json:"worktree_present"`
	BranchOID       string `json:"branch_oid"`
	RemoteURL       string `json:"remote_url"`
	// SupersededBy는 replacement 증거를 fingerprint 입력에 포함시킨다. 증거가
	// 바뀌면 preview는 무효가 되어야 한다.
	SupersededBy string `json:"superseded_by,omitempty"`
	// WorkspaceProcesses는 apply ①′가 종료할 집합이다. preview 뒤
	// 집합이 바뀌면 fingerprint가 달라져 apply가 멈춘다(#477).
	WorkspaceProcesses []NativeProcessReceipt `json:"workspace_processes,omitempty"`
}
