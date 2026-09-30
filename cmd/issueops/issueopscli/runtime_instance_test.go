package issueopscli

import (
	"os"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

// Cross-command configuration drift must never redirect a phase mutation or
// replace the process ancestry used to fence the actor.
func TestLifecycleCommandsKeepRuntimeAndActorOwnership(t *testing.T) {
	var events []string
	build := func(owner string) func([]string) error {
		runtime := IssueOpsCLIDeps{
			IssueOpsStateRoot: func() string { return owner },
			ObserveNativeProcessAncestry: func(pid int) ([]model.NativeProcessReceipt, error) {
				if pid != os.Getpid() {
					t.Fatalf("observer pid = %d", pid)
				}
				events = append(events, owner+":actor")
				return []model.NativeProcessReceipt{{PID: pid, Executable: owner}}, nil
			},
			ReadIssueOps: func(root, id string) (model.IssueOpsRecord, error) {
				events = append(events, owner+":read:"+root+":"+id)
				return model.IssueOpsRecord{ID: id}, nil
			},
			IssueOpsPRReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
				events = append(events, owner+":base:"+record.ID)
				return model.IssueOpsReadiness{Ready: true}
			},
		}
		gates := LoopGateDeps{
			AdvancePhaseReport: func(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, model.IssueOpsTrackedMaterials, error) {
				if actor.Host != "codex" || actor.SessionID != "session" || actor.CWD != "/workspace" || len(actor.NativeProcessAncestry) != 1 || actor.NativeProcessAncestry[0].Executable != owner {
					t.Fatalf("actor lost ownership: %+v", actor)
				}
				events = append(events, owner+":advance:"+root+":"+id+":"+to)
				return model.IssueOpsRecord{ID: id}, model.IssueOpsTrackedMaterials{}, nil
			},
			StrictPRReadinessWithState: func(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
				events = append(events, owner+":strict:"+root+":"+record.ID)
				return model.IssueOpsReadiness{Ready: true}
			},
		}
		return runtimeRunnerForIsolation(runtime, gates)
	}
	a, b := build("a"), build("b")
	for _, run := range []func([]string) error{a, b, a} {
		captureStdoutForContract(t, func() error {
			return run([]string{"phase", "--id", "cycle", "--to", "plan", "--host", "codex", "--session-id", "session", "--cwd", "/workspace", "--json"})
		})
		captureStdoutForContract(t, func() error { return run([]string{"pr-readiness", "--id", "cycle", "--strict", "--json"}) })
	}
	want := []string{"a:actor", "a:advance:a:cycle:plan", "a:read:a:cycle", "a:base:cycle", "a:strict:a:cycle", "b:actor", "b:advance:b:cycle:plan", "b:read:b:cycle", "b:base:cycle", "b:strict:b:cycle", "a:actor", "a:advance:a:cycle:plan", "a:read:a:cycle", "a:base:cycle", "a:strict:a:cycle"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("commands shared runtime or changed observation order:\ngot %v\nwant %v", events, want)
	}
}

func runtimeRunnerForIsolation(runtime IssueOpsCLIDeps, gates LoopGateDeps) func([]string) error {
	return func(args []string) error {
		return RunIssueOpsWithDependencies(args, Dependencies{Runtime: runtime, Gates: gates})
	}
}
