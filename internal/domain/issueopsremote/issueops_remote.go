package remote

const defaultIssueOpsRemoteThreshold = 0.70

type IssueOpsRemoteArtifact struct {
	Provider string `json:"provider,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type IssueOpsRemoteIssueCandidate struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	URL    string   `json:"url,omitempty"`
	State  string   `json:"state,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Score  *float64 `json:"score,omitempty"`
}

type IssueOpsRemoteLabelCandidate struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Score       *float64 `json:"score,omitempty"`
}

type IssueOpsRemoteScoringRequest struct {
	Provider        string                         `json:"provider,omitempty"`
	Threshold       float64                        `json:"threshold,omitempty"`
	Issue           IssueOpsRemoteArtifact         `json:"issue"`
	IssueCandidates []IssueOpsRemoteIssueCandidate `json:"issue_candidates,omitempty"`
	LabelCandidates []IssueOpsRemoteLabelCandidate `json:"label_candidates,omitempty"`
}

type IssueOpsRemoteScoredItem struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Title        string   `json:"title,omitempty"`
	URL          string   `json:"url,omitempty"`
	Score        float64  `json:"score"`
	Threshold    float64  `json:"threshold"`
	Selected     bool     `json:"selected"`
	Evidence     []string `json:"evidence"`
	ApplyHint    string   `json:"apply_hint,omitempty"`
	RejectReason string   `json:"reject_reason,omitempty"`
}

type IssueOpsRemoteScoringResult struct {
	OK                    bool                       `json:"ok"`
	Provider              string                     `json:"provider"`
	Threshold             float64                    `json:"threshold"`
	ExecutionClass        string                     `json:"execution_class,omitempty"`
	ReadOnly              bool                       `json:"read_only,omitempty"`
	JoinBefore            string                     `json:"join_before,omitempty"`
	SelectedRelatedIssues []IssueOpsRemoteScoredItem `json:"selected_related_issues"`
	RejectedRelatedIssues []IssueOpsRemoteScoredItem `json:"rejected_related_issues"`
	SelectedLabels        []IssueOpsRemoteScoredItem `json:"selected_labels"`
	RejectedLabels        []IssueOpsRemoteScoredItem `json:"rejected_labels"`
	ApplyInstructions     []string                   `json:"apply_instructions"`
	Warnings              []string                   `json:"warnings"`
}

type IssueOpsRemoteLLMJudgeRequest struct {
	Request IssueOpsRemoteScoringRequest
}

type IssueOpsRemoteJudgePromptResult struct {
	OK             bool   `json:"ok"`
	ExecutionClass string `json:"execution_class"`
	ReadOnly       bool   `json:"read_only"`
	JoinBefore     string `json:"join_before"`
	Prompt         string `json:"prompt"`
}
