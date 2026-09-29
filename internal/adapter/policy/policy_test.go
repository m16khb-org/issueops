package policy

import (
	policydomain "issueops/internal/contract/policy"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandPolicyAllowsReadOnlyInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	result := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if !result.OK || !result.Allowed {
		t.Fatalf("policy denied read-only command: %+v", result)
	}
}

func TestCommandPolicyDeniesOutsideWorkspaceAndShell(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideResult := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           outside,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if outsideResult.Allowed || !containsString(outsideResult.DenyReasons, "cwd_outside_workspace") {
		t.Fatalf("outside cwd not denied: %+v", outsideResult)
	}

	shellResult := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"sh", "-c", "echo ok"},
		Timeout:       "30s",
	})
	if shellResult.Allowed || !containsString(shellResult.DenyReasons, "shell_interpreter_not_allowed") {
		t.Fatalf("shell command not denied: %+v", shellResult)
	}
}

func TestCommandPolicyDeniesPathArgsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "note.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", outside)
	link := filepath.Join(inside, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		argv []string
	}{
		{name: "relative parent escape", argv: []string{"cat", filepath.Join("..", "..", filepath.Base(outside), "note.txt")}},
		{name: "absolute outside path", argv: []string{"cat", outsideFile}},
		{name: "flag value outside path", argv: []string{"sed", "--file=" + outsideFile}},
		{name: "symlink escape path", argv: []string{"cat", filepath.Join(link, "note.txt")}},
		{name: "home shorthand escape", argv: []string{"cat", "~/note.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{
				WorkspaceRoot: root,
				CWD:           inside,
				Argv:          tc.argv,
				Timeout:       "30s",
			})
			if result.Allowed || !containsString(result.DenyReasons, "path_outside_workspace") {
				t.Fatalf("outside path arg not denied: %+v", result)
			}
		})
	}

	insideResult := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           inside,
		Argv:          []string{"cat", filepath.Join("..", "inside", "local.txt")},
		Timeout:       "30s",
	})
	if !insideResult.Allowed {
		t.Fatalf("inside path arg should be allowed: %+v", insideResult)
	}
}

func TestCommandFakeRunDoesNotExecute(t *testing.T) {
	root := t.TempDir()
	result := FakeRunCommand(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"touch", "marker"},
		Timeout:       "30s",
		WriteAllowed:  true,
	})
	if !result.OK || result.Executed || !result.Policy.Allowed {
		t.Fatalf("unexpected fake-run result: %+v", result)
	}
	if existsForTest(filepath.Join(root, "marker")) {
		t.Fatalf("fake-run created marker; command executed")
	}
}

func TestRunReadOnlyCommandExecutesAllowedArgvOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := RunReadOnlyCommand(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"cat", "note.txt"},
		Timeout:       "30s",
	})
	if !result.OK || !result.Executed || result.ExitCode != 0 || result.Stdout != "hello\n" {
		t.Fatalf("read-only run failed: %+v", result)
	}
	denied := RunReadOnlyCommand(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"touch", "marker"},
		Timeout:       "30s",
		WriteAllowed:  true,
	})
	if denied.OK || denied.Executed || denied.ExitCode != 3 || !containsString(denied.Policy.DenyReasons, "write_not_allowed") {
		t.Fatalf("write command was not denied by read-only runner: %+v", denied)
	}
	if existsForTest(filepath.Join(root, "marker")) {
		t.Fatalf("read-only run created marker")
	}
}

func TestRunReadOnlyCommandUsesEmptyEnvUnlessAllowlisted(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ENV_LEAK_TEST", "leaked")
	argv := []string{"awk", `BEGIN { print ENVIRON["ISSUEOPS_ENV_LEAK_TEST"] }`}

	defaultEnv := RunReadOnlyCommand(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          argv,
		Timeout:       "30s",
	})
	if !defaultEnv.OK || defaultEnv.Stdout != "\n" {
		t.Fatalf("default read-only env should not inherit parent env: %+v", defaultEnv)
	}

	allowlisted := RunReadOnlyCommand(policydomain.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          argv,
		Timeout:       "30s",
		EnvAllowlist:  []string{"ISSUEOPS_ENV_LEAK_TEST"},
	})
	if !allowlisted.OK || allowlisted.Stdout != "leaked\n" {
		t.Fatalf("allowlisted env was not exposed: %+v", allowlisted)
	}
}

func TestPolicyTimeoutContract(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ input, duration, deny string }{
		{"30s", "30s", ""}, {"15m", "15m0s", ""}, {"15m1s", "15m1s", "timeout_exceeds_15m"},
		{"", "30s", "invalid_timeout"}, {"bad", "0s", "invalid_timeout"}, {"0s", "0s", "invalid_timeout"}, {"-1s", "-1s", "invalid_timeout"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := EvaluateCommandPolicy(policydomain.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"git", "status"}, Timeout: tc.input})
			if got.Timeout != tc.duration || got.Allowed != (tc.deny == "") || (tc.deny != "" && !containsString(got.DenyReasons, tc.deny)) {
				t.Fatalf("timeout %q: %+v", tc.input, got)
			}
		})
	}
}
