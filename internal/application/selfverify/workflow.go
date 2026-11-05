package selfverify

import contract "issueops/internal/contract/selfaugment"

// ExportAndSaveCandidatesDeps injects the export and save functions that ExportAndSaveCandidates
// sequences.
type ExportAndSaveCandidatesDeps struct {
	Export func() contract.SelfVerificationCandidateExportResult
	Save   func(*contract.SelfVerificationCandidateExportResult, string) error
}

// ExportAndSaveCandidates exports the candidates and, when save is set, saves them under key. A
// save error is returned together with the unsaved result.
func ExportAndSaveCandidates(save bool, key string, deps ExportAndSaveCandidatesDeps) (contract.SelfVerificationCandidateExportResult, error) {
	result := deps.Export()
	if save {
		if err := deps.Save(&result, key); err != nil {
			return result, err
		}
	}
	return result, nil
}
