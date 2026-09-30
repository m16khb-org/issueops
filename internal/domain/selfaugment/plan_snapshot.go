package selfaugment

import (
	"time"

	contract "issueops/internal/contract/selfaugment"
)

func NewPlanSnapshot(result contract.SelfAugmentPlanResult, generatedAt time.Time) contract.SelfAugmentPlanStateSnapshot {
	return contract.SelfAugmentPlanStateSnapshot{
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

func CandidateIDsByStatus(candidates []contract.SelfAugmentCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}
