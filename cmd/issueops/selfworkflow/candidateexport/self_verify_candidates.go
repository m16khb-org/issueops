package candidateexport

import (
	"path/filepath"
	"time"

	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

const SelfVerificationCandidateExportKind = "self_verification_candidate_export"

type SelfVerificationCandidateExportResult struct {
	OK                    bool                        `json:"ok"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	IssueOpsRoot          string                      `json:"issueops_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	SourceExists          bool                        `json:"source_exists"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
	StateCheckpoint       *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	Warnings              []string                    `json:"warnings"`
}

type SelfVerificationCandidate = contract.SelfVerificationCandidate

type SelfVerificationCandidateExportStateSnapshot struct {
	SchemaVersion         int                         `json:"schema_version"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	OK                    bool                        `json:"ok"`
	IssueOpsRoot          string                      `json:"issueops_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
}

func ExportSelfVerificationCandidates(root string) SelfVerificationCandidateExportResult {
	sourcePath := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	sourceExists := fileExists(sourcePath)
	candidates := SelfVerificationCandidateCatalog()
	openIDs := SelfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusOpen)
	satisfiedIDs := SelfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusSatisfied)
	selected := domain.SelectedCandidate(candidates)
	warnings := []string{}
	if !sourceExists {
		warnings = append(warnings, "skills/self-verify/CANDIDATES.md not found; using built-in candidate export catalog")
	}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  SelfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		IssueOpsRoot:          root,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SourcePath:            sourcePath,
		SourceExists:          sourceExists,
		CandidateCount:        len(candidates),
		OpenCandidateIDs:      openIDs,
		SatisfiedCandidateIDs: satisfiedIDs,
		SelectedCandidate:     selected,
		Candidates:            candidates,
		Warnings:              warnings,
	}
}

func SelfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}

func SelectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return domain.SelectedCandidateID(candidate)
}
