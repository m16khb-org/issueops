package guard

import (
	contract "issueops/internal/contract/guard"
	"testing"
)

func TestBlockingErrorUsesResultAuthorityAndBlockFindings(t *testing.T) {
	findings := []contract.GuardFinding{{Severity: "warn", Rule: "warning"}, {Severity: "block", Rule: "first"}, {Severity: "block", Rule: "second"}}
	if err := BlockingError(contract.GuardCheckResult{OK: true, Findings: findings}); err != nil {
		t.Fatal(err)
	}
	err := BlockingError(contract.GuardCheckResult{Findings: findings})
	blocked, ok := err.(contract.GuardBlockedError)
	if !ok || len(blocked.Findings) != 2 || blocked.Findings[0].Rule != "first" || blocked.Error() != "guard check blocked: first" {
		t.Fatalf("error=%#v", err)
	}
	empty := BlockingError(contract.GuardCheckResult{})
	if empty == nil || empty.Error() != "guard check blocked" {
		t.Fatalf("empty=%v", empty)
	}
}
