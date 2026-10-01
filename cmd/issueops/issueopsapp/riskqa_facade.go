package issueopsapp

import (
	"issueops/internal/adapter/verification/riskqa"
	app "issueops/internal/application/selfverify"
)

func validateRiskQATierEvidence(root string) app.RiskQAEvidence {
	return validateRiskQATierEvidenceWithScope(root, "")
}

func validateRiskQATierEvidenceWithScope(root, baseRef string) app.RiskQAEvidence {
	step, coversFullGoTest := riskqa.ValidateForSelfVerifyWithScope(root, baseRef)
	return app.RiskQAEvidence{
		Step:             step,
		CoversFullGoTest: coversFullGoTest,
	}
}
