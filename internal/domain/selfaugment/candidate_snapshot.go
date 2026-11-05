package selfaugment

import (
	contract "issueops/internal/contract/selfaugment"
	"time"
)

// NewCandidateExportSnapshot converts result into its schema-version-1 state snapshot and stamps
// generatedAt in RFC 3339 form.
func NewCandidateExportSnapshot(result contract.SelfVerificationCandidateExportResult, generatedAt time.Time) contract.SelfVerificationCandidateExportStateSnapshot {
	return contract.SelfVerificationCandidateExportStateSnapshot{
		SchemaVersion:         1,
		Kind:                  contract.SelfVerificationCandidateExportKind,
		LoopKind:              result.LoopKind,
		KoreanName:            result.KoreanName,
		OK:                    result.OK,
		IssueOpsRoot:          result.IssueOpsRoot,
		GeneratedAt:           generatedAt.Format(time.RFC3339Nano),
		SourcePath:            result.SourcePath,
		CandidateCount:        result.CandidateCount,
		OpenCandidateIDs:      result.OpenCandidateIDs,
		SatisfiedCandidateIDs: result.SatisfiedCandidateIDs,
		SelectedCandidate:     result.SelectedCandidate,
		Candidates:            result.Candidates,
	}
}
