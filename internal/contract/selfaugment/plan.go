package selfaugment

// PlanRequest 타입은 자가 증강 계획 요청 입력이다. Cycles는 계획할 반복 횟수, TargetScore는 목표 점수이다.
type PlanRequest struct {
	Cycles      int     `json:"cycles"`
	TargetScore float64 `json:"target_score"`
}

// SelfAugmentationPlanKind 상수는 계획 결과/스냅샷의 Kind 값이고, SelfAugmentationLessonKind 상수는 교훈 결과/스냅샷의 Kind 값이다.
const (
	SelfAugmentationPlanKind   = "self_augmentation_plan"
	SelfAugmentationLessonKind = "self_augmentation_lesson"
)

// PlanResult 타입은 자가 증강 계획 결과 DTO이다.
// 목표, 후보 목록, 선택된 후보, 실행 프로토콜과 검증 게이트, 저장소 신호를 담는다.
type PlanResult struct {
	OK                  bool             `json:"ok"`
	LoopKind            string           `json:"loop_kind"`
	KoreanName          string           `json:"korean_name"`
	Cycles              int              `json:"cycles"`
	TargetScore         float64          `json:"target_score"`
	TerminationEligible bool             `json:"termination_eligible"`
	IssueOpsRoot        string           `json:"issueops_root"`
	GeneratedAt         string           `json:"generated_at"`
	GeniusThinkPath     string           `json:"genius_think_path"`
	UsesGeniusThink     bool             `json:"uses_genius_think"`
	SelectedFormulas    []string         `json:"selected_formulas"`
	ResearchInfluences  []Influence      `json:"research_influences"`
	Goals               []Goal           `json:"goals"`
	Candidates          []Candidate      `json:"candidates"`
	SelectedCandidate   *Candidate       `json:"selected_candidate,omitempty"`
	ExecutionProtocol   []string         `json:"execution_protocol"`
	VerificationGate    []string         `json:"verification_gate"`
	Warnings            []string         `json:"warnings"`
	RepoSignals         RepoSignals      `json:"repo_signals"`
	StateCheckpoint     *StateCheckpoint `json:"state_checkpoint,omitempty"`
}

// Influence 타입은 계획에 반영한 외부 연구/기법 한 건과 그중 채택한 내용을 기록한다.
type Influence struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Adopted string `json:"adopted"`
}

// LessonRequest 타입은 자가 증강 중 얻은 교훈을 상태 저장소에 남기라는 요청이다.
type LessonRequest struct {
	CandidateID string `json:"candidate_id"`
	Lesson      string `json:"lesson"`
	NextAction  string `json:"next_action"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
	StateKey    string `json:"state_key"`
}

// LessonResult 타입은 교훈 기록 요청의 결과 DTO이다. 기록된 교훈 내용과 상태 체크포인트 정보를 담는다.
type LessonResult struct {
	OK              bool             `json:"ok"`
	Kind            string           `json:"kind"`
	LoopKind        string           `json:"loop_kind"`
	KoreanName      string           `json:"korean_name"`
	CandidateID     string           `json:"candidate_id"`
	Lesson          string           `json:"lesson"`
	NextAction      string           `json:"next_action"`
	Source          string           `json:"source"`
	Severity        string           `json:"severity"`
	IssueOpsRoot    string           `json:"issueops_root"`
	GeneratedAt     string           `json:"generated_at"`
	StateCheckpoint *StateCheckpoint `json:"state_checkpoint,omitempty"`
}

// LessonStateSnapshot 타입은 교훈 기록 결과 중 상태 저장소에 체크포인트로 남기는 스냅샷이다.
type LessonStateSnapshot struct {
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

// PlanStateSnapshot 타입은 계획 결과 중 상태 저장소에 체크포인트로 남기는 스냅샷이다.
// 후보 전체 대신 선택된 후보와 열린/충족된 후보 ID 목록만 보존한다.
type PlanStateSnapshot struct {
	SchemaVersion         int         `json:"schema_version"`
	Kind                  string      `json:"kind"`
	LoopKind              string      `json:"loop_kind"`
	KoreanName            string      `json:"korean_name"`
	OK                    bool        `json:"ok"`
	Cycles                int         `json:"cycles"`
	TargetScore           float64     `json:"target_score"`
	IssueOpsRoot          string      `json:"issueops_root"`
	GeneratedAt           string      `json:"generated_at"`
	SelectedCandidate     *Candidate  `json:"selected_candidate,omitempty"`
	CandidateCount        int         `json:"candidate_count"`
	OpenCandidateIDs      []string    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string    `json:"satisfied_candidate_ids"`
	Goals                 []Goal      `json:"goals"`
	SelectedFormulas      []string    `json:"selected_formulas"`
	ResearchInfluences    []Influence `json:"research_influences"`
}
