package issueopsapp

import (
	verifyworkapp "issueops/internal/application/verifywork"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestVerifyWorkInstancesKeepHarnessContextAndReloadPolicy(t *testing.T) {
	repo := t.TempDir()
	command := exec.Command("git", "init", "-q", repo)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, output)
	}
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	roots := []string{t.TempDir(), t.TempDir()}
	services := make([]verifyworkapp.Service, 2)
	for i, root := range roots {
		t.Setenv("ISSUEOPS_ROOT", root)
		services[i] = newVerifyWorkService()
	}
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	var wg sync.WaitGroup
	for i, service := range services {
		wg.Add(1)
		go func(i int, service verifyworkapp.Service) {
			defer wg.Done()
			result := service.Run(repo, false, nil)
			if !result.OK {
				t.Errorf("instance %d: %+v", i, result.Warnings)
			}
			want := filepath.Join(roots[i], ".issueops", "COMMIT_POLICY.md")
			if got := result.Preflight.CommitStyleHints["message_policy_doc_path"]; got != want {
				t.Errorf("instance %d policy path=%v want=%s", i, got, want)
			}
		}(i, service)
	}
	wg.Wait()
	directory := filepath.Join(repo, ".issueops")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(directory, "policy.json")
	// An override must be observed on each run of the same service.
	if err := os.WriteFile(policyPath, []byte(`{"additional_write_subcommands":{"git":["status"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := services[0].Run(repo, false, []string{"git", "status", "--short"})
	if blocked.OK || blocked.Command == nil || blocked.Command.Executed {
		t.Fatalf("write policy bypassed: %+v", blocked)
	}
	if err := os.WriteFile(policyPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	allowed := services[0].Run(repo, false, []string{"git", "status", "--short"})
	if !allowed.OK || !allowed.Command.Executed {
		t.Fatalf("policy was cached: %+v", allowed)
	}
	marker := filepath.Join(repo, "must-not-exist")
	denied := services[1].Run(repo, false, []string{"sh", "-c", "touch " + marker})
	if denied.OK || denied.Command.Executed {
		t.Fatalf("shell ran: %+v", denied)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("denied shell touched marker: %v", err)
	}
}
