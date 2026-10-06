package probe

import (
	"issueops/internal/adapter/verification/probe/candidateexport"
	selfverify "issueops/internal/contract/selfverify"
)

func ValidateSelfVerifyCandidateExport(binary, root string, seed int64) selfverify.StepResult {
	return candidateexport.ValidateSelfVerifyCandidateExport(binary, root, seed)
}
