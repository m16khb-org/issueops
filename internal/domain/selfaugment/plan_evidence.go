package selfaugment

// Goal evidence names the exact observable behind each score so an agent can
// re-produce or refute it instead of treating the gate as decorative.
func ImplementationEvidence(passed bool) []string {
	if passed {
		return []string{"git status --porcelain non-empty: uncommitted implementation diff observed"}
	}
	return []string{"git status --porcelain empty: no uncommitted diff found; implement the selected candidate first"}
}

func VerificationEvidence(passed bool) []string {
	if passed {
		return []string{"self-verify-latest state ok=true (from issueops self-verify --save-state)"}
	}
	return []string{"self-verify-latest state absent or failing; run issueops self-verify --save-state after verification"}
}

func LearningEvidence(passed bool) []string {
	if passed {
		return []string{"augmentation lessons present under self-augment-lesson-* state keys"}
	}
	return []string{"no augmentation lessons captured yet; record outcomes via the lesson capture flow"}
}
