package issueopsapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"issueops/cmd/issueops/basiccli"
	doctorapp "issueops/internal/application/doctor"
	doctorcontract "issueops/internal/contract/doctor"
	loopcontract "issueops/internal/contract/looprun"
	bootstrapcontract "issueops/internal/contract/projectbootstrap"
)

func TestDoctorInstancesKeepLifecycleLoopAndStateTogether(t *testing.T) {
	root, home, harness := t.TempDir(), t.TempDir(), t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", home)
	t.Setenv("ISSUEOPS_ROOT", harness)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "hooks.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	var states [2]string
	var services [2]doctorapp.Service
	var commands [2]basiccli.Doctor
	for i := range states {
		states[i] = t.TempDir()
		t.Setenv("ISSUEOPS_STATE_DIR", states[i])
		if _, err := newProjectBootstrapService(root).Run(bootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := os.WriteFile(filepath.Join(states[i], "unexpected.txt"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := newLoopService().Start(context.Background(), loopcontract.StartLoopRequest{Repo: root, Name: "first-only", Goal: "verify independent doctor state"}); err != nil {
				t.Fatal(err)
			}
		}
		services[i] = newDoctorService()
		commands[i] = newDoctorCommand()
	}
	wrong := filepath.Join(t.TempDir(), "wrong-state")
	t.Setenv("ISSUEOPS_STATE_DIR", wrong)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Chdir(t.TempDir())
	check := func(i int, result doctorcontract.HarnessDoctorResult, err error) {
		t.Helper()
		if err != nil || result.StateDir != states[i] || result.RepoRoot != root || result.LifecycleState.StateRoot != states[i] || !result.LifecycleState.NamespaceValid {
			t.Errorf("instance %d inconsistent snapshot: %+v err=%v", i, result, err)
			return
		}
		hasLoop, hasState := false, false
		for _, issue := range result.Issues {
			if issue.Code == "state_unexpected_file" {
				hasState = true
				if issue.Path != filepath.Join(states[i], "unexpected.txt") {
					t.Errorf("state artifact path drift: %s", issue.Path)
				}
			}
			if issue.Code == "loop_contracts_incomplete" {
				hasLoop = true
			}
			if issue.Code == "codex_hooks_missing" {
				t.Errorf("instance %d home drift", i)
			}
		}
		if hasState != (i == 0) {
			t.Errorf("instance %d state doctor drift: %+v", i, result.Issues)
		}
		if hasLoop != (i == 0) {
			t.Errorf("instance %d leaked loop state: %+v", i, result.Issues)
		}
	}
	var group sync.WaitGroup
	for i := range services {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			for n := 0; n < 3; n++ {
				result, err := services[i].Run(doctorcontract.HarnessDoctorRequest{RepoRoot: ".", Home: home, IssueOpsRoot: harness, StaticOnly: true})
				check(i, result, err)
			}
		}(i)
	}
	group.Wait()
	for i := range commands {
		raw := captureStdoutForContract(t, func() error { return commands[i].Run([]string{"--static-only", "--json"}) })
		var result doctorcontract.HarnessDoctorResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		check(i, result, nil)
		if result.IssueOpsRoot != harness {
			t.Fatalf("harness root drift: %s", result.IssueOpsRoot)
		}
	}
	if _, err := os.Stat(wrong); !os.IsNotExist(err) {
		t.Fatalf("doctor used changed state: %v", err)
	}
}
