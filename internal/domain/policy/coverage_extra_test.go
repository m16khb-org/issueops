package policy

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
)

func TestCatalogApplyExtendsExistingAndNewCommandsAcrossCategories(t *testing.T) {
	catalog := BuiltinCatalog()
	catalog.Apply(policycontract.PolicyOverrides{
		AdditionalShellInterpreters:   []string{"nu"},
		AdditionalNetworkCommands:     []string{"aria2c"},
		AdditionalNetworkSubcommands:  map[string][]string{"git": {"bundle"}, "hg": {"pull"}},
		AdditionalWriteCommands:       []string{"truncate"},
		AdditionalWriteSubcommands:    map[string][]string{"go": {"clean"}, "cargo": {"build"}},
		AdditionalReadOnlyCommands:    []string{"echo"},
		AdditionalReadOnlySubcommands: map[string][]string{"git": {"blame"}, "cargo": {"metadata"}},
	})
	tests := []struct {
		name string
		argv []string
		want CommandClassification
	}{
		{"shell", []string{"nu"}, CommandClassification{ShellCommand: true}},
		{"network command", []string{"aria2c", "x"}, CommandClassification{UsesNetwork: true}},
		{"network subcommand on existing command", []string{"git", "bundle"}, CommandClassification{UsesNetwork: true}},
		{"network subcommand on new command", []string{"hg", "pull"}, CommandClassification{UsesNetwork: true}},
		{"write command", []string{"truncate", "f"}, CommandClassification{Writes: true}},
		{"write subcommand on existing command", []string{"go", "clean"}, CommandClassification{Writes: true}},
		{"write subcommand on new command", []string{"cargo", "build"}, CommandClassification{Writes: true}},
		{"read-only subcommand on existing command", []string{"git", "blame"}, CommandClassification{ReadOnlyAllowed: true}},
		{"read-only subcommand on new command", []string{"cargo", "metadata"}, CommandClassification{ReadOnlyAllowed: true}},
		{"builtin untouched", []string{"git", "status"}, CommandClassification{ReadOnlyAllowed: true}},
	}
	for _, tt := range tests {
		if got := catalog.Classify(tt.argv); got != tt.want {
			t.Errorf("%s: Classify(%v)=%+v want %+v", tt.name, tt.argv, got, tt.want)
		}
	}
}

func TestCatalogClassifyEdgeCases(t *testing.T) {
	catalog := BuiltinCatalog()
	if got := catalog.Classify(nil); got != (CommandClassification{}) {
		t.Fatalf("empty argv classification=%+v", got)
	}
	if got := catalog.Classify([]string{"git"}); got != (CommandClassification{}) {
		t.Fatalf("subcommand-less git must not match subcommand catalogs: %+v", got)
	}
	if got := catalog.Classify([]string{"/usr/bin/BASH", "-c", "x"}); !got.ShellCommand {
		t.Fatalf("path and case must be normalized for shell detection: %+v", got)
	}
	if got := catalog.Classify([]string{"git", "PUSH"}); !got.Writes || !got.UsesNetwork {
		t.Fatalf("subcommand must be matched case-insensitively: %+v", got)
	}
	if got := catalog.Classify([]string{"git", "frobnicate"}); got != (CommandClassification{}) {
		t.Fatalf("unknown git subcommand classification=%+v", got)
	}
}

func TestCatalogSnapshotIsSortedAndDetachedFromCatalog(t *testing.T) {
	catalog := BuiltinCatalog()
	snap := catalog.Snapshot()
	for name, list := range map[string][]string{
		"shell": snap.ShellInterpreters, "network": snap.NetworkCommands, "write": snap.WriteCommands,
		"readonly": snap.ReadOnlyCommands, "git write": snap.WriteSubcommands["git"],
		"git readonly": snap.ReadOnlySubcommands["git"], "git network": snap.NetworkSubcommands["git"],
	} {
		if len(list) == 0 || !slices.IsSorted(list) {
			t.Errorf("%s list not sorted/non-empty: %v", name, list)
		}
	}
	if !slices.Contains(snap.ShellInterpreters, "bash") || !slices.Contains(snap.WriteSubcommands["go"], "test") {
		t.Fatalf("snapshot missing builtin entries: %+v", snap)
	}
	catalog.Apply(policycontract.PolicyOverrides{AdditionalShellInterpreters: []string{"nu"}})
	if slices.Contains(snap.ShellInterpreters, "nu") || !slices.Contains(catalog.Snapshot().ShellInterpreters, "nu") {
		t.Fatal("snapshot must not follow later overrides, but a new snapshot must include them")
	}
	// Callers hand snapshot slices to JSON encoders; mutating them must not widen the live policy.
	for name := range snap.ReadOnlySubcommands {
		for i := range snap.ReadOnlySubcommands[name] {
			snap.ReadOnlySubcommands[name][i] = "push"
		}
	}
	if catalog.Classify([]string{"git", "push"}).ReadOnlyAllowed {
		t.Fatal("mutating a snapshot made git push read-only in the live catalog")
	}
}

func TestCommandTimeoutParsing(t *testing.T) {
	tests := []struct {
		raw    string
		want   time.Duration
		wantOK bool
	}{
		{"", DefaultCommandTimeout, false}, // omitted: displayed as default but invalid until the adapter supplies it
		{"90s", 90 * time.Second, true},
		{"1h30m", 90 * time.Minute, true},
		{"soon", 0, false},
		{"10", 0, false},
	}
	for _, tt := range tests {
		got, ok := CommandTimeout(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("CommandTimeout(%q)=(%v,%v) want (%v,%v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestCommandEnvironmentPassesOnlyAllowlistedNames(t *testing.T) {
	environ := []string{"PATH=/bin", "SECRET=x", "HOME=/h", "EMPTY=", "NOEQUALS", "PATHX=1", "A=b=c"}
	got := CommandEnvironment(environ, []string{" PATH ", "", "EMPTY", "NOEQUALS", "A"})
	want := []string{"PATH=/bin", "EMPTY=", "A=b=c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("env=%v want %v", got, want)
	}
	if got := CommandEnvironment(environ, nil); got == nil || len(got) != 0 {
		t.Fatalf("empty allowlist must yield empty non-nil env, got %#v", got)
	}
}

func TestValidateRemoteCreateInputsRejectsSecretsInEveryField(t *testing.T) {
	if err := ValidateRemoteCreateInputs("issue", "Title", "Body", []string{"bug"}, []string{"alice"}); err != nil {
		t.Fatalf("clean inputs rejected: %v", err)
	}
	if err := ValidateRemoteCreateInputs("issue", "", "", nil, nil); err != nil {
		t.Fatalf("empty inputs rejected: %v", err)
	}
	secret := "token=abc123"
	tests := []struct {
		field             string
		title, body       string
		labels, assignees []string
	}{
		{"title", secret, "b", nil, nil},
		{"body", "t", secret, nil, nil},
		{"label", "t", "b", []string{"ok", secret}, nil},
		{"assignee", "t", "b", []string{"ok"}, []string{"alice", secret}},
	}
	for _, tt := range tests {
		err := ValidateRemoteCreateInputs("pr", tt.title, tt.body, tt.labels, tt.assignees)
		want := "pr " + tt.field + " contains secret-like content"
		if err == nil || err.Error() != want {
			t.Errorf("field %s: err=%v want %q", tt.field, err, want)
		}
		if err != nil && strings.Contains(err.Error(), "abc123") {
			t.Errorf("error leaks secret: %v", err)
		}
	}
}

func TestCommandDecisionWorkspaceDenyBranches(t *testing.T) {
	base := policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"ls"}}
	okFacts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, Timeout: time.Second, TimeoutValid: true, ReadOnlyAllowed: true}
	if d := EvaluateCommandDecision(base, okFacts); !d.Allowed || len(d.DenyReasons) != 0 {
		t.Fatalf("baseline should be allowed: %+v", d)
	}
	facts := okFacts
	facts.RootDirectory, facts.CWDDirectory, facts.CWDWithinRoot = false, false, false
	got := EvaluateCommandDecision(base, facts).DenyReasons
	want := []string{"cwd_not_directory", "cwd_outside_workspace", "workspace_root_not_directory"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deny=%v want %v", got, want)
	}
	facts = okFacts
	facts.PathOutsideRoot = true
	if got := EvaluateCommandDecision(base, facts).DenyReasons; !reflect.DeepEqual(got, []string{"path_outside_workspace"}) {
		t.Fatalf("deny=%v", got)
	}
	facts = okFacts
	facts.PathOutsideRoot = true
	empty := base
	empty.Argv = nil
	if got := EvaluateCommandDecision(empty, facts).DenyReasons; !reflect.DeepEqual(got, []string{"argv_required"}) {
		t.Fatalf("argv-dependent rules must not run without argv: %v", got)
	}
}

func TestCommandDecisionTimeoutBoundaries(t *testing.T) {
	req := policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"ls"}}
	facts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, ReadOnlyAllowed: true, TimeoutValid: true}
	tests := []struct {
		timeout time.Duration
		valid   bool
		want    []string
	}{
		{15 * time.Minute, true, nil},
		{15*time.Minute + time.Nanosecond, true, []string{"timeout_exceeds_15m"}},
		{0, true, []string{"invalid_timeout"}},
		{-time.Second, true, []string{"invalid_timeout"}},
		{time.Second, false, []string{"invalid_timeout"}},
		{time.Hour, false, []string{"invalid_timeout"}},
	}
	for _, tt := range tests {
		f := facts
		f.Timeout, f.TimeoutValid = tt.timeout, tt.valid
		got := EvaluateCommandDecision(req, f).DenyReasons
		if !slices.Equal(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
			t.Errorf("timeout=%v valid=%v deny=%v want %v", tt.timeout, tt.valid, got, tt.want)
		}
	}
}

func TestCommandDecisionShellPolicy(t *testing.T) {
	facts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, Timeout: time.Second, TimeoutValid: true, ShellCommand: true, ReadOnlyAllowed: true}
	req := policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"bash"}}

	d := EvaluateCommandDecision(req, facts)
	if d.Allowed || !slices.Equal(d.DenyReasons, []string{"shell_interpreter_not_allowed"}) || len(d.Warnings) != 0 {
		t.Fatalf("shell not allowed: %+v", d)
	}
	req.ShellAllowed = true
	req.ShellReason = " \t\n"
	d = EvaluateCommandDecision(req, facts)
	if d.Allowed || !slices.Equal(d.DenyReasons, []string{"shell_reason_required"}) || len(d.Warnings) != 0 {
		t.Fatalf("blank reason: %+v", d)
	}
	req.ShellReason = "bootstrap"
	d = EvaluateCommandDecision(req, facts)
	if !d.Allowed || !slices.Equal(d.Warnings, []string{"shell_interpreter_exception"}) {
		t.Fatalf("justified shell must be allowed with warning: %+v", d)
	}
}

func TestCommandDecisionWriteAllowedSkipsReadOnlyAllowlistButNotOtherDenies(t *testing.T) {
	facts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, Timeout: time.Second, TimeoutValid: true, Writes: true, UsesNetwork: true}
	req := policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"git", "push"}, WriteAllowed: true}
	if got := EvaluateCommandDecision(req, facts).DenyReasons; !slices.Equal(got, []string{"network_not_allowed"}) {
		t.Fatalf("write-allowed deny=%v", got)
	}
	req.NetworkAllowed = true
	if d := EvaluateCommandDecision(req, facts); !d.Allowed {
		t.Fatalf("fully allowed command denied: %+v", d)
	}
	req.WriteAllowed = false
	got := EvaluateCommandDecision(req, facts).DenyReasons
	if !slices.Equal(got, []string{"command_not_in_read_only_allowlist", "write_not_allowed"}) {
		t.Fatalf("deny=%v", got)
	}
}

func TestCommandDecisionDeduplicatesAndSortsWarnings(t *testing.T) {
	facts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, Timeout: time.Second, TimeoutValid: true, ReadOnlyAllowed: true,
		Warnings: []string{"z", "a", "a", ""}}
	d := EvaluateCommandDecision(policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"ls"}}, facts)
	if !slices.Equal(d.Warnings, []string{"a", "z"}) {
		t.Fatalf("warnings=%v", d.Warnings)
	}
}

func TestCommandDecisionSecretAndEnvAllowlistArguments(t *testing.T) {
	facts := CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true, Timeout: time.Second, TimeoutValid: true, ReadOnlyAllowed: true}
	req := policycontract.CommandPolicyRequest{WorkspaceRoot: "/r", CWD: "/r", Argv: []string{"cat", ".env"}, EnvAllowlist: []string{"PATH", "1BAD", "BAD NAME"}}
	got := EvaluateCommandDecision(req, facts).DenyReasons
	if !slices.Equal(got, []string{"invalid_env_allowlist_name", "secret_like_argument"}) {
		t.Fatalf("deny=%v", got)
	}
	req.Argv, req.EnvAllowlist = []string{"cat", "README.md"}, []string{"PATH", "_X1"}
	if d := EvaluateCommandDecision(req, facts); !d.Allowed {
		t.Fatalf("clean command denied: %+v", d)
	}
}

func TestPathCandidatesClassification(t *testing.T) {
	tests := []struct {
		arg  string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"  /etc/passwd  ", []string{"/etc/passwd"}},
		{"~", []string{"~"}},
		{"~/x", []string{"~/x"}},
		{"..", []string{".."}},
		{".", []string{"."}},
		{"./a", []string{"./a"}},
		{"a/b", []string{"a/b"}},
		{"plain", nil},
		{"~user", nil},
		{"-v", nil},
		{"--flag", nil},
		{"--out", nil},
		{"--out=plain", nil},
		{"--out=../x", []string{"../x"}},
		{"--out=https://h/x", nil},
		{"-o=/tmp/x", []string{"/tmp/x"}},
		{"=/tmp/x", []string{"=/tmp/x"}},
		{"user@host:path/x", nil},
		{"user@host", nil},
		{"HTTP://X/y", nil},
	}
	for _, tt := range tests {
		if got := PathCandidates(tt.arg); !slices.Equal(got, tt.want) {
			t.Errorf("PathCandidates(%q)=%v want %v", tt.arg, got, tt.want)
		}
	}
}

func TestBoundedDiagnosticLimitNormalization(t *testing.T) {
	long := strings.Repeat("x", maxBoundedDiagnosticBytes+100)
	for _, limit := range []int{0, -5, maxBoundedDiagnosticBytes + 1} {
		got := BoundedDiagnostic(long, limit)
		if want := strings.Repeat("x", maxBoundedDiagnosticBytes) + "...[truncated]"; got != want {
			t.Errorf("limit=%d len=%d want capped at %d", limit, len(got), maxBoundedDiagnosticBytes)
		}
	}
	if got := BoundedDiagnostic("  hi \n", 10); got != "hi" {
		t.Fatalf("not trimmed: %q", got)
	}
	if got := BoundedDiagnostic("see https://example.com/x?y=1 now", 100); got != "see [REDACTED_URL] now" {
		t.Fatalf("url not redacted: %q", got)
	}
}
