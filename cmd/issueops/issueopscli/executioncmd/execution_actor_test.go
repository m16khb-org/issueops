package executioncmd

import (
	"context"
	"flag"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestExecutionReplaceCLIInvokesExecutionWithObservedActor(t *testing.T) {
	var got model.ExecutionActionRequest
	execute := func(_ context.Context, _ string, request model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		got = request
		return model.ExecutionReplaceResult{
			OK: true, ID: request.ID, Action: request.ReplaceAction,
			Execution: model.Execution{Lease: model.WriteLease{Generation: request.ExpectedGeneration}},
		}, nil
	}
	observation := nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CODEX_THREAD_ID" {
				return "codex-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/parent", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return []model.NativeProcessReceipt{
				{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/issueops"},
				{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/codex/bin/codex"},
			}, nil
		},
	}
	err := runExecutionForTest([]string{
		"replace", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "13", "--preview", "--json",
	}, Deps{
		Runtime:                ExecutionDeps{ExecuteExecution: execute},
		StateRoot:              func() string { return t.TempDir() },
		PrintJSON:              func(any) error { return nil },
		nativeActorObservation: &observation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionReplace || got.ReplaceAction != model.ExecutionReplacePreview || got.ExpectedGeneration != 13 {
		t.Fatalf("replace request=%+v", got)
	}
	if got.Actor.Host != "codex" || got.Actor.SessionID != "codex-session" ||
		got.Actor.SessionProcess == nil || got.Actor.SessionProcess.PID != 42 ||
		got.CWD != "/repo.worktrees/parent" {
		t.Fatalf("observed replace request=%+v", got)
	}
}

func TestResolveNativeActorObservesNativeIdentityWhenActorFlagsAreAbsent(t *testing.T) {
	flags, visited := actorFlagsForTest(t, nil)
	ancestry := []model.NativeProcessReceipt{
		{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/issueops"},
		{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/codex/bin/codex"},
	}
	actor, cwd, err := resolveNativeActor("replace", flags, visited, nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CODEX_THREAD_ID" {
				return "codex-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/replace", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return ancestry, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.Host != "codex" || actor.SessionID != "codex-session" || cwd != "/repo.worktrees/replace" {
		t.Fatalf("resolved actor=%+v cwd=%q", actor, cwd)
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID != 42 || len(actor.ProcessAncestry) != 1 {
		t.Fatalf("native host receipt was not selected: %+v", actor)
	}
}

func TestResolveNativeActorPreservesCompleteExplicitFlags(t *testing.T) {
	args := []string{
		"--host", "claude", "--session-id", "claude-session", "--agent-id", "worker-1",
		"--session-pid", "42", "--session-started-at", "2026-08-01T00:00:00Z",
		"--session-executable", "/opt/claude/bin/claude", "--cwd", "/repo.worktrees/replace",
	}
	flags, visited := actorFlagsForTest(t, args)
	actor, cwd, err := resolveNativeActor("replace", flags, visited, nativeActorObservation{
		Getenv: func(string) string { t.Fatal("explicit flags must not read native env"); return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.Host != "claude" || actor.SessionID != "claude-session" || actor.AgentID != "worker-1" ||
		actor.SessionProcess == nil || actor.SessionProcess.PID != 42 || cwd != "/repo.worktrees/replace" {
		t.Fatalf("explicit actor=%+v cwd=%q", actor, cwd)
	}
}

func TestResolveNativeActorRejectsPartialExplicitFlags(t *testing.T) {
	for name, args := range map[string][]string{
		"host only":     {"--host", "codex"},
		"cwd only":      {"--cwd", "/repo.worktrees/replace"},
		"agent id only": {"--agent-id", "worker-1"},
	} {
		t.Run(name, func(t *testing.T) {
			flags, visited := actorFlagsForTest(t, args)
			_, _, err := resolveNativeActor("replace", flags, visited, nativeActorObservation{})
			if err == nil || !strings.Contains(err.Error(), "all ACTOR_FLAGS or none") {
				t.Fatalf("partial actor flags error = %v", err)
			}
		})
	}
}

func actorFlagsForTest(t *testing.T, args []string) (actorFlags, map[string]bool) {
	t.Helper()
	fs := flag.NewFlagSet("actor test", flag.ContinueOnError)
	flags := addActorFlags(fs, Deps{Runtime: testExecutionRuntime()})
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return flags, visitedFlagNames(fs)
}

func TestExecutionReplaceUsageAdvertisesOptionalActorFlags(t *testing.T) {
	if !strings.Contains(Usage, "[fingerprint/reason flags] [ACTOR_FLAGS] [--confirm]") {
		t.Fatalf("replace usage does not advertise optional actor flags: %s", Usage)
	}
}
