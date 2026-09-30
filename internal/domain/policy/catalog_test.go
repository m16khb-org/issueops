package policy

import (
	"testing"

	policycontract "issueops/internal/contract/policy"
)

func TestCatalogClassifiesBuiltinsAndWorkspaceOverrides(t *testing.T) {
	builtin := BuiltinCatalog()
	if got := builtin.Classify([]string{"git", "status"}); !got.ReadOnlyAllowed || got.Writes || got.UsesNetwork {
		t.Fatalf("git status classification=%+v", got)
	}
	if got := builtin.Classify([]string{"git", "push"}); got.ReadOnlyAllowed || !got.Writes || !got.UsesNetwork {
		t.Fatalf("git push classification=%+v", got)
	}
	overridden := BuiltinCatalog()
	overridden.Apply(policycontract.PolicyOverrides{AdditionalReadOnlyCommands: []string{"echo"}})
	if !overridden.Classify([]string{"echo", "ok"}).ReadOnlyAllowed {
		t.Fatal("workspace override did not apply")
	}
	if builtin.Classify([]string{"echo", "ok"}).ReadOnlyAllowed {
		t.Fatal("workspace override leaked to builtin catalog")
	}
}
