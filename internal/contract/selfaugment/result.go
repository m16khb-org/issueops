package selfaugment

// SelfVerificationKoreanName 상수는 자기 검증 루프의 한국어 표시 이름이다.
const SelfVerificationKoreanName = "자기 검증 루프"

// Result 타입은 자기 검증 루프 한 번 실행의 최종 결과 DTO이다.
// 반복별 실행 기록, 집계 요약, 상태 체크포인트 기록 결과, 선택적 LLM 평가를 담는다.
type Result struct {
	OK                  bool                     `json:"ok"`
	LoopKind            string                   `json:"loop_kind"`
	KoreanName          string                   `json:"korean_name"`
	Iterations          int                      `json:"iterations"`
	BaseSeed            int64                    `json:"base_seed"`
	TargetScore         float64                  `json:"target_score"`
	TerminationEligible bool                     `json:"termination_eligible"`
	ElapsedMS           int64                    `json:"elapsed_ms"`
	IssueOpsRoot        string                   `json:"issueops_root"`
	InspiredBy          string                   `json:"inspired_by"`
	LoopContract        []string                 `json:"loop_contract"`
	Summary             Summary                  `json:"summary"`
	StateCheckpoint     *StateCheckpoint         `json:"state_checkpoint,omitempty"`
	LLMEval             *SelfVerifyLLMEvalResult `json:"llm_eval,omitempty"`
	Runs                []Iteration              `json:"runs"`
}

// StateCheckpoint 타입은 결과를 상태 저장소에 체크포인트로 기록한 결과이다.
// 실패하면 OK가 false이고 Error에 사유가 들어간다.
type StateCheckpoint struct {
	OK       bool   `json:"ok"`
	Key      string `json:"key"`
	StateDir string `json:"state_dir,omitempty"`
	Path     string `json:"path,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SelfVerifyLLMEvalResult 타입은 자기 검증 증거 묶음을 읽기 전용 LLM 평가에 맡긴 결과이다.
// 점수와 차단 요인, 위험, 권장 다음 행동을 담고 평가가 실패하면 Error에 사유를 남긴다.
type SelfVerifyLLMEvalResult struct {
	OK                     bool     `json:"ok"`
	Mode                   string   `json:"mode"`
	ExecutionClass         string   `json:"execution_class"`
	ReadOnly               bool     `json:"read_only"`
	Score                  float64  `json:"score"`
	Summary                string   `json:"summary,omitempty"`
	Blockers               []string `json:"blockers,omitempty"`
	Risks                  []string `json:"risks,omitempty"`
	RecommendedNextActions []string `json:"recommended_next_actions,omitempty"`
	EvidencePacketBytes    int      `json:"evidence_packet_bytes"`
	Prompt                 string   `json:"prompt,omitempty"`
	Error                  string   `json:"error,omitempty"`
}
