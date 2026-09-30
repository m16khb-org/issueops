package issueopscycle

import (
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
		!reflect.DeepEqual(ready.Warnings, []string{"gates_skipped:1 (.issueops/issues/248/gates.md)", "gate G1 (evidence_pending): proof"}) || ready.Ready {
		t.Fatalf("checked=%v ready=%+v", checked, ready)
	}
}
