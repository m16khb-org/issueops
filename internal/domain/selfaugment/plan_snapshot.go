package selfaugment

import (
	"time"

	contract "issueops/internal/contract/selfaugment"
)

// NewPlanSnapshot converts result into its schema-version-1 state snapshot, recording open and
// satisfied candidate IDs and stamping generatedAt in RFC 3339 form.
func NewPlanSnapshot(result contract.PlanResult, generatedAt time.Time) contract.PlanStateSnapshot {
	return contract.PlanStateSnapshot{
		SchemaVersion:         1,
		Kind:                  contract.SelfAugmentationPlanKind,
		LoopKind:              result.LoopKind,
		KoreanName:            result.KoreanName,
		OK:                    result.OK,
		Cycles:                result.Cycles,
		TargetScore:           result.TargetScore,
		IssueOpsRoot:          result.IssueOpsRoot,
		GeneratedAt:           generatedAt.Format(time.RFC3339Nano),
		SelectedCandidate:     result.SelectedCandidate,
		CandidateCount:        len(result.Candidates),
		OpenCandidateIDs:      CandidateIDsByStatus(result.Candidates, contract.CandidateStatusOpen),
		SatisfiedCandidateIDs: CandidateIDsByStatus(result.Candidates, contract.CandidateStatusSatisfied),
		Goals:                 result.Goals,
		SelectedFormulas:      result.SelectedFormulas,
		ResearchInfluences:    result.ResearchInfluences,
	}
}

// CandidateIDsByStatus returns the IDs of candidates whose Status equals status, in input order, as
// a non-nil slice.
func CandidateIDsByStatus(candidates []contract.Candidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}
