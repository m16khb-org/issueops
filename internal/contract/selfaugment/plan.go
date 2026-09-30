package selfaugment

type SelfAugmentPlanRequest struct {
	Cycles      int     `json:"cycles"`
	TargetScore float64 `json:"target_score"`
}

const (
	SelfAugmentationPlanKind   = "self_augmentation_plan"
	SelfAugmentationLessonKind = "self_augmentation_lesson"
)

type SelfAugmentPlanResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Cycles              int                         `json:"cycles"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	IssueOpsRoot        string                      `json:"issueops_root"`
	GeneratedAt         string                      `json:"generated_at"`
	GeniusThinkPath     string                      `json:"genius_think_path"`
	UsesGeniusThink     bool                        `json:"uses_genius_think"`
	SelectedFormulas    []string                    `json:"selected_formulas"`
	ResearchInfluences  []SelfAugmentInfluence      `json:"research_influences"`
	Goals               []SelfAugmentGoal           `json:"goals"`
	Candidates          []SelfAugmentCandidate      `json:"candidates"`
	SelectedCandidate   *SelfAugmentCandidate       `json:"selected_candidate,omitempty"`
	ExecutionProtocol   []string                    `json:"execution_protocol"`
	VerificationGate    []string                    `json:"verification_gate"`
	Warnings            []string                    `json:"warnings"`
	RepoSignals         SelfAugmentRepoSignals      `json:"repo_signals"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentInfluence struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Adopted string `json:"adopted"`
}

type SelfAugmentLessonRequest struct {
	CandidateID string `json:"candidate_id"`
	Lesson      string `json:"lesson"`
	NextAction  string `json:"next_action"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
	StateKey    string `json:"state_key"`
}

type SelfAugmentLessonResult struct {
	OK              bool                        `json:"ok"`
	Kind            string                      `json:"kind"`
	LoopKind        string                      `json:"loop_kind"`
	KoreanName      string                      `json:"korean_name"`
	CandidateID     string                      `json:"candidate_id"`
	Lesson          string                      `json:"lesson"`
	NextAction      string                      `json:"next_action"`
	Source          string                      `json:"source"`
	Severity        string                      `json:"severity"`
	IssueOpsRoot    string                      `json:"issueops_root"`
	GeneratedAt     string                      `json:"generated_at"`
	StateCheckpoint *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentLessonStateSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	LoopKind      string `json:"loop_kind"`
	KoreanName    string `json:"korean_name"`
	OK            bool   `json:"ok"`
	CandidateID   string `json:"candidate_id"`
	Lesson        string `json:"lesson"`
	NextAction    string `json:"next_action"`
	Source        string `json:"source"`
	Severity      string `json:"severity"`
	IssueOpsRoot  string `json:"issueops_root"`
	GeneratedAt   string `json:"generated_at"`
}

type SelfAugmentPlanStateSnapshot struct {
	SchemaVersion         int                    `json:"schema_version"`
	Kind                  string                 `json:"kind"`
	LoopKind              string                 `json:"loop_kind"`
	KoreanName            string                 `json:"korean_name"`
	OK                    bool                   `json:"ok"`
	Cycles                int                    `json:"cycles"`
	TargetScore           float64                `json:"target_score"`
	IssueOpsRoot          string                 `json:"issueops_root"`
	GeneratedAt           string                 `json:"generated_at"`
	SelectedCandidate     *SelfAugmentCandidate  `json:"selected_candidate,omitempty"`
	CandidateCount        int                    `json:"candidate_count"`
	OpenCandidateIDs      []string               `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string               `json:"satisfied_candidate_ids"`
	Goals                 []SelfAugmentGoal      `json:"goals"`
	SelectedFormulas      []string               `json:"selected_formulas"`
	ResearchInfluences    []SelfAugmentInfluence `json:"research_influences"`
}
