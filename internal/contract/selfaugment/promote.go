package selfaugment

// SelfAugmentationKoreanName 상수는 자가 증강 루프의 한국어 표시 이름이다.
const SelfAugmentationKoreanName = "자가 증강 루프"

// PromoteResult 타입은 저장된 자기 검증 요약 체크포인트를 기준선 키로 승격하는 작업의 결과이다.
// DryRun이 true이면 실제로 쓰지 않고 승격 가능 여부만 보여 준다.
type PromoteResult struct {
	OK                  bool    `json:"ok"`
	StateDir            string  `json:"state_dir"`
	FromKey             string  `json:"from_key"`
	BaselineKey         string  `json:"baseline_key"`
	Confirm             bool    `json:"confirm"`
	DryRun              bool    `json:"dry_run"`
	Promoted            bool    `json:"promoted"`
	SourcePassed        bool    `json:"source_passed"`
	Path                string  `json:"path,omitempty"`
	Bytes               int     `json:"bytes,omitempty"`
	SnapshotGeneratedAt string  `json:"snapshot_generated_at"`
	Summary             Summary `json:"summary"`
}
