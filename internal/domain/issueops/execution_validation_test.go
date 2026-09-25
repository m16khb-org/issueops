package issueops

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestValidateExecutionRejectsSelectionWithoutReadyProbe(t *testing.T) {
	execution := issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeOrca,
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/repo", Root: "/repo.worktrees/run", Branch: "run",
			BaseHead: strings.Repeat("a", 40), Driver: "orca", LinkedAt: "2026-09-25T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
		Selection: &issueopscontract.ExecutionSelection{
			RequestedMode: "auto", ResolvedMode: "orca", ProbeAttempted: true,
			ProbeAvailable: true, ProbeReady: false,
			ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-09-25T00:00:00Z",
		},
	}
	if err := ValidateExecution(execution); err == nil || err.Error() != "Orca selection requires a ready probe" {
		t.Fatalf("unready Orca selection must be rejected: %v", err)
	}
}
