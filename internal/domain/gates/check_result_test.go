package gates

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/gates"
)

func TestNormalizeCheckFillsRootCWDAndTimeout(t *testing.T) {
	got := NormalizeCheck(model.CheckRequest{WorkspaceRoot: "  /ws ", TimeoutSeconds: -5})
	if got.WorkspaceRoot != "/ws" || got.CWD != "/ws" || got.TimeoutSeconds != model.TimeoutDefaultSeconds {
		t.Fatalf("root from empty cwd: %+v", got)
	}
	got = NormalizeCheck(model.CheckRequest{CWD: " /cwd ", TimeoutSeconds: 7})
	if got.WorkspaceRoot != "/cwd" || got.CWD != "/cwd" || got.TimeoutSeconds != 7 {
		t.Fatalf("cwd to root: %+v", got)
	}
	got = NormalizeCheck(model.CheckRequest{WorkspaceRoot: "/a", CWD: "/a/b"})
	if got.WorkspaceRoot != "/a" || got.CWD != "/a/b" {
		t.Fatalf("distinct values must be kept: %+v", got)
	}
}

func TestShouldCheckSkipsAbandonedStatusOnlyCommandlessAndMetGates(t *testing.T) {
	base := Gate{CheckCmd: "true"}
	if !ShouldCheck(base, false) {
		t.Fatal("unchecked gate with a command must be checked")
	}
	cases := map[string]struct {
		gate       Gate
		statusOnly bool
	}{
		"abandoned":  {Gate{CheckCmd: "true", Abandoned: true}, false},
		"statusOnly": {base, true},
		"noCommand":  {Gate{CheckCmd: "   "}, false},
		"met":        {Gate{CheckCmd: "true", Checked: true, Evidence: "ok"}, false},
	}
	for name, c := range cases {
		if ShouldCheck(c.gate, c.statusOnly) {
			t.Errorf("%s: must not be checked", name)
		}
	}
	if !ShouldCheck(Gate{CheckCmd: "true", Checked: true, Evidence: "pending"}, false) {
		t.Fatal("checked gate with pending evidence must be rechecked")
	}
}

func TestApplyCheckMarksPassingGateAndReportsFailures(t *testing.T) {
	ledger := Parse("- [ ] G1: one\n  CHECK: true\n  EVIDENCE: pending\n- [ ] G2: two\n  CHECK: false\n")
	result, warnings, changed := ApplyCheck(&ledger, 0, &CheckOutcome{Passed: true, Evidence: "proof", AuditLogID: "a1"}, "f.md")
	if !changed || len(warnings) != 0 || result.State != StateMet || !result.Checked || result.Evidence != "proof" || result.AuditLogID != "a1" || !result.HasCheck {
		t.Fatalf("pass: result=%+v warnings=%v changed=%v", result, warnings, changed)
	}
	if !strings.Contains(Render(ledger), "- [x] G1: one") || !strings.Contains(Render(ledger), "EVIDENCE: proof") {
		t.Fatalf("ledger not updated:\n%s", Render(ledger))
	}

	result, warnings, changed = ApplyCheck(&ledger, 1, &CheckOutcome{CheckError: "exit code 1", PolicyDenied: true, AuditLogID: "a2"}, "f.md")
	if changed || result.State != StateUnchecked || result.CheckError != "exit code 1" || !result.PolicyDenied {
		t.Fatalf("denied: result=%+v changed=%v", result, changed)
	}
	if !reflect.DeepEqual(warnings, []string{"f.md G2: check command denied by policy"}) {
		t.Fatalf("warnings=%v", warnings)
	}

	_, warnings, _ = ApplyCheck(&ledger, 1, &CheckOutcome{CheckError: "boom"}, "f.md")
	if len(warnings) != 0 {
		t.Fatalf("non-policy failure must not warn: %v", warnings)
	}

	result, _, changed = ApplyCheck(&ledger, 1, nil, "f.md")
	if changed || result.State != StateUnchecked || result.CheckError != "" {
		t.Fatalf("nil outcome must be a pure state report: %+v", result)
	}
}

func TestSummarizeFileAndAddFileResultAggregateTotals(t *testing.T) {
	ledger := Parse("- [x] G1: a\n  EVIDENCE: ok\n- [ ] G2: b\n- [ ] G3: c\nABANDON: G3 skip\n")
	file := SummarizeFile(model.FileResult{}, ledger)
	if file.GateCount != 3 || file.Met != 1 || file.Unmet != 1 || file.Abandoned != 1 || file.Complete {
		t.Fatalf("file=%+v", file)
	}
	total := AddFileResult(model.CheckResult{OK: true}, file, []string{"w"})
	if total.TotalGates != 3 || total.TotalUnmet != 1 || total.Complete || !total.OK || !reflect.DeepEqual(total.Warnings, []string{"w"}) {
		t.Fatalf("total=%+v", total)
	}
	done := SummarizeFile(model.FileResult{}, Parse("- [x] G1: a\n  EVIDENCE: ok\n"))
	total = AddFileResult(model.CheckResult{OK: true}, done, nil)
	if !total.Complete || total.TotalMet != 1 {
		t.Fatalf("complete total=%+v", total)
	}
	total = AddFileResult(total, model.FileResult{Error: "unreadable"}, nil)
	if total.OK || total.Complete {
		t.Fatalf("file error must clear OK and Complete: %+v", total)
	}
}

func TestDeniedAndCompletedCheckOutcomes(t *testing.T) {
	denied := DeniedCheck("id", []string{"a", "b"})
	if !denied.PolicyDenied || denied.Passed || denied.AuditLogID != "id" || denied.CheckError != "check denied by policy: a; b" {
		t.Fatalf("denied=%+v", denied)
	}
	gate := Gate{Expect: "ok"}
	pass := CompletedCheck(gate, "ok pkg\n", "", 0, false, "x")
	if !pass.Passed || pass.Evidence != "ok pkg" || pass.CheckError != "" || pass.AuditLogID != "x" {
		t.Fatalf("pass=%+v", pass)
	}
	fail := CompletedCheck(gate, "nope", "", 0, false, "y")
	if fail.Passed || fail.Evidence != "" || !strings.HasPrefix(fail.CheckError, "expect not matched") {
		t.Fatalf("a failing check must not carry evidence: %+v", fail)
	}
	if out := CompletedCheck(gate, "ok", "", 0, true, ""); out.Passed || !strings.HasPrefix(out.CheckError, "check timed out") {
		t.Fatalf("timeout=%+v", out)
	}
}

func TestValidateLedgerRejectsEmpty(t *testing.T) {
	if err := ValidateLedger(Parse("# Gates: x\n")); err == nil || err.Error() != "no gates found" {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateLedger(Parse("- [ ] G1: a\n")); err != nil {
		t.Fatal(err)
	}
}

func TestSortIssueFoldersOrdersNumericBeforeNamedAndNumericallyAmongNumbers(t *testing.T) {
	names := []string{"beta", "10", "alpha", "2", "1a", "", "100"}
	SortIssueFolders(names)
	want := []string{"2", "10", "100", "", "1a", "alpha", "beta"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %q want %q", names, want)
	}
}

func TestMutationAdmissionRejectsMissingFieldsAndDuplicates(t *testing.T) {
	if err := ValidateCreate("g.md", true); err == nil || !strings.Contains(err.Error(), "g.md") {
		t.Fatalf("existing file must be rejected: %v", err)
	}
	if err := ValidateCreate("g.md", false); err != nil {
		t.Fatal(err)
	}
	created := CreatedLedger(model.InitResult{File: "g.md"}, "- [ ] G1: a\n- [ ] G2: b\n")
	if !created.OK || !created.Created || created.GateCount != 2 {
		t.Fatalf("created=%+v", created)
	}

	for _, req := range []model.AbandonRequest{
		{GateID: "G1", Reason: "r"},
		{File: "f", Reason: "r"},
		{File: "f", GateID: " ", Reason: "r"},
		{File: "f", GateID: "G1", Reason: "  "},
	} {
		if _, _, err := PrepareAbandon(req); err == nil {
			t.Errorf("accepted %+v", req)
		}
	}
	_, res, err := PrepareAbandon(model.AbandonRequest{File: "f", GateID: "G1", Reason: "r"})
	if err != nil || res.File != "f" || res.GateID != "G1" || res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	ledger := Parse("- [ ] G1: a\n- [ ] G2: b\nABANDON: G2 old\n")
	if err := AbandonLedger(&ledger, model.AbandonRequest{File: "f", GateID: "G9", Reason: "r"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing gate: %v", err)
	}
	if err := AbandonLedger(&ledger, model.AbandonRequest{File: "f", GateID: "G2", Reason: "r"}); err == nil || !strings.Contains(err.Error(), "already abandoned") {
		t.Fatalf("double abandon: %v", err)
	}
	if err := AbandonLedger(&ledger, model.AbandonRequest{File: "f", GateID: "G1", Reason: "no longer needed"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(Render(ledger), "ABANDON: G1 no longer needed") {
		t.Fatalf("abandon not recorded:\n%s", Render(ledger))
	}
	if rec := RecordedAbandon(res); !rec.OK || !rec.Recorded {
		t.Fatalf("rec=%+v", rec)
	}
}

func TestNormalizeCheckZeroTimeoutFallsBackToDefault(t *testing.T) {
	got := NormalizeCheck(model.CheckRequest{CWD: "/ws"})
	if got.TimeoutSeconds != model.TimeoutDefaultSeconds {
		t.Fatalf("timeout=%d", got.TimeoutSeconds)
	}
}

func TestSortIssueFoldersHandlesDigitNineLeadingZerosAndEqualNumbers(t *testing.T) {
	names := []string{"19", "9", "2", "01"}
	SortIssueFolders(names)
	if want := []string{"01", "2", "9", "19"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("got %q want %q", names, want)
	}
	names = []string{"1", "01"}
	SortIssueFolders(names)
	if want := []string{"1", "01"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("equal numbers must keep input order: got %q", names)
	}
}
