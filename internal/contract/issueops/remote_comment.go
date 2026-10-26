package issueops

// RemoteCommentResult is the output of `issueops remote comment`: one
// user-requested comment on the cycle's issue or PR/MR.
type RemoteCommentResult struct {
	OK        bool   `json:"ok"`
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	On        string `json:"on"`
	TargetURL string `json:"target_url"`
	BodyChars int    `json:"body_chars"`
	// BodySHA256 pins the previewed text; the confirm command repeats it so a
	// --body-file edited after the preview is refused instead of posted.
	BodySHA256  string   `json:"body_sha256"`
	Posted      bool     `json:"posted"`
	CommentURL  string   `json:"comment_url,omitempty"`
	NextCommand string   `json:"next_command,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}
