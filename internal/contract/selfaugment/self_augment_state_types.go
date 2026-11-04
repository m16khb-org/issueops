package selfaugment

// StateSnapshot 타입은 자기 검증 루프 결과를 상태 저장소에 체크포인트로 남기는 스냅샷이다.
// 반복별 실행 기록은 빼고 요약(Summary)만 보존한다.
type StateSnapshot struct {
	SchemaVersion int     `json:"schema_version"`
	Kind          string  `json:"kind"`
	LoopKind      string  `json:"loop_kind,omitempty"`
	KoreanName    string  `json:"korean_name,omitempty"`
	OK            bool    `json:"ok"`
	Iterations    int     `json:"iterations"`
	BaseSeed      int64   `json:"base_seed"`
	TargetScore   float64 `json:"target_score,omitempty"`
	ElapsedMS     int64   `json:"elapsed_ms"`
	IssueOpsRoot  string  `json:"issueops_root"`
	GeneratedAt   string  `json:"generated_at"`
	Summary       Summary `json:"summary"`
}
