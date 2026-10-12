package orca

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

func TestClientAcceptsEnvelopeAndTextOutputAtExactLimit(t *testing.T) {
	t.Run("envelope", func(t *testing.T) {
		runner := newFakeRunner(t)
		base := string(fixtureOutput(t, "status_ready.json").Stdout)
		padded := base + strings.Repeat(" ", MaxEnvelopeBytes-len(base))
		runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(padded)}
		if _, err := NewClient(runner).Status(context.Background()); err != nil {
			t.Fatalf("envelope of exactly %d bytes rejected: %v", MaxEnvelopeBytes, err)
		}
	})
	t.Run("help text", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json" + strings.Repeat(" ", MaxEnvelopeBytes-len("--worktree --command --title --json")))}
		runner.responses["orca terminal create --worktree id:w --command claude --model 'm' --dangerously-skip-permissions --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"t","worktreeId":"w"}}}`)}
		_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "w", Agent: "claude", Model: "m"})
		if err != nil {
			t.Fatalf("help text of exactly %d bytes rejected: %v", MaxEnvelopeBytes, err)
		}
	})
}

func TestUseRunAcceptsRunIDAtExactLengthLimit(t *testing.T) {
	runner := newFakeRunner(t)
	runID := strings.Repeat("r", 1024)
	sentinel := errors.New("run-use reached the runner")
	runner.errors["orca orchestration run-use --id "+runID+" --json"] = sentinel
	if _, err := NewClient(runner).UseRun(context.Background(), runID); !errors.Is(err, sentinel) {
		t.Fatalf("1024-byte run id was refused before invocation: %v", err)
	}
	if _, err := NewClient(runner).UseRun(context.Background(), runID+"r"); err == nil || errors.Is(err, sentinel) {
		t.Fatalf("1025-byte run id must be refused before invocation: %v", err)
	}
}

func TestStableVisualTabTitlesBoundaries(t *testing.T) {
	t.Run("layout count at limit", func(t *testing.T) {
		layouts := make([]visualLayoutPayload, port.OrcaMaxBaselineIDs)
		if _, err := stableVisualTabTitles(layouts); err != nil {
			t.Fatalf("%d layouts rejected: %v", len(layouts), err)
		}
		if _, err := stableVisualTabTitles(make([]visualLayoutPayload, port.OrcaMaxBaselineIDs+1)); err == nil || !strings.Contains(err.Error(), "layout inventory exceeds") {
			t.Fatalf("layout overflow error = %v", err)
		}
	})
	t.Run("identity length limits", func(t *testing.T) {
		for _, tt := range []struct {
			name              string
			tab, leaf, title  string
			wantAcceptedLimit bool
		}{
			{"tab", strings.Repeat("t", 1024), "leaf", "title", true},
			{"leaf", "tab", strings.Repeat("l", 1024), "title", true},
			{"title", "tab", "leaf", strings.Repeat("m", 4096), true},
			{"tab over", strings.Repeat("t", 1025), "leaf", "title", false},
			{"leaf over", "tab", strings.Repeat("l", 1025), "title", false},
			{"title over", "tab", "leaf", strings.Repeat("m", 4097), false},
		} {
			err := addVisualTabTitle(map[string]string{}, tt.tab, tt.leaf, tt.title)
			if (err == nil) != tt.wantAcceptedLimit {
				t.Errorf("%s: err = %v, accepted want %t", tt.name, err, tt.wantAcceptedLimit)
			}
		}
	})
}

func TestOwnerAgentCommandRejectsNULAtFirstByte(t *testing.T) {
	for _, tt := range []struct {
		name, model, effort string
		extra               []string
	}{
		{"model", "\x00m", "", nil},
		{"effort", "m", "\x00high", nil},
		{"extra arg", "m", "", []string{"\x00arg"}},
	} {
		if command, ok := ownerAgentCommand("omp", tt.model, tt.effort, false, tt.extra); ok {
			t.Errorf("%s: launch command %q was rendered", tt.name, command)
		}
	}
}

func TestContainsAllHelpFlagsStripsInlineValue(t *testing.T) {
	if !containsAllHelpFlags("[--worktree=<id>]", []string{"--worktree"}) {
		t.Fatal("--worktree=<id> must satisfy --worktree")
	}
	if containsAllHelpFlags("--worktree=--json", []string{"--json"}) {
		t.Fatal("an inline value must not be read as a flag")
	}
}

func TestInvokeTerminalIntentAttestsCodexHookTrustBypassOnlyForCodex(t *testing.T) {
	for _, tt := range []struct {
		host string
		want bool
	}{{"codex", true}, {"claude", false}} {
		fx := newStageFixture(t)
		fx.probe.Host = tt.host
		fake := fx.fake()
		if _, err := NewExecutionClient(fake).InvokeIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTerminal)); err != nil {
			t.Fatalf("host %s: %v", tt.host, err)
		}
		if countCalls(fake.calls, "create-terminal") != 1 {
			t.Fatalf("host %s: terminal was not created: %v", tt.host, fake.calls)
		}
		if got := fake.terminalRequest.AllowCodexHookTrustBypass; got != tt.want {
			t.Errorf("host %s: AllowCodexHookTrustBypass = %t, want %t", tt.host, got, tt.want)
		}
	}
}

func TestBoundedStreamBufferFlagsAFullBufferOnlyOnNonEmptyWrite(t *testing.T) {
	buffer := boundedStreamBuffer{limit: 3}
	if n, err := buffer.Write([]byte("abc")); n != 3 || err != nil || buffer.exceeded {
		t.Fatalf("write filling the limit exactly n=%d err=%v exceeded=%t", n, err, buffer.exceeded)
	}
	if string(buffer.Bytes()) != "abc" {
		t.Fatalf("retained = %q", buffer.Bytes())
	}
	if _, err := buffer.Write(nil); err != nil || buffer.exceeded {
		t.Fatalf("empty write to a full buffer err=%v exceeded=%t", err, buffer.exceeded)
	}
	if _, err := buffer.Write([]byte("d")); err != nil || !buffer.exceeded {
		t.Fatalf("write beyond a full buffer err=%v exceeded=%t", err, buffer.exceeded)
	}
}

func TestBoundedDiagnosticKeepsTextAtExactLimit(t *testing.T) {
	exact := strings.Repeat("x", 1024)
	if got := boundedDiagnostic(exact); got != exact {
		t.Fatalf("1024-byte diagnostic was altered (len %d)", len(got))
	}
	if got := boundedDiagnostic(exact + "x"); got != exact+"..." {
		t.Fatalf("1025-byte diagnostic len %d", len(got))
	}
}

func TestExecRunnerReportsExitCodeOfFailedCommand(t *testing.T) {
	output, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{"/bin/sh", "-c", "exit 7"})
	if err == nil || output.ExitCode != 7 {
		t.Fatalf("exit code = %d err=%v, want 7 with error", output.ExitCode, err)
	}
}

func TestProbeRejectsRunListFromAnotherRuntime(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	other := fixtureOutput(t, "run_list.json")
	other.Stdout = []byte(strings.ReplaceAll(string(other.Stdout), "runtime-1", "runtime-other"))
	runner.responses["orca orchestration run-list --json"] = other

	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Code != "orchestration_unready" {
		t.Fatalf("run-list from another runtime was accepted: %#v", result)
	}
}
