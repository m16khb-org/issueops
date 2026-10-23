package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestBranchRetargetCLIForwardsRequestAndPrintsRecord(t *testing.T) {
	prev := testIssueOpsRuntime.RetargetIssueOpsBranchWithActor
	t.Cleanup(func() { testIssueOpsRuntime.RetargetIssueOpsBranchWithActor = prev })
	var got issueopscontract.IssueOpsBranchRetargetRequest
	var gotID string
	testIssueOpsRuntime.RetargetIssueOpsBranchWithActor = func(_ string, id string, req issueopscontract.IssueOpsBranchRetargetRequest, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
		gotID, got = id, req
		return issueopscontract.IssueOpsRecord{OK: true, ID: id, BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			BaseBranch: req.BaseBranch,
			Retargets:  []issueopscontract.IssueOpsBranchRetarget{{FromBase: "release/stg", ToBase: req.BaseBranch, Reason: req.Reason}},
		}}, nil
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "retarget", "--id", "io-1", "--base-branch", "2803-umbrella", "--reason", "child MR retargeted to the umbrella", "--json"})
	})
	if gotID != "io-1" || got.BaseBranch != "2803-umbrella" || !strings.Contains(got.Reason, "umbrella") {
		t.Fatalf("retarget request must reach the core unchanged: id=%q req=%+v", gotID, got)
	}
	var ack map[string]any
	if err := json.Unmarshal([]byte(out), &ack); err != nil {
		t.Fatalf("branch retarget should return JSON: %v\n%s", err, out)
	}
	if ack["ok"] != true || ack["id"] != "io-1" || ack["branch_prepare"] != nil {
		t.Fatalf("branch retarget must answer with the compact mutation ack: %#v", ack)
	}
}

func TestBranchUsageListsRetarget(t *testing.T) {
	if !strings.Contains(issueOpsBranchPrepareUsage, "branch retarget --id ID --base-branch REF --reason TEXT") {
		t.Fatalf("branch usage must document retarget: %s", issueOpsBranchPrepareUsage)
	}
}
