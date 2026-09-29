package gates

import (
	"os"
	"path/filepath"
	"testing"

	model "issueops/internal/contract/gates"
)

func TestGateMutationPreservesPrivateFileMode(t *testing.T) {
	file := filepath.Join(t.TempDir(), "GATES.md")
	if err := os.WriteFile(file, []byte("- [ ] G1: proof\n  CHECK: printf ok\n  EXPECT: ok\n  EVIDENCE: pending\n- [ ] G2: manual\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service := gateServiceForTest()
	if result, err := service.Check(model.CheckRequest{Files: []string{file}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}, CWD: filepath.Dir(file), WorkspaceRoot: filepath.Dir(file)}); err != nil || result.TotalMet != 1 {
		t.Fatalf("check=%+v err=%v", result, err)
	}
	if _, err := service.Abandon(model.AbandonRequest{File: file, GateID: "G2", Reason: "later"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
}
