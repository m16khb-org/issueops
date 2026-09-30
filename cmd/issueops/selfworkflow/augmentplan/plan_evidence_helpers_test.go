package augmentplan

import domain "issueops/internal/domain/selfaugment"

func hasImplementationDelta(root string) bool { return Repository.HasImplementationDelta(root) }
func selfVerificationPassed() bool            { return planner().SelfVerificationPassed() }
func lessonsCaptured() bool                   { return planner().LessonsCaptured() }
func implementationEvidence(root string) []string {
	return domain.ImplementationEvidence(hasImplementationDelta(root))
}
func verificationEvidence() []string { return domain.VerificationEvidence(selfVerificationPassed()) }
func learningEvidence() []string     { return domain.LearningEvidence(lessonsCaptured()) }
