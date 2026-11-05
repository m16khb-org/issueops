// Package issueopsremote is the domain layer for remote issue and PR/MR artifacts: provider and project-key resolution from URLs, validation of create, comment, and verify requests against sealed cycle authority, and scoring of related-issue and label candidates. It performs no I/O; callers supply the observed facts.
package issueopsremote

const defaultIssueOpsRemoteThreshold = 0.70

// ArtifactDraft is the title and body of an issue or PR/MR being drafted; Provider is optional.
type ArtifactDraft struct {
	Provider string `json:"provider,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

// IssueCandidate is an existing issue considered as related to the draft. A non-nil Score replaces the computed one.
type IssueCandidate struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	URL    string   `json:"url,omitempty"`
	State  string   `json:"state,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Score  *float64 `json:"score,omitempty"`
}

// LabelCandidate is a provider label considered for the draft, with optional description, aliases, and explicit Score.
type LabelCandidate struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Score       *float64 `json:"score,omitempty"`
}

// ScoringRequest is the input to candidate scoring: provider, selection threshold (0.70 when unset), the draft, and the issue and label candidates.
type ScoringRequest struct {
	Provider        string           `json:"provider,omitempty"`
	Threshold       float64          `json:"threshold,omitempty"`
	Issue           ArtifactDraft    `json:"issue"`
	IssueCandidates []IssueCandidate `json:"issue_candidates,omitempty"`
	LabelCandidates []LabelCandidate `json:"label_candidates,omitempty"`
}

// ScoredItem is one scored candidate with its score, threshold, selection outcome, and evidence, plus an apply hint when selected or a reject reason when not.
type ScoredItem struct {
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

// ScoringResult is the scoring output: selected and rejected issues and labels, apply instructions, and warnings. It is read-only and must be joined before any remote artifact write.
type ScoringResult struct {
	OK                    bool         `json:"ok"`
	Provider              string       `json:"provider"`
	Threshold             float64      `json:"threshold"`
	ExecutionClass        string       `json:"execution_class,omitempty"`
	ReadOnly              bool         `json:"read_only,omitempty"`
	JoinBefore            string       `json:"join_before,omitempty"`
	SelectedRelatedIssues []ScoredItem `json:"selected_related_issues"`
	RejectedRelatedIssues []ScoredItem `json:"rejected_related_issues"`
	SelectedLabels        []ScoredItem `json:"selected_labels"`
	RejectedLabels        []ScoredItem `json:"rejected_labels"`
	ApplyInstructions     []string     `json:"apply_instructions"`
	Warnings              []string     `json:"warnings"`
}

// LLMJudgeRequest wraps a ScoringRequest that the host agent judges instead of the local heuristic.
type LLMJudgeRequest struct {
	Request ScoringRequest
}

// JudgePromptResult is the host-agent judge prompt together with its execution contract: a read-only background_join that must finish before remote_artifact_write.
type JudgePromptResult struct {
	OK             bool   `json:"ok"`
	ExecutionClass string `json:"execution_class"`
	ReadOnly       bool   `json:"read_only"`
	JoinBefore     string `json:"join_before"`
	Prompt         string `json:"prompt"`
}
