package selfaugment

import (
	"time"

	contract "issueops/internal/contract/selfaugment"
)

const SelfVerificationSummaryKind = "self_verification_summary"

func NewSelfVerificationSummarySnapshot(result contract.Result, generatedAt time.Time) contract.StateSnapshot {
	return contract.StateSnapshot{
		SchemaVersion: 1,
		Kind:          SelfVerificationSummaryKind,
		LoopKind:      result.LoopKind,
		KoreanName:    result.KoreanName,
		OK:            result.OK,
		Iterations:    result.Iterations,
		BaseSeed:      result.BaseSeed,
		TargetScore:   result.TargetScore,
		ElapsedMS:     result.ElapsedMS,
		IssueOpsRoot:  result.IssueOpsRoot,
		GeneratedAt:   generatedAt.Format(time.RFC3339Nano),
		Summary:       result.Summary,
	}
}
