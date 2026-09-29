package issueopsmodeswitch

type Inventory struct {
	ID              string `json:"id"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	CurrentMode     string `json:"current_mode"`
	RequestedMode   string `json:"requested_mode"`
	WorktreeRoot    string `json:"worktree_root"`
	WorktreePresent bool   `json:"worktree_present"`
	BranchOID       string `json:"branch_oid"`
	LeaseStatus     string `json:"lease_status"`
	LeaseGeneration uint64 `json:"lease_generation"`
	PendingID       string `json:"pending_operation_id"`
}
