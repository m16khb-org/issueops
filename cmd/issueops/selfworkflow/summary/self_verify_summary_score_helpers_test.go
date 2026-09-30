package summary

import app "issueops/internal/application/selfverify"

func MapGoalScores(result SelfAugmentResult, target float64) []SelfVerificationGoalScore {
	return app.MapGoalScores(result, target)
}
