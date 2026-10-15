package issueopsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/qualitycli"
	core "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	authoritycontract "issueops/internal/contract/authority"
	inspectcontract "issueops/internal/contract/inspect"
	model "issueops/internal/contract/issueops"
	"issueops/internal/contract/mcpservice"
	contract "issueops/internal/contract/quality"
)

func TestModelResolveAgentsPrintsClaudeRoleAgentArguments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForContract(t, func() error {
		return runModel([]string{"resolve", "--host", "claude", "--role", "implement", "--agents", "--json", "--repo", repo})
	})
	var printed struct {
		AgentArgs []string `json:"agent_args"`
	}
	if err := json.Unmarshal([]byte(out), &printed); err != nil {
		t.Fatalf("resolve output is not JSON: %v\n%s", err, out)
	}
	if len(printed.AgentArgs) != 2 || printed.AgentArgs[0] != "--agents" || printed.AgentArgs[1] == "" {
		t.Fatalf("agent_args = %q, want [--agents <definitions>]", printed.AgentArgs)
	}
}

func TestInspectCommandHonoursCodexHomeEnvironment(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), "env-codex")
	t.Setenv("HOME", home)
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Setenv("CODEX_HOME", codexHome)
	out := captureStdoutForContract(t, func() error { return runInspect([]string{"--json", t.TempDir()}) })
	var info inspectcontract.InspectInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("inspect output is not JSON: %v\n%s", err, out)
	}
	if got, want := codexConfigPath(t, info), filepath.Join(codexHome, "config.toml"); got != want {
		t.Fatalf("codex config_path = %q, want %q", got, want)
	}
}

func codexConfigPath(t *testing.T, info inspectcontract.InspectInfo) string {
	t.Helper()
	for _, host := range info.Integration.Hosts {
		if host.Host == inspectcontract.HostCodex {
			return host.ConfigPath
		}
	}
	t.Fatal("inspect result has no codex host")
	return ""
}

func TestDefaultMCPTransportIsHTTPOnlyOnDarwinAndLinux(t *testing.T) {
	for goos, want := range map[string]string{"darwin": "http", "linux": "http", "windows": "stdio", "freebsd": "stdio"} {
		if got := defaultMCPTransport(goos); got != want {
			t.Errorf("defaultMCPTransport(%q) = %q, want %q", goos, got, want)
		}
	}
}

type recordingAuthorityIssuer struct {
	calls []authoritycontract.IssueRequest
}

func (i *recordingAuthorityIssuer) Issue(_ context.Context, req authoritycontract.IssueRequest) (authoritycontract.Receipt, error) {
	i.calls = append(i.calls, req)
	return authoritycontract.Receipt{AuthorityFile: "/state/authority/file", ExpiresAt: "2030-01-01T00:00:00Z"}, nil
}

func runAuthorizeWithFakes(t *testing.T, extra ...string) (*recordingAuthorityIssuer, string, error) {
	t.Helper()
	issuer := &recordingAuthorityIssuer{}
	var stdout bytes.Buffer
	args := append([]string{
		"--workspace-root", t.TempDir(), "--host", "codex", "--session-id", "s1",
		"--session-started-at", "start", "--session-executable", "/bin/codex",
	}, extra...)
	err := mcpAuthorizeCommand{
		issuer:  issuer,
		observe: func(int) ([]model.NativeProcessReceipt, error) { return nil, nil },
		pid:     func() int { return 1 },
		stdout:  &stdout,
	}.Run(args)
	return issuer, stdout.String(), err
}

func TestMCPAuthorizeRequiresPositiveSessionPID(t *testing.T) {
	issuer, _, err := runAuthorizeWithFakes(t, "--session-pid", "0")
	if err == nil || !strings.Contains(err.Error(), "--session-pid") || len(issuer.calls) != 0 {
		t.Fatalf("pid 0: err=%v issued=%d, want --session-pid usage error and no issue", err, len(issuer.calls))
	}
	issuer, out, err := runAuthorizeWithFakes(t, "--session-pid", "1")
	if err != nil || len(issuer.calls) != 1 || !strings.Contains(out, "authority_file: /state/authority/file") {
		t.Fatalf("pid 1: err=%v issued=%d out=%q", err, len(issuer.calls), out)
	}
}

func TestMCPAuthorizeRejectsCWDOutsideWorkspaceRoot(t *testing.T) {
	outside := t.TempDir()
	issuer, _, err := runAuthorizeWithFakes(t, "--session-pid", "1", "--cwd", outside)
	if err == nil || !strings.Contains(err.Error(), "mcp authorize: cwd is outside workspace_root") || len(issuer.calls) != 0 {
		t.Fatalf("outside cwd: err=%v issued=%d, want scope error and no issue", err, len(issuer.calls))
	}
}

func TestMCPRequestScopeResolvesIssueOpsRecordToExecutionWorkspaceRoot(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateRoot)
	repo := gitRepoForHTTPTest(t)
	recordWorktree := t.TempDir()
	executionRoot := t.TempDir()
	seeded := seedHTTPLeaseRecord(t, repo, recordWorktree, claimWiringActor(t))
	seeded.Execution.Workspace.Root = executionRoot
	if _, err := (core.CycleRecordStore{StateRoot: issueOpsStateRoot()}).Save(t.Context(), seeded); err != nil {
		t.Skipf("store refuses a record whose execution root differs from worktree_path: %v", err)
	}
	resolve := mcpcli.NewRequestScope(authorityoutbound.ScopeResolver{}, issueOpsMCPRecordRoots(issueOpsStateRoot()))
	scope, err := resolve(t.Context(), "issueops_execution", map[string]any{"id": seeded.ID, "cwd": executionRoot})
	want, evalErr := filepath.EvalSymlinks(executionRoot)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	if err != nil || scope.WorkspaceRoot != want {
		t.Fatalf("scope=%+v err=%v, want workspace root %q", scope, err, want)
	}
}

type failingMCPService struct{ err error }

func (s failingMCPService) Start(context.Context) (mcpservice.Status, error) {
	return mcpservice.Status{Status: mcpservice.StatusRunning}, s.err
}
func (s failingMCPService) Stop(context.Context) (mcpservice.Status, error) {
	return s.Start(context.Background())
}
func (s failingMCPService) Status(context.Context) (mcpservice.Status, error) {
	return s.Start(context.Background())
}

func TestMCPServiceCommandJSONModeReturnsServiceError(t *testing.T) {
	want := errors.New("supervisor refused")
	var stdout bytes.Buffer
	err := mcpServiceCommand{service: failingMCPService{err: want}, stdout: &stdout}.Run([]string{"start", "--json"})
	if !errors.Is(err, want) || !strings.Contains(stdout.String(), `"status"`) {
		t.Fatalf("err=%v stdout=%q, want service error after the JSON status", err, stdout.String())
	}
}

func TestQualityInspectCommandInspectsAbsoluteTargetNotDefaultRoot(t *testing.T) {
	defaultRoot, target := t.TempDir(), t.TempDir()
	for _, dir := range []string{defaultRoot, target} {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module qualityfixture\ngo 1.26\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	deps := newQualityDependencies(defaultRoot, t.TempDir())
	var result contract.InspectResult
	deps.PrintJSON = func(value any) error { result = value.(contract.InspectResult); return nil }
	t.Chdir(filepath.Dir(target))
	// A blocked gate is irrelevant here; only the inspected root is observed.
	_ = qualitycli.Run([]string{"inspect", "--repo", filepath.Base(target), "--json"}, deps)
	if !filepath.IsAbs(result.IssueOpsRoot) || filepath.Base(result.IssueOpsRoot) != filepath.Base(target) {
		t.Fatalf("inspected root %q, want the absolute path of %q (default root was %q)", result.IssueOpsRoot, target, defaultRoot)
	}
}

func TestStatusCommandReportsExplicitRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForContract(t, func() error { return runStatus([]string{"--repo", repo, "--json"}) })
	var printed struct {
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal([]byte(out), &printed); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, out)
	}
	if printed.Repo != repo {
		t.Fatalf("status repo = %q, want %q", printed.Repo, repo)
	}
}
