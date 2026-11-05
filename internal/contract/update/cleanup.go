// Package update defines the DTOs for MCP proxy process cleanup during
// `issueops update`. It sits in the contract layer.
package update

// MCPProxyProcess is one observed MCP proxy process. IdentityVerified records
// whether the process identity (executable and start time) was confirmed, which
// cleanup should require before terminating it.
type MCPProxyProcess struct {
	PID              int
	ParentPID        int
	Command          string
	StartTime        string
	Executable       string
	IdentityVerified bool
}

// MCPCleanupProcess is one matched process and the action cleanup took or
// would take on it.
type MCPCleanupProcess struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Action  string `json:"action"`
}

// MCPCleanupResult summarizes a cleanup run. With DryRun set, nothing is
// terminated and Processes lists what would be acted on.
type MCPCleanupResult struct {
	OK         bool                `json:"ok"`
	DryRun     bool                `json:"dry_run"`
	Matched    int                 `json:"matched"`
	Terminated int                 `json:"terminated"`
	Processes  []MCPCleanupProcess `json:"processes"`
	Message    string              `json:"message,omitempty"`
}
