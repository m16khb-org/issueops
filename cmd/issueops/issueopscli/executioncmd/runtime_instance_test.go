package executioncmd

import (
	"context"
	"fmt"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestExecutionCommandsKeepRunnerAndActorOwnership(t *testing.T) {
	for key, value := range map[string]string{"CODEX_THREAD_ID": "session", "CLAUDE_CODE_SESSION_ID": "", "PI_SESSION_ID": ""} {
		t.Setenv(key, value)
	}
	var events []string
	build := func(owner string) func([]string) error {
		check := func(root string, actor model.NativeActor) {
			t.Helper()
			if root != owner || actor.Host != "codex" || actor.SessionID != "session" || len(actor.ProcessAncestry) != 1 || actor.ProcessAncestry[0].Executable != "/"+owner+"/codex" {
				t.Fatalf("execution ownership crossed: owner=%s root=%s actor=%+v", owner, root, actor)
			}
		}
		runtime := ExecutionDeps{
			ObserveNativeProcessAncestry: func(pid int) ([]model.NativeProcessReceipt, error) {
				if pid != os.Getpid() {
					t.Fatalf("unexpected process observation: %d", pid)
				}
				events = append(events, owner+":observe")
				return []model.NativeProcessReceipt{{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/" + owner + "/codex"}}, nil
			},
			ExecuteExecution: func(_ context.Context, root string, req model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
				check(root, req.Actor)
				if req.ID != "cycle" {
					t.Fatalf("lost cycle id: %+v", req)
				}
				if req.Action == model.ExecutionActionClaim && (!req.ClaimCurrentToken || req.Generation != 7 || req.Actor.SessionProcess.PID != 42) {
					t.Fatalf("claim authority changed: %+v", req)
				}
				events = append(events, owner+":"+string(req.Action))
				return model.ExecutionResult{OK: true, ID: req.ID}, nil
			},
			SyncExecutionBase: func(_ context.Context, root string, req model.ExecutionSyncBaseRequest, _ model.ExecutionSyncBaseDeps) (model.ExecutionSyncBaseResult, error) {
				check(root, req.Actor)
				if req.CompletionGeneration != 7 || req.Mode != model.ExecutionSyncBasePreview {
					t.Fatalf("sync authority changed: %+v", req)
				}
				events = append(events, owner+":sync")
				return model.ExecutionSyncBaseResult{OK: true, ID: req.ID}, nil
			},
			SwitchExecutionMode: func(_ context.Context, root string, req model.ExecutionSwitchModeRequest) (model.ExecutionSwitchModeResult, error) {
				check(root, req.Actor)
				if req.Mode != "direct" || req.Apply || req.Confirm {
					t.Fatalf("switch preview changed: %+v", req)
				}
				events = append(events, owner+":switch")
				return model.ExecutionSwitchModeResult{OK: true, ID: req.ID}, nil
			},
		}
		return executionRunnerForIsolation(runtime, Deps{StateRoot: func() string { return owner }, PrintJSON: func(value any) error {
			if who, ok := value.(ExecutionWhoamiResult); ok {
				if len(who.Ancestry) != 1 || who.Ancestry[0].Executable != "/"+owner+"/codex" {
					t.Fatalf("whoami observation crossed: %+v", who)
				}
				events = append(events, owner+":whoami")
			}
			return nil
		}})
	}
	a, b := build("a"), build("b")
	actor := []string{"--host", "codex", "--session-id", "session", "--session-pid", "42", "--session-started-at", "2026-08-01T00:00:00Z", "--session-executable", "/codex", "--cwd", "/workspace", "--json"}
	for _, run := range []func([]string) error{a, b, a} {
		for _, args := range [][]string{
			{"whoami", "--json"},
			append([]string{"release", "--id", "cycle", "--generation", "7"}, actor...),
			{"claim", "--id", "cycle", "--generation", "7", "--claim-current-token", "--json"},
			append([]string{"sync-base", "--id", "cycle", "--completion-generation", "7", "--preview"}, actor...),
			append([]string{"switch-mode", "--id", "cycle", "--mode", "direct"}, actor...),
		} {
			if err := run(args); err != nil {
				t.Fatal(fmt.Errorf("%s: %w", args[0], err))
			}
		}
	}
	want := []string{"a:observe", "a:whoami", "a:observe", "a:release", "a:observe", "a:claim", "a:observe", "a:sync", "a:observe", "a:switch", "b:observe", "b:whoami", "b:observe", "b:release", "b:observe", "b:claim", "b:observe", "b:sync", "b:observe", "b:switch", "a:observe", "a:whoami", "a:observe", "a:release", "a:observe", "a:claim", "a:observe", "a:sync", "a:observe", "a:switch"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("execution instances shared callbacks:\ngot %v\nwant %v", events, want)
	}
}

func executionRunnerForIsolation(runtime ExecutionDeps, deps Deps) func([]string) error {
	deps.Runtime = runtime
	return func(args []string) error { return Run(args, deps) }
}

func TestExecutionWithoutRunnersFailsClosed(t *testing.T) {
	for key, value := range map[string]string{"CODEX_THREAD_ID": "session", "CLAUDE_CODE_SESSION_ID": "", "PI_SESSION_ID": ""} {
		t.Setenv(key, value)
	}
	for _, args := range [][]string{{"status", "--id", "cycle"}, {"sync-base", "--id", "cycle", "--preview"}, {"switch-mode", "--id", "cycle", "--mode", "direct"}, {"whoami"}} {
		err := Run(args, Deps{StateRoot: func() string { return "/state" }})
		if err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Fatalf("missing execution runner: args=%v err=%v", args, err)
		}
	}
}
