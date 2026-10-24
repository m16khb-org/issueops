package policy

import (
	"strings"
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

// `sh -n FILE` parses and executes nothing, so it is not a shell execution;
// every other interpreter form stays one.
func TestCatalogTreatsOnlyShellSyntaxChecksAsNonShell(t *testing.T) {
	builtin := BuiltinCatalog()
	for argv, shell := range map[string]bool{
		"sh -n fetch.sh":          false,
		"bash -n a.sh b.sh":       false,
		"sh fetch.sh":             true,
		"sh -c echo":              true,
		"sh -n":                   true,
		"sh -n -c echo":           true,
		"bash -n -O extglob a.sh": true,
		"sh -nx a.sh":             true,
		"fish -n a.fish":          true,
	} {
		if got := builtin.Classify(strings.Fields(argv)).ShellCommand; got != shell {
			t.Errorf("%q: ShellCommand=%v want %v", argv, got, shell)
		}
	}
}
