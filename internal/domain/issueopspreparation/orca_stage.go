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

// ValidateOrcaStageReceipts preserves the ordered stage progression after the
// adapter verifies the worktree and launch evidence for an owner stage.
func ValidateOrcaStageReceipts(facts OrcaStageFacts) error {
	switch facts.Stage {
	case "worktree_create":
		if facts.Prepared || facts.Launch || facts.TerminalPTYID != "" || facts.RunID != "" || facts.RunBound || facts.TaskID != "" {
			return fmt.Errorf("worktree intent contains a later-stage receipt")
		}
	case "terminal_create":
		if facts.TerminalPTYID != "" || facts.RunID != "" || facts.RunBound || facts.TaskID != "" {
			return fmt.Errorf("terminal intent contains a later-stage receipt")
		}
	case "run_create":
		if facts.TerminalPTYID == "" || facts.RunID != "" || facts.RunBound || facts.TaskID != "" {
			return fmt.Errorf("Run intent requires exactly one terminal receipt")
		}
	case "run_bind":
		if facts.TerminalPTYID == "" || facts.RunID == "" || facts.RunBound || facts.TaskID != "" {
			return fmt.Errorf("Run bind intent requires terminal and Run receipts")
		}
	case "task_create":
		if facts.TerminalPTYID == "" || facts.RunID == "" || !facts.RunBound || facts.TaskID != "" {
			return fmt.Errorf("task intent requires terminal and bound Run receipts")
		}
	case "dispatch":
		if facts.TerminalPTYID == "" || facts.RunID == "" || !facts.RunBound || facts.TaskID == "" {
			return fmt.Errorf("dispatch intent requires terminal, bound Run, and task receipts")
		}
	default:
		return fmt.Errorf("unsupported Orca execution intent stage %q", facts.Stage)
	}
	return nil
}
