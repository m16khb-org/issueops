package orca

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/port"
)

func okEnvelope(result string) CommandOutput {
	return CommandOutput{Stdout: []byte(`{"ok":true,"result":` + result + `,"_meta":{"runtimeId":"runtime-1"}}`)}
}

func failEnvelope(code, message string) CommandOutput {
	return CommandOutput{Stdout: []byte(`{"ok":false,"error":{"code":"` + code + `","message":"` + message + `"},"_meta":{"runtimeId":"runtime-1"}}`)}
}

func requireOrcaCode(t *testing.T, err error, code string) {
	t.Helper()
	orcaErr, ok := errors.AsType[*port.OrcaError](err)
	if !ok || orcaErr.Code != code {
		t.Fatalf("error = %#v (%v), want OrcaError code %q", orcaErr, err, code)
	}
}

func requireOrcaError(t *testing.T, err error, code string) *port.OrcaError {
	t.Helper()
	requireOrcaCode(t, err, code)
	orcaErr, _ := errors.AsType[*port.OrcaError](err)
	return orcaErr
}

func TestProbeStopsAtTheFirstFailingPrerequisite(t *testing.T) {
	setup := func(runner *fakeRunner) {
		runner.lookPaths["orca"] = "/usr/local/bin/orca"
		runner.lookPaths["codex"] = "/usr/local/bin/codex"
		runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
		runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
		addCompleteProbeLeafHelp(runner)
	}
	tests := []struct {
		name      string
		request   port.OrcaProbeRequest
		arrange   func(*fakeRunner)
		wantCode  string
		available bool
		wantCalls int // -1 = not asserted
	}{
		{name: "unsupported provider", request: port.OrcaProbeRequest{Repo: "/repo", Provider: "bitbucket"}, wantCode: "unsupported_provider", wantCalls: 0},
		{name: "unsupported agent", request: port.OrcaProbeRequest{Repo: "/repo", Agent: "gemini"}, wantCode: "unsupported_agent", wantCalls: 0},
		{name: "orca missing from PATH", request: port.OrcaProbeRequest{Repo: "/repo"}, wantCode: "orca_not_found", wantCalls: 0},
		{
			name: "status cannot be decoded", request: port.OrcaProbeRequest{Repo: "/repo"}, available: true, wantCode: "status_failed", wantCalls: 1,
			arrange: func(r *fakeRunner) {
				r.lookPaths["orca"] = "/usr/local/bin/orca"
				r.responses["orca status --json"] = CommandOutput{Stdout: []byte(`{"ok":`)}
			},
		},
		{
			name: "repo cannot be resolved", request: port.OrcaProbeRequest{Repo: "/repo"}, available: true, wantCode: "repo_unresolved", wantCalls: 2,
			arrange: func(r *fakeRunner) {
				setup(r)
				r.responses["orca repo show --repo path:/repo --json"] = failEnvelope("selector_not_found", "no such repo")
			},
		},
		{
			name: "repo path differs from the requested one", request: port.OrcaProbeRequest{Repo: "/other"}, available: true, wantCode: "repo_path_mismatch", wantCalls: 2,
			arrange: func(r *fakeRunner) {
				setup(r)
				r.responses["orca repo show --repo path:/other --json"] = fixtureOutput(t, "repo_show.json")
			},
		},
		{
			name: "a required leaf flag is absent", request: port.OrcaProbeRequest{Repo: "/repo"}, available: true, wantCode: "capability_missing", wantCalls: -1,
			arrange: func(r *fakeRunner) {
				setup(r)
				r.responses["orca worktree list --help"] = CommandOutput{Stdout: []byte("--repo --json")}
			},
		},
		{
			name: "a help command fails", request: port.OrcaProbeRequest{Repo: "/repo"}, available: true, wantCode: "capability_missing", wantCalls: -1,
			arrange: func(r *fakeRunner) {
				setup(r)
				r.errors["orca terminal list --help"] = errors.New("exec failed")
			},
		},
		{
			name: "host agent binary missing", request: port.OrcaProbeRequest{Repo: "/repo", Agent: "claude"}, available: true, wantCode: "agent_not_found", wantCalls: -1,
			arrange: setup,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			if tt.arrange != nil {
				tt.arrange(runner)
			}
			got, err := NewClient(runner).Probe(context.Background(), tt.request)
			if err != nil {
				t.Fatal(err)
			}
			if got.Ready || got.Code != tt.wantCode || got.Available != tt.available {
				t.Fatalf("probe = %#v, want code=%s available=%t", got, tt.wantCode, tt.available)
			}
			if tt.wantCalls >= 0 && len(runner.calls) != tt.wantCalls {
				t.Fatalf("probe made %d calls, want %d: %#v", len(runner.calls), tt.wantCalls, runner.calls)
			}
		})
	}
}

func TestProbeEchoesNormalizedRequestIdentity(t *testing.T) {
	got, err := NewClient(newFakeRunner(t)).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Provider: " BitBucket "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "bitbucket" || got.Agent != "codex" {
		t.Fatalf("probe identity = provider %q agent %q, want lowercased provider and the codex default", got.Provider, got.Agent)
	}
}

func TestProbeSurfacesRepoAndStatusDetailForTheOperator(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/usr/local/bin/orca"
		runner.responses["orca status --json"] = failEnvelope("runtime_unreachable", "app is not running")
		got, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Code != "status_failed" || !strings.Contains(got.Detail, "runtime_unreachable") || !strings.Contains(got.Detail, "app is not running") {
			t.Fatalf("status failure detail = %#v", got)
		}
	})
	t.Run("repo", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/usr/local/bin/orca"
		runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
		runner.responses["orca repo show --repo path:/repo --json"] = failEnvelope("selector_not_found", "no such repo")
		got, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Code != "repo_unresolved" || !strings.Contains(got.Detail, "selector_not_found") || !strings.Contains(got.Detail, "no such repo") {
			t.Fatalf("repo failure detail = %#v", got)
		}
	})
}

func TestDeliveryIdentityResolution(t *testing.T) {
	const statusNoVersion = `{"runtime":{"state":"ready","reachable":true,"runtimeId":"runtime-live"},"target":{"serverId":"server-1","kind":"local"}}`
	t.Run("status failure is propagated", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/opt/orca"
		runner.responses["orca status --json"] = failEnvelope("runtime_unreachable", "down")
		_, err := NewClient(runner).DeliveryIdentity(context.Background())
		requireOrcaCode(t, err, "runtime_unreachable")
	})
	t.Run("missing executable", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca status --json"] = okEnvelope(statusNoVersion)
		_, err := NewClient(runner).DeliveryIdentity(context.Background())
		requireOrcaCode(t, err, "orca_identity_unresolved")
		if len(runner.calls) != 1 {
			t.Fatalf("version was queried without an executable: %#v", runner.calls)
		}
	})
	t.Run("version falls back to the installed binary and prefers the server id", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/opt/orca"
		runner.responses["orca status --json"] = okEnvelope(statusNoVersion)
		runner.responses["orca --version"] = CommandOutput{Stdout: []byte("orca 2.5.1\n")}
		identity, err := NewClient(runner).DeliveryIdentity(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if identity.Version != "2.5.1" || identity.TargetIdentity != "server-1" || identity.RuntimeID != "runtime-live" {
			t.Fatalf("identity = %+v", identity)
		}
	})
	t.Run("version command failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/opt/orca"
		runner.responses["orca status --json"] = okEnvelope(statusNoVersion)
		runner.errors["orca --version"] = errors.New("boom")
		_, err := NewClient(runner).DeliveryIdentity(context.Background())
		if orcaErr := requireOrcaError(t, err, "orca_identity_unresolved"); !strings.Contains(orcaErr.Detail, "version") {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
	})
	t.Run("blank version output", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/opt/orca"
		runner.responses["orca status --json"] = okEnvelope(statusNoVersion)
		runner.responses["orca --version"] = CommandOutput{Stdout: []byte("orca\n")}
		_, err := NewClient(runner).DeliveryIdentity(context.Background())
		requireOrcaCode(t, err, "orca_identity_unresolved")
	})
	t.Run("missing target identity", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.lookPaths["orca"] = "/opt/orca"
		runner.responses["orca status --json"] = okEnvelope(`{"runtime":{"runtimeId":"runtime-live","appVersion":"1.0.0"}}`)
		_, err := NewClient(runner).DeliveryIdentity(context.Background())
		if orcaErr := requireOrcaError(t, err, "orca_identity_unresolved"); !strings.Contains(orcaErr.Detail, "server identity") {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
	})
}

func TestResolveRepoRejectsRelativePathsAndPropagatesLookupErrors(t *testing.T) {
	runner := newFakeRunner(t)
	_, err := NewClient(runner).ResolveRepo(context.Background(), "relative/repo")
	requireOrcaCode(t, err, "repo_identity_invalid")
	if len(runner.calls) != 0 {
		t.Fatalf("relative repo reached Orca: %#v", runner.calls)
	}

	runner.responses["orca repo show --repo path:/abs/repo --json"] = failEnvelope("selector_not_found", "unknown repo")
	_, err = NewClient(runner).ResolveRepo(context.Background(), "/abs/repo")
	requireOrcaCode(t, err, "selector_not_found")

	runner.responses["orca repo show --repo path:/abs/repo --json"] = okEnvelope(`{"repo":{"id":"","path":"/abs/repo"}}`)
	_, err = NewClient(runner).ResolveRepo(context.Background(), "/abs/repo")
	if orcaErr := requireOrcaError(t, err, "repo_identity_mismatch"); !orcaErr.Invoked {
		t.Fatal("an unidentified repo registration was observed from Orca and must count as invoked")
	}
}

func TestListWorktreesPropagatesOrcaFailure(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca worktree list --repo path:/repo --limit 512 --json"] = failEnvelope("selector_not_found", "gone")
	_, err := NewClient(runner).ListWorktrees(context.Background(), "/repo")
	requireOrcaCode(t, err, "selector_not_found")
}

func TestCreateWorktreeRejectsInvalidRequestsBeforeInvokingOrca(t *testing.T) {
	t.Run("unsupported provider", func(t *testing.T) {
		runner := newFakeRunner(t)
		_, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{Repo: "/repo", Name: "n", Provider: "bitbucket"})
		if orcaErr := requireOrcaError(t, err, "unsupported_provider"); orcaErr.Detail != "bitbucket" {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("github requires a positive issue", func(t *testing.T) {
		for _, issue := range []int{0, -3} {
			runner := newFakeRunner(t)
			_, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{Repo: "/repo", Name: "n", Provider: "github", Issue: issue})
			requireOrcaCode(t, err, "github_issue_required")
			if len(runner.calls) != 0 {
				t.Fatalf("issue %d reached Orca: %#v", issue, runner.calls)
			}
		}
	})
}

func TestCreateWorktreeReturnsOrcaFailureWithoutBranchRepair(t *testing.T) {
	runner := newFakeRunner(t)
	create := "orca worktree create --repo path:/repo --name n --base-branch main --no-parent --setup skip --comment c --issue 5 --json"
	runner.responses[create] = failEnvelope("runtime_error", "git exploded")
	_, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{Repo: "/repo", Name: "n", BaseBranch: "main", Provider: "github", Issue: 5, Comment: "c"})
	requireOrcaCode(t, err, "runtime_error")
	if len(runner.calls) != 1 {
		t.Fatalf("a failed create ran follow-up commands: %#v", runner.calls)
	}
}

func TestCreateWorktreeBranchCanonicalization(t *testing.T) {
	const create = "orca worktree create --repo path:/repo --name n --base-branch main --no-parent --setup skip --comment c --issue 5 --json"
	request := port.OrcaCreateWorktreeRequest{Repo: "/repo", Name: "n", BaseBranch: "main", Provider: "github", Issue: 5, Comment: "c"}
	worktree := func(branch, path string) CommandOutput {
		return okEnvelope(`{"worktree":{"id":"wt-1","repoId":"repo-1","path":"` + path + `","branch":"` + branch + `"}}`)
	}
	t.Run("branch already equals the request", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = worktree("n", "/repo.worktrees/n")
		got, err := NewClient(runner).CreateWorktree(context.Background(), request)
		if err != nil || got.Branch != "n" || len(runner.calls) != 1 {
			t.Fatalf("worktree=%#v err=%v calls=%#v", got, err, runner.calls)
		}
	})
	t.Run("unrelated branch is a mismatch and runs no git", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = worktree("someone-else", "/repo.worktrees/n")
		_, err := NewClient(runner).CreateWorktree(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "worktree_branch_mismatch"); !orcaErr.Invoked {
			t.Fatal("the worktree already exists, so the mismatch must report invoked")
		}
		if len(runner.calls) != 1 {
			t.Fatalf("mismatch ran git: %#v", runner.calls)
		}
	})
	t.Run("namespaced branch with a relative worktree path is a mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = worktree("alice/n", "relative/wt")
		_, err := NewClient(runner).CreateWorktree(context.Background(), request)
		requireOrcaCode(t, err, "worktree_branch_mismatch")
		if len(runner.calls) != 1 {
			t.Fatalf("relative worktree path ran git: %#v", runner.calls)
		}
	})
	t.Run("rename failure stops before setting the upstream", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = worktree("alice/n", "/repo.worktrees/n")
		runner.errors["git branch -m n"] = errors.New("branch exists")
		_, err := NewClient(runner).CreateWorktree(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "canonicalize Orca worktree branch") || !strings.Contains(err.Error(), "branch exists") {
			t.Fatalf("rename failure = %v", err)
		}
		if len(runner.calls) != 2 {
			t.Fatalf("upstream was set after a failed rename: %#v", runner.calls)
		}
	})
	t.Run("upstream failure is reported", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = worktree("alice/n", "/repo.worktrees/n")
		runner.responses["git branch -m n"] = CommandOutput{}
		runner.errors["git branch --set-upstream-to main n"] = errors.New("no such upstream")
		got, err := NewClient(runner).CreateWorktree(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "no such upstream") {
			t.Fatalf("upstream failure = %v", err)
		}
		if got.Branch != "alice/n" {
			t.Fatalf("branch was reported canonical despite the failure: %q", got.Branch)
		}
	})
}

func TestExactNumericBranchSuffix(t *testing.T) {
	for _, tt := range []struct {
		observed, requested string
		want                bool
	}{
		{"2609-fix-2", "2609-fix", true},
		{"2609-fix-17", "2609-fix", true},
		{" 2609-fix-2 ", " 2609-fix ", true},
		{"2609-fix-1", "2609-fix", false},
		{"2609-fix-0", "2609-fix", false},
		{"2609-fix-02", "2609-fix", false},
		{"2609-fix-", "2609-fix", false},
		{"2609-fix-x", "2609-fix", false},
		{"2609-fix", "2609-fix", false},
		{"other-2", "2609-fix", false},
	} {
		if got := exactNumericBranchSuffix(tt.observed, tt.requested); got != tt.want {
			t.Errorf("exactNumericBranchSuffix(%q, %q) = %t, want %t", tt.observed, tt.requested, got, tt.want)
		}
	}
}

func TestRemoveWorktreeForceFlag(t *testing.T) {
	for _, tt := range []struct {
		force   bool
		command string
	}{
		{false, "orca worktree rm --worktree id:wt-1 --json"},
		{true, "orca worktree rm --worktree id:wt-1 --force --json"},
	} {
		runner := newFakeRunner(t)
		runner.responses[tt.command] = okEnvelope(`{}`)
		if err := NewClient(runner).RemoveWorktree(context.Background(), "wt-1", tt.force); err != nil {
			t.Fatalf("force=%t: %v", tt.force, err)
		}
	}
	runner := newFakeRunner(t)
	runner.responses["orca worktree rm --worktree id:wt-1 --json"] = failEnvelope("selector_not_found", "gone")
	requireOrcaCode(t, NewClient(runner).RemoveWorktree(context.Background(), "wt-1", false), "selector_not_found")
}

func TestCloseTerminalValidatesHandleAndPropagatesOrcaFailures(t *testing.T) {
	runner := newFakeRunner(t)
	for _, handle := range []string{"", "term-a", "pty_1", "term_a b"} {
		err := NewClient(runner).CloseTerminal(context.Background(), handle)
		if err == nil || !strings.Contains(err.Error(), "invalid Orca terminal handle") {
			t.Fatalf("handle %q error = %v", handle, err)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid handles reached Orca: %#v", runner.calls)
	}

	runner.responses["orca terminal close --terminal term_a --json"] = failEnvelope("runtime_error", "something else")
	requireOrcaCode(t, NewClient(runner).CloseTerminal(context.Background(), "term_a"), "runtime_error")

	runner.responses["orca terminal close --terminal term_a --json"] = failEnvelope("selector_not_found", "tab_not_found")
	requireOrcaCode(t, NewClient(runner).CloseTerminal(context.Background(), "term_a"), "selector_not_found")
}

func TestShowTerminalInventoryRequiresConcreteHandle(t *testing.T) {
	runner := newFakeRunner(t)
	_, err := NewClient(runner).showTerminalInventory(context.Background(), " not-a-handle ")
	requireOrcaCode(t, err, "terminal_handle_invalid")
	if len(runner.calls) != 0 {
		t.Fatalf("invalid handle reached Orca: %#v", runner.calls)
	}

	runner.responses["orca terminal show --terminal term_a --json"] = failEnvelope("selector_not_found", "gone")
	_, err = NewClient(runner).showTerminalInventory(context.Background(), "term_a")
	requireOrcaCode(t, err, "selector_not_found")
}

func TestListTerminalsRejectsIncompleteOrUnboundedVisualLayouts(t *testing.T) {
	tab := func(id, leaf, title string) string {
		return `{"tabId":"` + id + `","activeLeafId":"` + leaf + `","title":"` + title + `"}`
	}
	layout := func(tabs ...string) string { return `{"root":{"tabs":[` + strings.Join(tabs, ",") + `]}}` }
	tests := []struct {
		name, layouts, want string
	}{
		{"conflicting titles for one tab", layout(tab("t", "l", "a"), tab("t", "l", "b")), "conflicting titles"},
		{"padded identity", layout(tab(" t", "l", "a")), "not canonical"},
		{"oversized title", layout(tab("t", "l", strings.Repeat("x", 4097))), "not canonical"},
		{"too many layouts", "[" + strings.Repeat(layout()+",", port.OrcaMaxBaselineIDs) + layout() + "]", "layout inventory exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			layouts := tt.layouts
			if !strings.HasPrefix(layouts, "[") {
				layouts = "[" + layouts + "]"
			}
			runner.responses["orca terminal list --limit 512 --json"] = okEnvelope(`{"terminals":[],"visualLayouts":` + layouts + `,"totalCount":0,"truncated":false}`)
			_, err := NewClient(runner).ListAllTerminals(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestStableVisualTabTitlesSkipsIncompleteTabsAndKeysByTabAndLeaf(t *testing.T) {
	var layout visualLayoutPayload
	layout.Root.Tabs = append(layout.Root.Tabs,
		struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		}{TabID: "t1", ActiveLeafID: "l1", Title: "marker"},
		struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		}{TabID: "t2", ActiveLeafID: "", Title: "no leaf"},
		struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		}{TabID: "t1", ActiveLeafID: "l1", Title: "marker"},
	)
	titles, err := stableVisualTabTitles([]visualLayoutPayload{layout})
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[visualTabKey("t1", "l1")] != "marker" {
		t.Fatalf("titles = %#v", titles)
	}
	if visualTabKey("t1", " ") != "" || visualTabKey("", "l1") != "" {
		t.Fatal("a tab key must not exist for an incomplete tab/leaf pair")
	}
}

func TestCreateTerminalPreInvocationAndResponseFailures(t *testing.T) {
	const help = "orca terminal create --help"
	t.Run("unsupported agent profile", func(t *testing.T) {
		runner := newFakeRunner(t)
		_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "wt", Agent: "gemini", Model: "m"})
		if orcaErr := requireOrcaError(t, err, "unsupported_agent_profile"); orcaErr.Detail != "gemini" {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("help probe failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.errors[help] = errors.New("exec failed")
		_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "wt", Agent: "claude", Model: "m"})
		requireOrcaCode(t, err, "terminal_create_capability_unavailable")
	})
	t.Run("create failure is propagated", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[help] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
		runner.responses["orca terminal create --worktree id:wt --command claude --model 'm' --dangerously-skip-permissions --json"] = failEnvelope("runtime_error", "no pty")
		_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "wt", Agent: "claude", Model: "m"})
		requireOrcaCode(t, err, "runtime_error")
	})
}

func TestSendTerminalPromptRejectsInvalidInputsBeforeInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	for name, tt := range map[string]struct{ handle, prompt string }{
		"abstract handle": {"term-1", "hello"},
		"blank prompt":    {"term_1", "  \n"},
		"escape in text":  {"term_1", "hello\x1b[31m"},
		"nul in text":     {"term_1", "hel\x00lo"},
	} {
		_, err := NewClient(runner).SendTerminalPrompt(context.Background(), tt.handle, tt.prompt, "")
		if orcaErr := requireOrcaError(t, err, "terminal_prompt_invalid"); orcaErr.Invoked {
			t.Fatalf("%s: validation failures never reach Orca", name)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestSendTerminalPromptOrcaOutcomes(t *testing.T) {
	const command = "orca terminal send --terminal term_1 --text \x1b[200~hi\x1b[201~ --enter --json"
	t.Run("transport failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = failEnvelope("terminal_not_writable", "closed")
		_, err := NewClient(runner).SendTerminalPrompt(context.Background(), "term_1", "hi", "")
		requireOrcaCode(t, err, "terminal_not_writable")
	})
	t.Run("not accepted", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"send":{"accepted":false}}`)
		_, err := NewClient(runner).SendTerminalPrompt(context.Background(), "term_1", "hi", "")
		if orcaErr := requireOrcaError(t, err, "terminal_prompt_rejected"); !orcaErr.Invoked {
			t.Fatal("a rejected prompt was sent to Orca and must report invoked")
		}
	})
}

func TestListTasksInventoryRejectsPaddedRunIdentityBeforeInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	client := NewClient(runner)
	_, err := client.listRunTasksInventory(context.Background(), " run_a", "--brief")
	requireOrcaCode(t, err, "run_identity_invalid")
	_, err = client.listRunGatesInventory(context.Background(), "")
	requireOrcaCode(t, err, "run_identity_invalid")
	if len(runner.calls) != 0 {
		t.Fatalf("invalid Run identities reached Orca: %#v", runner.calls)
	}
}

func TestListRunGatesInventoryFailsClosed(t *testing.T) {
	const command = "orca orchestration gate-list --run run_a --json"
	t.Run("orca error", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = failEnvelope("runtime_error", "boom")
		_, err := NewClient(runner).listRunGatesInventory(context.Background(), "run_a")
		requireOrcaCode(t, err, "runtime_error")
	})
	t.Run("gate row without identity", func(t *testing.T) {
		for _, row := range []string{
			`{"id":"","task_id":"t","status":"s"}`,
			`{"id":"g","task_id":" ","status":"s"}`,
			`{"id":"g","task_id":"t","status":""}`,
		} {
			runner := newFakeRunner(t)
			runner.responses[command] = okEnvelope(`{"runId":"run_a","gates":[` + row + `],"count":1}`)
			_, err := NewClient(runner).listRunGatesInventory(context.Background(), "run_a")
			if err == nil || !strings.Contains(err.Error(), "gate row identity is incomplete") {
				t.Fatalf("row %s error = %v", row, err)
			}
		}
	})
	t.Run("complete rows carry the runtime", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"runId":"run_a","gates":[{"id":"g","task_id":"t","status":"pending"}],"count":1}`)
		got, err := NewClient(runner).listRunGatesInventory(context.Background(), "run_a")
		if err != nil || got.RuntimeID != "runtime-1" || len(got.Rows) != 1 || got.Rows[0] != (port.OrcaGate{RuntimeID: "runtime-1", ID: "g", TaskID: "t", Status: "pending"}) {
			t.Fatalf("gates = %#v err=%v", got, err)
		}
	})
}

func TestRunInventorySnapshotValidation(t *testing.T) {
	good := port.OrcaRun{RuntimeID: "runtime-1", ID: "run_a", Objective: "a"}
	with := func(mutate func(*port.OrcaRun)) port.OrcaRunInventory {
		run := good
		mutate(&run)
		return port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{run}}
	}
	tests := []struct {
		name      string
		inventory port.OrcaRunInventory
		code      string
	}{
		{"blank runtime", port.OrcaRunInventory{RuntimeID: " "}, "run_inventory_runtime_invalid"},
		{"padded runtime", port.OrcaRunInventory{RuntimeID: "runtime-1 "}, "run_inventory_runtime_invalid"},
		{"run from another runtime", with(func(r *port.OrcaRun) { r.RuntimeID = "runtime-2" }), "run_inventory_identity_invalid"},
		{"blank objective", with(func(r *port.OrcaRun) { r.Objective = "  " }), "run_inventory_identity_invalid"},
		{"padded objective", with(func(r *port.OrcaRun) { r.Objective = "a " }), "run_inventory_identity_invalid"},
		{"padded run id", with(func(r *port.OrcaRun) { r.ID = "run_a " }), "run_inventory_identity_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(newFakeRunner(t))
			_, err := client.ListAllTasksFromRuns(context.Background(), tt.inventory)
			requireOrcaCode(t, err, tt.code)
			_, err = client.ListGatesFromRuns(context.Background(), tt.inventory)
			requireOrcaCode(t, err, tt.code)
		})
	}
}

func TestRunInventoryReadersAcceptAnEmptySnapshot(t *testing.T) {
	runner := newFakeRunner(t)
	client := NewClient(runner)
	inventory := port.OrcaRunInventory{RuntimeID: "runtime-1"}
	tasks, err := client.ListDispatchedTasksFromRuns(context.Background(), inventory)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
	gates, err := client.ListGatesFromRuns(context.Background(), inventory)
	if err != nil || len(gates) != 0 {
		t.Fatalf("gates=%v err=%v", gates, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("an empty snapshot queried Orca: %#v", runner.calls)
	}
}

func TestListGatesFromRunsRejectsForeignRuntimeAndRunErrors(t *testing.T) {
	t.Run("gate list from another runtime", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration gate-list --run run_a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runId":"run_a","gates":[],"count":0},"_meta":{"runtimeId":"runtime-other"}}`)}
		_, err := NewClient(runner).ListGatesFromRuns(context.Background(), oneRunInventory())
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("a failing run read aborts the inventory", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.errors["orca orchestration gate-list --run run_a --json"] = errors.New("run_a failed")
		_, err := NewClient(runner).ListGatesFromRuns(context.Background(), oneRunInventory())
		if err == nil || !strings.Contains(err.Error(), "run_a failed") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestListTasksFromRunsRejectsDuplicateTaskInOneRun(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration task-list --brief --run run_a --json"] = okEnvelope(`{"runId":"run_a","count":2,"tasks":[{"id":"t1","status":"ready"},{"id":"t1","status":"ready"}]}`)
	_, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), oneRunInventory())
	requireOrcaCode(t, err, "task_inventory_ambiguous")
}

func TestListRunInventoryPropagatesStatusFailure(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca status --json"] = failEnvelope("runtime_unreachable", "down")
	_, err := NewClient(runner).ListRunInventory(context.Background())
	requireOrcaCode(t, err, "runtime_unreachable")
	if len(runner.calls) != 1 {
		t.Fatalf("runs were listed after status failed: %#v", runner.calls)
	}
}

func TestListRunInventoryRejectsStatusRuntimeMismatch(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca status --json"] = okEnvelope(`{"runtime":{"runtimeId":"runtime-9"}}`)
	_, err := NewClient(runner).ListRunInventory(context.Background())
	if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunListPagesRejectMalformedRows(t *testing.T) {
	for name, page := range map[string]string{
		"run without id":        `{"runs":[{"objective":"a"}],"nextCursor":null}`,
		"run without objective": `{"runs":[{"id":"run_a"}],"nextCursor":null}`,
		"empty cursor":          `{"runs":[],"nextCursor":""}`,
		"padded cursor":         `{"runs":[],"nextCursor":" c1"}`,
		"non string cursor":     `{"runs":[],"nextCursor":7}`,
	} {
		t.Run(name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses["orca orchestration run-list --json"] = okEnvelope(page)
			_, err := NewClient(runner).ListRuns(context.Background())
			if err == nil {
				t.Fatal("malformed run-list page accepted")
			}
			if len(runner.calls) != 1 {
				t.Fatalf("a malformed page was followed: %#v", runner.calls)
			}
		})
	}
}

func TestCreateRunRejectsUnboundedObjectives(t *testing.T) {
	for _, objective := range []string{" ", " obj", "obj ", "ob\x00j", strings.Repeat("x", 4097), ""} {
		runner := newFakeRunner(t)
		_, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: objective})
		requireOrcaCode(t, err, "run_objective_invalid")
		if len(runner.calls) != 0 {
			t.Fatalf("objective %q reached Orca", objective)
		}
	}
	runner := newFakeRunner(t)
	runner.responses["orca orchestration run-create --objective "+strings.Repeat("x", 4096)+" --json"] = okEnvelope(`{"run":{"id":"run_a","objective":"` + strings.Repeat("x", 4096) + `"}}`)
	if _, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: strings.Repeat("x", 4096)}); err != nil {
		t.Fatalf("the 4096-byte boundary must be accepted: %v", err)
	}
}

func TestRunMutationsRejectIncompleteOrFailedOrcaResponses(t *testing.T) {
	const create = "orca orchestration run-create --objective o --json"
	t.Run("orca failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = failEnvelope("runtime_error", "boom")
		_, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: "o"})
		requireOrcaCode(t, err, "runtime_error")
	})
	t.Run("response without run identity", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = okEnvelope(`{"run":{"objective":"o"}}`)
		_, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: "o"})
		if orcaErr := requireOrcaError(t, err, "run_identity_incomplete"); !orcaErr.Invoked {
			t.Fatal("the Run was created remotely; the incomplete response must report invoked")
		}
	})
	t.Run("response with padded objective", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[create] = okEnvelope(`{"run":{"id":"run_a","objective":"o "}}`)
		_, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: "o"})
		requireOrcaCode(t, err, "run_identity_incomplete")
	})
}

func TestUseRunValidatesIdentityAndBinding(t *testing.T) {
	t.Run("invalid run id", func(t *testing.T) {
		for _, id := range []string{"", " run_a", "run_a ", "ru\x00n"} {
			runner := newFakeRunner(t)
			_, err := NewClient(runner).UseRun(context.Background(), id)
			requireOrcaCode(t, err, "run_identity_invalid")
			if len(runner.calls) != 0 {
				t.Fatalf("run id %q reached Orca", id)
			}
		}
	})
	t.Run("missing coordinator handle", func(t *testing.T) {
		runner := newFakeRunner(t)
		t.Setenv("ORCA_TERMINAL_HANDLE", "")
		_, err := NewClient(runner).UseRun(context.Background(), "run_a")
		requireOrcaCode(t, err, "coordinator_identity_unavailable")
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("orca bound a different run", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration run-use --id run_a --json"] = okEnvelope(`{"run":{"id":"run_b","objective":"o"}}`)
		_, err := NewClient(runner).UseRun(context.Background(), "run_a")
		if orcaErr := requireOrcaError(t, err, "run_binding_mismatch"); !orcaErr.Invoked {
			t.Fatal("binding happened remotely and must report invoked")
		}
	})
	t.Run("orca failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration run-use --id run_a --json"] = failEnvelope("run_not_found", "nope")
		_, err := NewClient(runner).UseRun(context.Background(), "run_a")
		requireOrcaCode(t, err, "run_not_found")
	})
}

func TestCurrentCoordinatorHandleRequiresCanonicalConcreteTerminal(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		ok          bool
	}{
		{"concrete", "term_abc-1_Z", true},
		{"max length", "term_" + strings.Repeat("a", 251), true},
		{"unset", "", false},
		{"leading space", " term_a", false},
		{"trailing newline", "term_a\n", false},
		{"abstract name", "terminal-1", false},
		{"too long", "term_" + strings.Repeat("a", 252), false},
		{"inner space", "term_a b", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ORCA_TERMINAL_HANDLE", tt.value)
			handle, err := currentCoordinatorHandle()
			if tt.ok {
				if err != nil || handle != tt.value {
					t.Fatalf("handle=%q err=%v", handle, err)
				}
				return
			}
			requireOrcaCode(t, err, "coordinator_identity_unavailable")
		})
	}
}

func TestCurrentRunProjectionFailures(t *testing.T) {
	const command = "orca orchestration run-current --json"
	t.Run("orca failure keeps the runtime", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = failEnvelope("runtime_error", "boom")
		inventory, err := NewClient(runner).currentRunInventory(context.Background())
		requireOrcaCode(t, err, "runtime_error")
		if inventory.RuntimeID != "runtime-1" || inventory.Run != nil {
			t.Fatalf("inventory = %#v", inventory)
		}
	})
	t.Run("malformed projection", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"run":"not-an-object"}`)
		inventory, err := NewClient(runner).currentRunInventory(context.Background())
		requireOrcaCode(t, err, "run_identity_invalid")
		if inventory.RuntimeID != "runtime-1" || inventory.Run != nil {
			t.Fatalf("inventory = %#v", inventory)
		}
	})
	t.Run("incomplete projection", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"run":{"id":"run_a"}}`)
		_, err := NewClient(runner).CurrentRun(context.Background())
		requireOrcaCode(t, err, "run_identity_incomplete")
	})
	t.Run("no coordinator", func(t *testing.T) {
		runner := newFakeRunner(t)
		t.Setenv("ORCA_TERMINAL_HANDLE", "")
		_, err := NewClient(runner).CurrentRun(context.Background())
		requireOrcaCode(t, err, "coordinator_identity_unavailable")
	})
}

func TestInboxPresenceFailsClosed(t *testing.T) {
	const command = "orca orchestration inbox --limit 1 --json"
	t.Run("orca failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = failEnvelope("runtime_error", "boom")
		_, err := NewClient(runner).InboxPresence(context.Background())
		requireOrcaCode(t, err, "runtime_error")
	})
	t.Run("missing count", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"messages":[]}`)
		_, err := NewClient(runner).InboxPresence(context.Background())
		if err == nil || !strings.Contains(err.Error(), "completeness metadata is missing") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestCreateTaskValidatesRunAndCoordinatorBeforeInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	_, err := NewClient(runner).CreateTask(context.Background(), port.OrcaCreateTaskRequest{RunID: " ", Spec: "s", Title: "t", DisplayName: "d"})
	requireOrcaCode(t, err, "run_identity_invalid")

	t.Setenv("ORCA_TERMINAL_HANDLE", "")
	_, err = NewClient(runner).CreateTask(context.Background(), port.OrcaCreateTaskRequest{RunID: "run_a", Spec: "s", Title: "t", DisplayName: "d"})
	requireOrcaCode(t, err, "coordinator_identity_unavailable")
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCreateTaskRunBinding(t *testing.T) {
	const command = "orca orchestration task-create --spec s --task-title t --display-name d --run run_a --json"
	request := port.OrcaCreateTaskRequest{RunID: "run_a", Spec: "s", Title: "t", DisplayName: "d"}
	t.Run("task created in another run", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"task":{"id":"task-1","run_id":"run_b","status":"ready"}}`)
		_, err := NewClient(runner).CreateTask(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "task_run_mismatch"); !orcaErr.Invoked {
			t.Fatal("the task exists remotely and must report invoked")
		}
	})
	t.Run("task without a run id inherits the requested run", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = okEnvelope(`{"task":{"id":"task-1","status":"ready"}}`)
		got, err := NewClient(runner).CreateTask(context.Background(), request)
		if err != nil || got.RunID != "run_a" || got.RuntimeID != "runtime-1" || got.ID != "task-1" {
			t.Fatalf("task=%#v err=%v", got, err)
		}
	})
	t.Run("orca failure still carries the requested run", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = failEnvelope("runtime_error", "boom")
		got, err := NewClient(runner).CreateTask(context.Background(), request)
		requireOrcaCode(t, err, "runtime_error")
		if got.RunID != "run_a" {
			t.Fatalf("task = %#v", got)
		}
	})
}

func TestDispatchPreInvocationValidationAndNullResponse(t *testing.T) {
	good := port.OrcaDispatchRequest{RunID: "run_a", TaskID: "task-1", ToHandle: "term_a"}
	t.Run("invalid run", func(t *testing.T) {
		runner := newFakeRunner(t)
		request := good
		request.RunID = ""
		_, err := NewClient(runner).Dispatch(context.Background(), request)
		requireOrcaCode(t, err, "run_identity_invalid")
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("missing coordinator", func(t *testing.T) {
		runner := newFakeRunner(t)
		t.Setenv("ORCA_TERMINAL_HANDLE", "")
		_, err := NewClient(runner).Dispatch(context.Background(), good)
		requireOrcaCode(t, err, "coordinator_identity_unavailable")
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("null dispatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration dispatch --task task-1 --to term_a --run run_a --json"] = okEnvelope(`{"dispatch":null}`)
		_, err := NewClient(runner).Dispatch(context.Background(), good)
		requireOrcaCode(t, err, "not_found")
	})
	t.Run("orca failure", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration dispatch --task task-1 --to term_a --run run_a --json"] = failEnvelope("runtime_error", "boom")
		_, err := NewClient(runner).Dispatch(context.Background(), good)
		requireOrcaCode(t, err, "runtime_error")
	})
}

func TestShowRequestValidatesIdentityAndPropagatesFailure(t *testing.T) {
	runner := newFakeRunner(t)
	_, err := NewClient(runner).ShowRequest(context.Background(), "not-a-uuid")
	requireOrcaCode(t, err, "request_identity_invalid")
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	const id = "11111111-1111-4111-8111-111111111111"
	runner.responses["orca orchestration request-show --request "+id+" --json"] = failEnvelope("not_found", "unknown")
	_, err = NewClient(runner).ShowRequest(context.Background(), " "+id+" ")
	requireOrcaCode(t, err, "not_found")
}

func TestRunTextRejectsOversizedAndFailedOutput(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca big --help"] = CommandOutput{Stdout: make([]byte, MaxEnvelopeBytes+1)}
	runner.errors["orca broken --help"] = errors.New("exec failed")
	client := NewClient(runner)
	if _, err := client.runText(context.Background(), "", readTimeout, []string{"orca", "big", "--help"}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized output error = %v", err)
	}
	if _, err := client.runText(context.Background(), "", readTimeout, []string{"orca", "broken", "--help"}); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("runner failure error = %v", err)
	}
}

func TestDecodeResultRejectsOversizedMissingAndNullResults(t *testing.T) {
	var target struct{}
	if _, err := decodeResult(CommandOutput{Stdout: make([]byte, MaxEnvelopeBytes+1)}, &target); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized envelope error = %v", err)
	}
	for name, body := range map[string]string{
		"missing result": `{"ok":true,"_meta":{"runtimeId":"rt"}}`,
		"null result":    `{"ok":true,"result":null,"_meta":{"runtimeId":"rt"}}`,
		"wrong shape":    `{"ok":true,"result":"text","_meta":{"runtimeId":"rt"}}`,
	} {
		runtimeID, err := decodeResult(CommandOutput{Stdout: []byte(body)}, &target)
		if err == nil || !strings.Contains(err.Error(), "decode Orca result") || runtimeID != "rt" {
			t.Errorf("%s: runtime=%q err=%v", name, runtimeID, err)
		}
	}
	// A nil target accepts any ok envelope, including one without a result.
	if runtimeID, err := decodeResult(CommandOutput{Stdout: []byte(`{"ok":true,"_meta":{"runtimeId":"rt"}}`)}, nil); err != nil || runtimeID != "rt" {
		t.Fatalf("nil target runtime=%q err=%v", runtimeID, err)
	}
}

func TestDecodeResultDefaultsRejectionCodeAndRedactsDiagnostics(t *testing.T) {
	_, err := decodeResult(CommandOutput{Stdout: []byte(`{"ok":false,"error":{"message":"  token=abc  "},"_meta":{"runtimeId":"rt"}}`), Invoked: true}, nil)
	orcaErr := requireOrcaError(t, err, "orca_rejected")
	if orcaErr.Detail != "<redacted>" || !orcaErr.Invoked {
		t.Fatalf("rejection = %#v", orcaErr)
	}
}
