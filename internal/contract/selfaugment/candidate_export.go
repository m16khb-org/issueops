package selfaugment

import selfverify "issueops/internal/contract/selfverify"

const SelfVerificationCandidateExportKind = "self_verification_candidate_export"

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
	StateCheckpoint       *SelfAugmentStateCheckpoint            `json:"state_checkpoint,omitempty"`
	Warnings              []string                               `json:"warnings"`
}

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
