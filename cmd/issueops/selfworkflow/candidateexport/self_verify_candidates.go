package candidateexport

import (
	"path/filepath"
	"time"

	augmentcontract "issueops/internal/contract/selfaugment"
	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

const SelfVerificationCandidateExportKind = augmentcontract.SelfVerificationCandidateExportKind

type SelfVerificationCandidateExportResult = augmentcontract.SelfVerificationCandidateExportResult

type SelfVerificationCandidate = contract.SelfVerificationCandidate

type SelfVerificationCandidateExportStateSnapshot = augmentcontract.SelfVerificationCandidateExportStateSnapshot

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
