package probe

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/candidateexport"

func ValidateSelfVerifyCandidateExport(binary, root string, seed int64) selfverify.StepResult {
	return candidateexport.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func ValidateSelfVerifyCandidateExportWithDeps(binary, root string, seed int64, deps candidateexport.CandidateExportValidationDeps) selfverify.StepResult {
	return candidateexport.ValidateSelfVerifyCandidateExportWithDeps(binary, root, seed, deps)
}

func CandidateExportValidationErrors(key string, exportResult selfaugment.SelfVerificationCandidateExportResult, snapshot selfaugment.SelfVerificationCandidateExportStateSnapshot) []string {
	return candidateexport.CandidateExportValidationErrors(key, exportResult, snapshot)
}
