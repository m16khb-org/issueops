package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// RetiredCycleNote names how a cycle whose record cleanup deleted ended, from
// the outcome cleanup kept. QA: after abandon, `status --id` said only "file
// does not exist", which reads like a failure instead of the confirmation the
// agent was checking for. An unusable outcome yields "".
func RetiredCycleNote(outcome model.IssueOpsCycleOutcome) string {
	if outcome.Outcome == "" {
		return ""
	}
	return fmt.Sprintf("cleanup removed it (outcome %s at %s)", outcome.Outcome, outcome.RetainedAt)
}
