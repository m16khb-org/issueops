package issueopsapp

import (
	statusapp "issueops/internal/application/status"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusInstancesKeepCapturedStoresAndInspectionContext(t *testing.T) {
	var services [2]statusapp.Service
	var roots [2]string
	payloads := []string{"first", "second payload"}
	for i, payload := range payloads {
		roots[i] = t.TempDir()
		home := filepath.Join(roots[i], "home")
		if err := os.MkdirAll(home, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("ISSUEOPS_ROOT", roots[i])
		t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(roots[i], "state"))
		t.Setenv("ISSUEOPS_WORKER_DIR", filepath.Join(roots[i], "worker"))
		t.Setenv("ISSUEOPS_DAEMON_DIR", filepath.Join(roots[i], "daemon"))
		if _, err := newStateService(filepath.Join(roots[i], "state")).Write("self-verify-fixture", payload); err != nil {
			t.Fatal(err)
		}
		if _, err := newWorkerService().Enqueue("fixture", payload); err != nil {
			t.Fatal(err)
		}
		services[i] = newStatusService()
	}
	for i, service := range services {
		got := service.Run(roots[i])
		if got.Inspect.IssueOpsRoot != roots[i] || got.Inspect.Integration.CodexSkillPath != filepath.Join(roots[i], "home", ".codex", "skills", skillName) {
			t.Fatalf("inspection context leaked for %d: %+v", i, got.Inspect)
		}
		if !got.State.OK || len(got.State.Records) != 1 || !got.SelfVerify.Found || got.SelfVerify.LatestKey != "self-verify-fixture" || got.SelfVerify.Bytes != len(payloads[i]) {
			t.Fatalf("state leaked for %d: %+v %+v", i, got.State, got.SelfVerify)
		}
		if !got.Workers.OK || len(got.Workers.Jobs) != 1 || got.Workers.Jobs[0].Payload != payloads[i] {
			t.Fatalf("worker store leaked for %d: %+v", i, got.Workers)
		}
	}
}
