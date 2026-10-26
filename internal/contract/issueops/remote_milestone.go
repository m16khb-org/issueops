package issueops

// RemoteMilestoneResult is the output of `issueops remote milestone`: a
// user-requested milestone on the cycle's issue or PR/MR, read back.
type RemoteMilestoneResult struct {
	OK          bool   `json:"ok"`
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	On          string `json:"on"`
	TargetURL   string `json:"target_url"`
	Milestone   string `json:"milestone"`
	ReadBack    string `json:"read_back,omitempty"`
	Set         bool   `json:"set"`
	NextCommand string `json:"next_command,omitempty"`
}
