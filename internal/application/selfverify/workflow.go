package selfverify

import contract "issueops/internal/contract/selfaugment"

type ExportAndSaveCandidatesDeps struct {
	Export func() contract.SelfVerificationCandidateExportResult
	Save   func(*contract.SelfVerificationCandidateExportResult, string) error
}

func ExportAndSaveCandidates(save bool, key string, deps ExportAndSaveCandidatesDeps) (contract.SelfVerificationCandidateExportResult, error) {
	result := deps.Export()
	if save {
		if err := deps.Save(&result, key); err != nil {
			return result, err
		}
	}
	return result, nil
}
