package rerun

import domain "issueops/internal/domain/selfverify"

func SelfVerifyRerunCommands(step string, seed int64, score float64) []string {
	return domain.SelfVerifyRerunCommands(step, seed, score)
}
func SelfVerifyStepRerunCommand(label string) (string, bool) {
	return domain.SelfVerifyStepRerunCommand(label)
}
func FormatScore(score float64) string { return domain.FormatScore(score) }
