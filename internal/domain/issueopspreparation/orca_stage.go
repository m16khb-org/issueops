package issueopspreparation

import "fmt"

const (
	OrcaStageInvocation = "invocation"
	OrcaStageInspection = "inspection"
)

type OrcaStageFacts struct {
	Mode          string
	Stage         string
	Prepared      bool
	Launch        bool
	TerminalPTYID string
	RunID         string
	RunBound      bool
	TaskID        string
}

// ValidateOrcaStagePrerequisites determines which sealed owner evidence the
// adapter must verify before a mutation or a read-only recovery inspection.
func ValidateOrcaStagePrerequisites(facts OrcaStageFacts) (bool, error) {
	if facts.Mode != OrcaStageInvocation && facts.Mode != OrcaStageInspection {
		return false, fmt.Errorf("unsupported Orca validation mode %q", facts.Mode)
	}
	switch facts.Stage {
	case "worktree_create":
		return false, nil
	case "terminal_create", "run_create", "run_bind", "task_create", "dispatch":
		if facts.Mode == OrcaStageInspection {
			if !facts.Prepared || !facts.Launch {
				return false, fmt.Errorf("owner intent requires sealed worktree and launch receipts")
			}
		} else if !facts.Prepared {
			return false, fmt.Errorf("owner intent requires a sealed worktree receipt")
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported Orca execution intent stage %q", facts.Stage)
	}
}

// orcaReceiptShape is the exact later-stage receipt presence an owner stage
// requires; every field is compared for equality, so absent receipts must be
// absent and required receipts must be present.
type orcaReceiptShape struct {
	terminal, run, bound, task bool
	// noWorktreeEvidence additionally requires Prepared and Launch to be unset.
	noWorktreeEvidence bool
	message            string
}

var orcaReceiptShapes = map[string]orcaReceiptShape{
	"worktree_create": {noWorktreeEvidence: true, message: "worktree intent contains a later-stage receipt"},
	"terminal_create": {message: "terminal intent contains a later-stage receipt"},
	"run_create":      {terminal: true, message: "Run intent requires exactly one terminal receipt"},
	"run_bind":        {terminal: true, run: true, message: "Run bind intent requires terminal and Run receipts"},
	"task_create":     {terminal: true, run: true, bound: true, message: "task intent requires terminal and bound Run receipts"},
	"dispatch":        {terminal: true, run: true, bound: true, task: true, message: "dispatch intent requires terminal, bound Run, and task receipts"},
}

func (shape orcaReceiptShape) matches(facts OrcaStageFacts) bool {
	if shape.noWorktreeEvidence && (facts.Prepared || facts.Launch) {
		return false
	}
	return (facts.TerminalPTYID != "") == shape.terminal &&
		(facts.RunID != "") == shape.run &&
		facts.RunBound == shape.bound &&
		(facts.TaskID != "") == shape.task
}

// ValidateOrcaStageReceipts preserves the ordered stage progression after the
// adapter verifies the worktree and launch evidence for an owner stage.
func ValidateOrcaStageReceipts(facts OrcaStageFacts) error {
	shape, ok := orcaReceiptShapes[facts.Stage]
	if !ok {
		return fmt.Errorf("unsupported Orca execution intent stage %q", facts.Stage)
	}
	if !shape.matches(facts) {
		return fmt.Errorf("%s", shape.message)
	}
	return nil
}
