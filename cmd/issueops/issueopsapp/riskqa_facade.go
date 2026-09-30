package issueopsapp

import (
	"issueops/internal/adapter/verification/riskqa"
	app "issueops/internal/application/selfverify"
)

func validateRiskQATierEvidence(root string) app.RiskQAEvidence {
	step, coversFullGoTest := riskqa.ValidateForSelfVerify(root)
	return app.RiskQAEvidence{
		Step:             step,
		CoversFullGoTest: coversFullGoTest,
	}
}
