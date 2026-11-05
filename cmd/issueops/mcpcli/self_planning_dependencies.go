package mcpcli

import (
	"context"

	contract "issueops/internal/contract/selfaugment"
)

// SelfPlanningDependencies 타입은 self-augment 계획 수립, 후보 내보내기와 저장, lesson 저장 연산을 묶는다.
type SelfPlanningDependencies struct {
	Plan             func(contract.PlanRequest) contract.PlanResult
	ExportCandidates func() contract.SelfVerificationCandidateExportResult
	SaveCandidates   func(context.Context, *contract.SelfVerificationCandidateExportResult, string) error
	SaveLesson       func(context.Context, contract.LessonRequest) (contract.LessonResult, error)
}
