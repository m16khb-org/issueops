package issueopsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/issueopscli"
	contract "issueops/internal/contract/issueopsbenchmark"
)

func TestBenchmarkCLIInstancesKeepCapturedState(t *testing.T) {
	fixtures := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixtures, "fixture.json"), []byte(`{"id":"fixture","title":"Benchmark","user_prompt":"Improve workflow","repo_context":"Go CLI","critical_failures":["missing issue"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	var deps [2]issueopscli.Dependencies
	for i := range deps {
		t.Setenv("ISSUEOPS_STATE_DIR", roots[i])
		deps[i] = issueOpsCLIDependencies()
	}
	t.Setenv("ISSUEOPS_STATE_DIR", roots[2])
	for _, i := range []int{0, 1, 0} {
		raw := captureStdoutForContract(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"benchmark", "run", "--fixtures", fixtures, "--json"}, deps[i])
		})
		var result contract.IssueOpsBenchmarkRunResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		persisted, err := os.ReadFile(filepath.Join(roots[i], "issueops-benchmarks", result.ID+".json"))
		if err != nil {
			t.Fatalf("instance %d did not persist in its captured state directory: %v", i, err)
		}
		var stored contract.IssueOpsBenchmarkRunResult
		if err := json.Unmarshal(persisted, &stored); err != nil || stored.ID != result.ID || stored.FixtureCount != 1 {
			t.Fatalf("stored benchmark: %+v err=%v", stored, err)
		}
	}
	for i, count := range []int{2, 1} {
		entries, err := os.ReadDir(filepath.Join(roots[i], "issueops-benchmarks"))
		if err != nil || len(entries) != count {
			t.Fatalf("instance %d persisted %d runs, want %d: %v", i, len(entries), count, err)
		}
	}
	if _, err := os.Stat(filepath.Join(roots[2], "issueops-benchmarks")); !os.IsNotExist(err) {
		t.Fatalf("benchmark wrote to the changed ambient state directory: %v", err)
	}
}
