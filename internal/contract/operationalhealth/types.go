package operationalhealth

import "time"

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

const (
	ProcessStatusLive             = "live"
	ProcessStatusDead             = "dead"
	ProcessStatusIdentityMismatch = "identity-mismatch"
	ProcessStatusUnknown          = "unknown"
)

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

type LeaseHolderIndex struct {
	Key         string
	LifecycleID string
	Generation  uint64
	Host        string
	SessionID   string
	AgentID     string
}

type GitWorktree struct {
	Path      string
	Branch    string
	Head      string
	Clean     bool
	Canonical bool
}

type GitRef struct {
	Name     string
	Branch   string
	OID      string
	Location string
}

type InventoryProblem struct {
	Source string
	Code   string
	Detail string
}

type StateArtifact struct {
	Path string
	Code string
}

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

type Options struct {
	Now              time.Time
	PreserveCycleIDs []string
}

type Finding struct {
	Code         string `json:"code"`
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	Summary      string `json:"summary"`
	Path         string `json:"path,omitempty"`
}

type Result struct {
	Healthy  bool      `json:"healthy"`
	Findings []Finding `json:"findings"`
}

type CycleAuthority string

const (
	AuthorityLive      CycleAuthority = "live"
	AuthorityPreserved CycleAuthority = "preserved"
	AuthorityDead      CycleAuthority = "dead"
	AuthorityUnknown   CycleAuthority = "unknown"
)
