// Package operationalhealth is the contract for local operational-health
// inspection: the snapshot of cycles, lease holders, worktrees, refs and state
// artifacts that adapters collect, and the findings and authority verdicts the
// domain classifier derives from it.
package operationalhealth

import "time"

// Finding codes reported in Finding.Code, one per class of operational
// problem or unknown inventory.
const (
	FindingInventoryUnknown     = "operational_inventory_unknown"
	FindingDeadOwner            = "operational_dead_owner"
	FindingWorktreeResidue      = "operational_worktree_residue"
	FindingNonMainBranchResidue = "operational_non_main_branch_residue"
	FindingStateArtifactResidue = "operational_state_artifact_residue"
	FindingExecutionFailure     = "operational_execution_failure"
	FindingIssueCreateFailure   = "operational_issue_create_failure"
	FindingCleanupFailure       = "operational_cleanup_failure"
)

// Process statuses a cycle's lease holder may report in
// Cycle.HolderProcessStatus after probing the recorded PID and identity.
const (
	ProcessStatusLive             = "live"
	ProcessStatusDead             = "dead"
	ProcessStatusIdentityMismatch = "identity-mismatch"
	ProcessStatusUnknown          = "unknown"
)

// Cycle is the durable view of one IssueOps lifecycle record as the health
// check sees it: branch, phase, lease, holder process identity and which
// failure or completion records exist.
type Cycle struct {
	ID                        string
	Repo                      string
	Branch                    string
	Phase                     string
	LeaseStatus               string
	Generation                uint64
	HolderHost                string
	HolderSessionID           string
	HolderAgentID             string
	HolderPID                 int
	HolderStartedAt           string
	HolderExecutable          string
	HolderProcessStatus       string
	CompletionPresent         bool
	ExecutionFailurePresent   bool
	CleanupFailurePresent     bool
	IssueCreateFailurePresent bool
	WorktreePath              string
}

// LeaseHolderIndex is one row of the lease-holder index, mapping a holder to
// the lifecycle and generation it holds.
type LeaseHolderIndex struct {
	Key         string
	LifecycleID string
	Generation  uint64
	Host        string
	SessionID   string
	AgentID     string
}

// GitWorktree is one worktree reported by git, with its HEAD, whether it is
// clean and whether it is the canonical cycle worktree.
type GitWorktree struct {
	Path      string
	Branch    string
	Head      string
	Clean     bool
	Canonical bool
}

// GitRef is one local or remote branch ref with its object ID and the
// location (local or remote name) it was read from.
type GitRef struct {
	Name     string
	Branch   string
	OID      string
	Location string
}

// InventoryProblem records a source that could not be read completely, so the
// affected inventory is treated as unknown rather than healthy.
type InventoryProblem struct {
	Source string
	Code   string
	Detail string
}

// StateArtifact is a leftover file in the state directory and the code that
// classifies why it is residue.
type StateArtifact struct {
	Path string
	Code string
}

// Snapshot is the complete operational inventory the classifier judges: the
// repository state plus cycles, lease-holder indexes, worktrees, refs, state
// artifacts and any problems met while collecting them.
type Snapshot struct {
	RepoRoot        string
	CanonicalBranch string
	SourceHead      string
	SourceClean     bool

	Cycles             []Cycle
	LeaseHolderIndexes []LeaseHolderIndex
	GitWorktrees       []GitWorktree
	LocalRefs          []GitRef
	RemoteRefs         []GitRef
	StateArtifacts     []StateArtifact
	InventoryProblems  []InventoryProblem
}

// Options tunes a health evaluation: Now is the evaluation time and
// PreserveCycleIDs lists cycles the operator asked to keep.
type Options struct {
	Now              time.Time
	PreserveCycleIDs []string
}

// Finding is one operational-health problem with its code, the affected
// resource and a human-readable summary.
type Finding struct {
	Code         string `json:"code"`
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	Summary      string `json:"summary"`
	Path         string `json:"path,omitempty"`
}

// Result is the outcome of a health evaluation; Healthy is true only when
// there are no Findings.
type Result struct {
	Healthy  bool      `json:"healthy"`
	Findings []Finding `json:"findings"`
}

// CycleAuthority is the verdict on whether a cycle still holds authority over
// its worktree and branch.
type CycleAuthority string

// Cycle authority verdicts: live (holder process alive), preserved (kept on
// purpose), dead (no remaining authority) and unknown (identity incomplete).
const (
	AuthorityLive      CycleAuthority = "live"
	AuthorityPreserved CycleAuthority = "preserved"
	AuthorityDead      CycleAuthority = "dead"
	AuthorityUnknown   CycleAuthority = "unknown"
)
