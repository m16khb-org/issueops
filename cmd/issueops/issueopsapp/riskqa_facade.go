package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow"
	"issueops/internal/adapter/verification/riskqa"
)

func validateRiskQATierEvidence(root string) selfworkflow.SelfVerifyRiskQAEvidence {
	step, coversFullGoTest := riskqa.ValidateForSelfVerify(root)
	return selfworkflow.SelfVerifyRiskQAEvidence{
		Step:             step,
		CoversFullGoTest: coversFullGoTest,
	}
}
