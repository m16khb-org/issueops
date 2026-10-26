package issueops

// RemoteLabelResult is the output of `issueops remote label`: labels the user
// asked to add to the cycle's issue or PR/MR, with the labels read back.
type RemoteLabelResult struct {
	OK          bool     `json:"ok"`
	ID          string   `json:"id"`
	Provider    string   `json:"provider"`
	On          string   `json:"on"`
	TargetURL   string   `json:"target_url"`
	Add         []string `json:"add"`
	Labels      []string `json:"labels,omitempty"`
	Added       bool     `json:"added"`
	NextCommand string   `json:"next_command,omitempty"`
}
