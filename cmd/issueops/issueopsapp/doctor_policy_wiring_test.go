package issueopsapp

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	doctorcontract "issueops/internal/contract/doctor"
	bootstrapcontract "issueops/internal/contract/projectbootstrap"
)

func TestDoctorCLIAppliesDomainFindingsWithoutWriting(t *testing.T) {
	root, home, state, harness := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ISSUEOPS_ROOT", harness)
	t.Setenv("ISSUEOPS_STATE_DIR", state)
	if _, err := newProjectBootstrapService(root).Run(bootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(home, ".codex"), filepath.Join(harness, "bin"), filepath.Join(harness, "internal"), filepath.Join(root, ".issueops", "state")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{filepath.Join(home, ".codex", "hooks.json"): "{}", filepath.Join(root, ".issueops", "STATE.md"): "# Shared JSONL schema", filepath.Join(harness, "bin", "issueops"): "fixture binary", filepath.Join(harness, "internal", "example.go"): "package fixture"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for path, stamp := range map[string]time.Time{filepath.Join(harness, "bin", "issueops"): old, filepath.Join(harness, "internal", "example.go"): old.Add(2 * time.Second)} {
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, ".issueops", "TESTING.md")); err != nil {
		t.Fatal(err)
	}
	before := doctorFixtureBytes(t, root, home, state, harness)
	output := captureStdoutForContract(t, func() error { return runDoctor([]string{"--repo", root, "--static-only", "--json"}) })
	var result doctorcontract.HarnessDoctorResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Healthy || result.RepoRoot != root || !result.LifecycleState.NamespaceValid {
		t.Fatalf("wrong public diagnosis: %+v", result)
	}
	counts := map[string]int{}
	for _, issue := range result.Issues {
		counts[issue.Code]++
	}
	for code, want := range map[string]int{"project_docs_missing": 1, "repo_local_state_present": 2, "binary_drift": 1} {
		if counts[code] != want {
			t.Fatalf("issue %s count=%d want %d: %+v", code, counts[code], want, result.Issues)
		}
	}
	for _, check := range result.Checks {
		if check.Name == "pipe_capacity" || check.Name == "mcp_gateway" {
			t.Fatalf("static-only ran live diagnosis: %+v", check)
		}
	}
	if after := doctorFixtureBytes(t, root, home, state, harness); !reflect.DeepEqual(before, after) {
		t.Fatal("read-only doctor changed fixture files")
	}
}

func doctorFixtureBytes(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				files[path] = "directory"
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}
