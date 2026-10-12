package policy

import (
	"os"
	"path/filepath"
	"testing"

	policycontract "issueops/internal/contract/policy"
)

func TestCommandObserverPreservesSuppliedAuditLogIDAndGeneratesMissingOne(t *testing.T) {
	root := t.TempDir()
	req := policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"git", "status"}, Timeout: "30s"}
	if got := evaluateCommandPolicy(req).AuditLogID; got == "" {
		t.Fatal("missing audit id was not generated")
	}
	req.AuditLogID = "audit-supplied-1"
	if got := evaluateCommandPolicy(req).AuditLogID; got != "audit-supplied-1" {
		t.Fatalf("audit id = %q, want supplied value", got)
	}
}

func TestCommandPolicyResolvesMissingPathsThroughSymlinkedParents(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	eval := func(arg string) policycontract.CommandPolicyEvaluation {
		return evaluateCommandPolicy(policycontract.CommandPolicyRequest{
			WorkspaceRoot: root, CWD: root, Argv: []string{"cat", arg}, Timeout: "30s",
		})
	}
	for _, arg := range []string{
		filepath.Join(link, "missing.txt"),
		filepath.Join(link, "missing-dir", "deep", "missing.txt"),
	} {
		if r := eval(arg); r.Allowed || !containsString(r.DenyReasons, "path_outside_workspace") {
			t.Fatalf("%s via symlink to outside not denied: %+v", arg, r)
		}
	}
	for _, arg := range []string{
		filepath.Join(root, "real", "missing.txt"),
		filepath.Join(root, "real", "missing-dir", "deep.txt"),
	} {
		if r := eval(arg); !r.Allowed {
			t.Fatalf("%s inside workspace denied: %+v", arg, r)
		}
	}
}

func TestCommandPolicyHomeShorthandResolvesAgainstHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	eval := func(arg string) policycontract.CommandPolicyEvaluation {
		return evaluateCommandPolicy(policycontract.CommandPolicyRequest{
			WorkspaceRoot: root, CWD: root, Argv: []string{"cat", arg}, Timeout: "30s",
		})
	}
	for _, arg := range []string{"~", "~/note.txt"} {
		if r := eval(arg); !r.Allowed {
			t.Fatalf("%q with HOME inside workspace denied: %+v", arg, r)
		}
	}
	if r := eval("~/../../escape.txt"); r.Allowed || !containsString(r.DenyReasons, "path_outside_workspace") {
		t.Fatalf("~/../../escape.txt not denied: %+v", r)
	}
}
