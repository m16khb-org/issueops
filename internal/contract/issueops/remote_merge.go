package issueops

// Merge methods accepted by `issueops remote merge-pr`. Squash is the default.
const (
	RemoteMergeMethodSquash = "squash"
	RemoteMergeMethodMerge  = "merge"
	RemoteMergeMethodRebase = "rebase"
)

// Aggregate CI check states of a pull or merge request head.
const (
	RemoteChecksPassing = "passing"
	RemoteChecksFailing = "failing"
	RemoteChecksPending = "pending"
	RemoteChecksNone    = "none"
)

// Merge blocker codes. Each one stops `merge-pr --confirm` before any provider
// mutation; the issueops-merge skill maps every code to its recovery path.
const (
	RemoteMergeBlockerNotOpen       = "pr_not_open"
	RemoteMergeBlockerHeadMismatch  = "head_mismatch"
	RemoteMergeBlockerChecksFailing = "checks_failing"
	RemoteMergeBlockerChecksPending = "checks_pending"
	RemoteMergeBlockerConflict      = "merge_conflict"
	RemoteMergeBlocked              = "merge_blocked"
)

// RemotePullRequestMergeState is one provider readback of a PR/MR taken before
// a merge decision. Head, checks and mergeability come from the same read.
type RemotePullRequestMergeState struct {
	URL   string
	State string // open | merged | closed
	Draft bool
	// HeadOID is the source branch tip the provider would merge.
	HeadOID    string
	BaseBranch string
	Checks     string
	Failing    []string
	Pending    []string
	Conflict   bool
	// Blocked carries the provider's own reason when it refuses the merge for
	// something other than checks or conflicts (required review, protection).
	Blocked        string
	MergeCommitOID string
}

type RemoteMergeBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RemoteMergeResult is the CLI/MCP-neutral output of `issueops remote merge-pr`.
type RemoteMergeResult struct {
	OK              bool                 `json:"ok"`
	ID              string               `json:"id"`
	Provider        string               `json:"provider"`
	URL             string               `json:"url"`
	Method          string               `json:"method"`
	State           string               `json:"state"`
	Draft           bool                 `json:"draft"`
	HeadOID         string               `json:"head_oid"`
	ExpectedHeadOID string               `json:"expected_head_oid"`
	BaseBranch      string               `json:"base_branch"`
	Checks          string               `json:"checks"`
	FailingChecks   []string             `json:"failing_checks,omitempty"`
	PendingChecks   []string             `json:"pending_checks,omitempty"`
	Blockers        []RemoteMergeBlocker `json:"blockers"`
	Merged          bool                 `json:"merged"`
	AlreadyMerged   bool                 `json:"already_merged,omitempty"`
	MarkedReady     bool                 `json:"marked_ready,omitempty"`
	MergeCommitOID  string               `json:"merge_commit_oid,omitempty"`
	NextCommand     string               `json:"next_command,omitempty"`
}
