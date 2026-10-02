package mcpcli

import (
	"context"

	contract "issueops/internal/contract/selfaugment"
)

type SelfPlanningDependencies struct {
	Plan             func(contract.SelfAugmentPlanRequest) contract.SelfAugmentPlanResult
	ExportCandidates func() contract.SelfVerificationCandidateExportResult
	SaveCandidates   func(context.Context, *contract.SelfVerificationCandidateExportResult, string) error
	SaveLesson       func(context.Context, contract.SelfAugmentLessonRequest) (contract.SelfAugmentLessonResult, error)
}
