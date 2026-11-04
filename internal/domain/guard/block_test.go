package guard

import (
	contract "issueops/internal/contract/guard"
	"testing"
)

func TestBlockingErrorUsesResultAuthorityAndBlockFindings(t *testing.T) {
	findings := []contract.Finding{{Severity: "warn", Rule: "warning"}, {Severity: "block", Rule: "first"}, {Severity: "block", Rule: "second"}}
	if err := BlockingError(contract.CheckResult{OK: true, Findings: findings}); err != nil {
		t.Fatal(err)
	}
	err := BlockingError(contract.CheckResult{Findings: findings})
	blocked, ok := err.(contract.BlockedError)
	if !ok || len(blocked.Findings) != 2 || blocked.Findings[0].Rule != "first" || blocked.Error() != "guard check blocked: first" {
		t.Fatalf("error=%#v", err)
	}
	empty := BlockingError(contract.CheckResult{})
	if empty == nil || empty.Error() != "guard check blocked" {
		t.Fatalf("empty=%v", empty)
	}
}
