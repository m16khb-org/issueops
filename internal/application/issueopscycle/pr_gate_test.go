package issueopscycle

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestApplyLoopGatePreservesMissingAndWarningOrder(t *testing.T) {
	ready := model.IssueOpsReadiness{Ready: true, Missing: []string{"base"}, Warnings: []string{"existing"}}
	got := ApplyLoopGate(ready, "repo", func(string) ([]string, []string) { return []string{"loop_active", "base"}, []string{"loop running"} })
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"base", "loop_active"}) || !reflect.DeepEqual(got.Warnings, []string{"existing", "loop running"}) {
		t.Fatalf("loop readiness=%+v", got)
	}
}

func TestGuardPRPhaseSkipsUnrelatedTargetsAndGatesPRReentry(t *testing.T) {
	reads, gates := 0, 0
	ports := cycleport.PRPhaseGuard{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			reads++
			return model.IssueOpsRecord{Phase: model.IssueOpsPhasePR}, nil
		},
		Gate: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			gates++
			return model.IssueOpsReadiness{Missing: []string{"loop_active"}}
		},
	}
	if err := GuardPRPhase("state", "id", "feedback", ports); err != nil || reads != 0 || gates != 0 {
		t.Fatalf("unrelated phase err=%v reads=%d gates=%d", err, reads, gates)
	}
	// Re-entering pr reseals pr_head, so it passes the same loop and ledger gate as entry.
	if err := GuardPRPhase("state", "id", "pr", ports); err == nil || !strings.Contains(err.Error(), "cannot enter pr phase: missing loop_active") || reads != 1 || gates != 1 {
		t.Fatalf("pr reseal err=%v reads=%d gates=%d", err, reads, gates)
	}
}

func TestGuardPRPhaseReturnsReadErrorWithoutEvaluatingGate(t *testing.T) {
	boom := errors.New("record unreadable")
	gates := 0
	err := GuardPRPhase("state", "id", " pr ", cycleport.PRPhaseGuard{
		Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, boom },
		Gate: func(model.IssueOpsRecord) model.IssueOpsReadiness { gates++; return model.IssueOpsReadiness{} },
	})
	if !errors.Is(err, boom) || gates != 0 {
		t.Fatalf("err=%v gates=%d", err, gates)
	}
}
