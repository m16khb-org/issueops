package selfaugment

import selfverify "issueops/internal/contract/selfverify"

// SelfVerificationCandidateExportKind 상수는 후보 내보내기 결과와 상태 스냅샷의 Kind 필드에 기록되는 식별 문자열이다.
const SelfVerificationCandidateExportKind = "self_verification_candidate_export"

// SelfVerificationCandidateExportResult 타입은 자기 검증 루프의 개선 후보 목록을 내보낸 결과 DTO이다.
// 열린 후보와 이미 충족된 후보의 ID, 선택된 후보, 상태 체크포인트 기록 결과를 함께 담는다.
type SelfVerificationCandidateExportResult struct {
	OK                    bool                                   `json:"ok"`
	Kind                  string                                 `json:"kind"`
	LoopKind              string                                 `json:"loop_kind"`
	KoreanName            string                                 `json:"korean_name"`
	IssueOpsRoot          string                                 `json:"issueops_root"`
	GeneratedAt           string                                 `json:"generated_at"`
	SourcePath            string                                 `json:"source_path"`
	SourceExists          bool                                   `json:"source_exists"`
	CandidateCount        int                                    `json:"candidate_count"`
	OpenCandidateIDs      []string                               `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                               `json:"satisfied_candidate_ids"`
	SelectedCandidate     *selfverify.SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []selfverify.SelfVerificationCandidate `json:"candidates"`
	StateCheckpoint       *StateCheckpoint                       `json:"state_checkpoint,omitempty"`
	Warnings              []string                               `json:"warnings"`
}

// SelfVerificationCandidateExportStateSnapshot 타입은 후보 내보내기 결과 중 상태 저장소에 체크포인트로 남기는 부분의 스냅샷이다.
// 소스 존재 여부와 경고, 체크포인트 자체 정보는 제외하고 다시 읽을 때 필요한 필드만 담는다.
type SelfVerificationCandidateExportStateSnapshot struct {
	SchemaVersion         int                                    `json:"schema_version"`
	Kind                  string                                 `json:"kind"`
	LoopKind              string                                 `json:"loop_kind"`
	KoreanName            string                                 `json:"korean_name"`
	OK                    bool                                   `json:"ok"`
	IssueOpsRoot          string                                 `json:"issueops_root"`
	GeneratedAt           string                                 `json:"generated_at"`
	SourcePath            string                                 `json:"source_path"`
	CandidateCount        int                                    `json:"candidate_count"`
	OpenCandidateIDs      []string                               `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                               `json:"satisfied_candidate_ids"`
	SelectedCandidate     *selfverify.SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []selfverify.SelfVerificationCandidate `json:"candidates"`
}
