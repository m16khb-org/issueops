package mcpcli

import contract "issueops/internal/contract/selfaugment"

type SelfPlanningDependencies struct {
	Plan             func(contract.SelfAugmentPlanRequest) contract.SelfAugmentPlanResult
	ExportCandidates func() contract.SelfVerificationCandidateExportResult
	SaveCandidates   func(*contract.SelfVerificationCandidateExportResult, string) error
	SaveLesson       func(contract.SelfAugmentLessonRequest) (contract.SelfAugmentLessonResult, error)
}
