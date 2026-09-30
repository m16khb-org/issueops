package update

type MCPProxyProcess struct {
	PID              int
	ParentPID        int
	Command          string
	StartTime        string
	Executable       string
	IdentityVerified bool
}

type MCPCleanupProcess struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Action  string `json:"action"`
}

type MCPCleanupResult struct {
	OK         bool                `json:"ok"`
	DryRun     bool                `json:"dry_run"`
	Matched    int                 `json:"matched"`
	Terminated int                 `json:"terminated"`
	Processes  []MCPCleanupProcess `json:"processes"`
	Message    string              `json:"message,omitempty"`
}
