package candidateexport

import (
	"time"

	app "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

const SelfVerificationCandidateExportKind = augmentcontract.SelfVerificationCandidateExportKind

type SelfVerificationCandidateExportResult = augmentcontract.SelfVerificationCandidateExportResult

type SelfVerificationCandidate = contract.SelfVerificationCandidate

type SelfVerificationCandidateExportStateSnapshot = augmentcontract.SelfVerificationCandidateExportStateSnapshot

// Source observation is installed by the composition root.
var ObserveSource func(string) (string, bool)

func ExportSelfVerificationCandidates(root string) SelfVerificationCandidateExportResult {
	return app.ExportCandidates(root, app.ExportCandidatesDeps{Source: ObserveSource, Now: time.Now})
}

func SelfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}

func SelectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return domain.SelectedCandidateID(candidate)
}
