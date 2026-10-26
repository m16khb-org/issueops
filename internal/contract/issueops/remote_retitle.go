package issueops

// RemoteRetitleResult is the output of `issueops remote retitle`: a
// user-requested title change on the cycle's issue or PR/MR, read back.
type RemoteRetitleResult struct {
	OK          bool   `json:"ok"`
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	On          string `json:"on"`
	TargetURL   string `json:"target_url"`
	Title       string `json:"title"`
	ReadBack    string `json:"read_back,omitempty"`
	Retitled    bool   `json:"retitled"`
	NextCommand string `json:"next_command,omitempty"`
}
