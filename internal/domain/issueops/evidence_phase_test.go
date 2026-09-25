package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestValidateEvidenceRecordingPhase(t *testing.T) {
	for _, phase := range []model.IssueOpsPhase{model.IssueOpsPhaseProblem, model.IssueOpsPhasePlan, model.IssueOpsPhaseCompatibilityReview} {
		if err := ValidateEvidenceRecordingPhase(phase, "implementation review"); err == nil || !strings.Contains(err.Error(), "implement phase") {
			t.Fatalf("phase %s should reject review: %v", phase, err)
		}
	}
	for _, phase := range []model.IssueOpsPhase{model.IssueOpsPhaseImplement, model.IssueOpsPhaseAISlopClean, model.IssueOpsPhasePR} {
		if err := ValidateEvidenceRecordingPhase(phase, "implementation review"); err != nil {
			t.Fatalf("phase %s should allow review: %v", phase, err)
		}
	}
}
