package issueops

// RemoteAssignResult is the output of `issueops remote assign`: assignees the
// user asked to add to the cycle's issue or PR/MR, read back.
type RemoteAssignResult struct {
	OK          bool     `json:"ok"`
	ID          string   `json:"id"`
	Provider    string   `json:"provider"`
	On          string   `json:"on"`
	TargetURL   string   `json:"target_url"`
	Add         []string `json:"add"`
	Assignees   []string `json:"assignees,omitempty"`
	Assigned    bool     `json:"assigned"`
	NextCommand string   `json:"next_command,omitempty"`
}
