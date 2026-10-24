package issueopscycle

import (
	"errors"
	"reflect"
	"testing"

	gatescontract "issueops/internal/contract/gates"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestApplyGateLedgersSkipsOtherIssueAndChecksOwnLedgerOnly(t *testing.T) {
	files := []string{"/repo/.issueops/issues/21/gates.md", "/repo/.issueops/issues/248/gates.md"}
	var checked []string
	ports := cycleport.GateLedgerReadiness{
		Discover: func(string) ([]string, error) { return files, nil },
		Check: func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
			checked = append(checked, req.Files[0])
			return gatescontract.CheckResult{Complete: false, Files: []gatescontract.FileResult{{Gates: []gatescontract.GateResult{{ID: "G1", State: "evidence_pending", Title: "proof"}}}}}, nil
		},
	}
	ready := ApplyGateLedgers(model.IssueOpsReadiness{Ready: true}, "/repo", "21", ports)
	if !reflect.DeepEqual(checked, files[:1]) || !reflect.DeepEqual(ready.Missing, []string{"gates_incomplete:.issueops/issues/21/gates.md"}) ||
		!reflect.DeepEqual(ready.Warnings, []string{"gates_skipped:1 (.issueops/issues/248/gates.md): other issues' ledgers, not judged for this cycle; no action needed", "gate G1 (evidence_pending): proof"}) || ready.Ready {
		t.Fatalf("checked=%v ready=%+v", checked, ready)
	}
}

func TestApplyGateLedgersLeavesReadinessUntouchedWhenNothingToJudge(t *testing.T) {
	base := model.IssueOpsReadiness{Ready: true}
	checks := 0
	check := func(gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
		checks++
		return gatescontract.CheckResult{}, nil
	}
	for name, ports := range map[string]cycleport.GateLedgerReadiness{
		"discovery failure": {Discover: func(string) ([]string, error) {
			return []string{"/repo/.issueops/gates/a.md"}, errors.New("walk failed")
		}, Check: check},
		"no ledgers": {Discover: func(string) ([]string, error) { return nil, nil }, Check: check},
	} {
		if got := ApplyGateLedgers(base, "/repo", "1", ports); !reflect.DeepEqual(got, base) || checks != 0 {
			t.Fatalf("%s: got=%+v checks=%d", name, got, checks)
		}
	}
	discovered := 0
	counting := cycleport.GateLedgerReadiness{Discover: func(string) ([]string, error) { discovered++; return nil, nil }, Check: check}
	if got := ApplyGateLedgers(base, "   ", "1", counting); !reflect.DeepEqual(got, base) || discovered != 0 {
		t.Fatalf("blank root must not discover: got=%+v discovered=%d", got, discovered)
	}
}

func TestApplyGateLedgersReportsUncheckableLedgerAsWarningNotBlocker(t *testing.T) {
	ports := cycleport.GateLedgerReadiness{
		Discover: func(string) ([]string, error) { return []string{"/repo/.issueops/gates/a.md"}, nil },
		Check: func(gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
			return gatescontract.CheckResult{}, errors.New("ledger parse failed")
		},
	}
	got := ApplyGateLedgers(model.IssueOpsReadiness{Ready: true}, "/repo", "", ports)
	if !got.Ready || len(got.Missing) != 0 || !reflect.DeepEqual(got.Warnings, []string{"gates .issueops/gates/a.md: ledger parse failed"}) {
		t.Fatalf("got=%+v", got)
	}
}

func TestApplyGateLedgersAcceptsCompleteLedger(t *testing.T) {
	ports := cycleport.GateLedgerReadiness{
		Discover: func(string) ([]string, error) { return []string{"/repo/.issueops/gates/a.md"}, nil },
		Check: func(gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
			return gatescontract.CheckResult{Complete: true}, nil
		},
	}
	got := ApplyGateLedgers(model.IssueOpsReadiness{Ready: true}, "/repo", "", ports)
	if !got.Ready || len(got.Missing) != 0 || len(got.Warnings) != 0 {
		t.Fatalf("got=%+v", got)
	}
}
