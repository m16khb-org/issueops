package selfaugment

import (
	contract "issueops/internal/contract/selfaugment"
	verify "issueops/internal/domain/selfverify"
)

func NewCandidateExport(sourceExists bool) contract.SelfVerificationCandidateExportResult {
	candidates := verify.CandidateCatalog()
	warnings := []string{}
	if !sourceExists {
		warnings = append(warnings, "skills/self-verify/CANDIDATES.md not found; using built-in candidate export catalog")
	}
	return contract.SelfVerificationCandidateExportResult{
		OK: true, Kind: contract.SelfVerificationCandidateExportKind, LoopKind: "self_verification", KoreanName: contract.SelfVerificationKoreanName,
		SourceExists: sourceExists, CandidateCount: len(candidates),
		OpenCandidateIDs:      verify.CandidateIDsByStatus(candidates, contract.CandidateStatusOpen),
		SatisfiedCandidateIDs: verify.CandidateIDsByStatus(candidates, contract.CandidateStatusSatisfied),
		SelectedCandidate:     verify.SelectedCandidate(candidates), Candidates: candidates, Warnings: warnings,
	}
}
