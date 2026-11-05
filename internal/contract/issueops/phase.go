package issueops

// Phase is a stage of the IssueOps cycle lifecycle.
type Phase string

// The Phase values, in lifecycle order from problem framing to done.
const (
	PhaseProblem             Phase = "problem"
	PhaseGrill               Phase = "grill"
	PhasePlan                Phase = "plan"
	PhaseCompatibilityReview Phase = "compatibility-review"
	PhaseImplement           Phase = "implement"
	PhaseAISlopClean         Phase = "ai-slop-clean"
	PhaseFeedback            Phase = "feedback"
	PhasePR                  Phase = "pr"
	PhaseDone                Phase = "done"
)

// Phases lists every Phase in lifecycle order. Iterate it instead of a map of
// phases so rendering and comparison stay deterministic. Callers must not modify it.
var Phases = []Phase{
	PhaseProblem,
	PhaseGrill,
	PhasePlan,
	PhaseCompatibilityReview,
	PhaseImplement,
	PhaseAISlopClean,
	PhaseFeedback,
	PhasePR,
	PhaseDone,
}
