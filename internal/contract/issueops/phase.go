package issueops

type Phase string

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
