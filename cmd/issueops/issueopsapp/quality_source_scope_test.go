package issueopsapp

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/qualitycli"
	contract "issueops/internal/contract/quality"
)

func TestQualityProductionSourceWiringAndCLI(t *testing.T) {
	root := t.TempDir()
	writeValidZeroAudit(t, root)
	command := exec.Command("git", "init", "-q", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", output, err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module qualityfixture\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "product.go"), []byte("package fixture\nfunc Product() int { return 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "repro.go"), []byte("package fixture\nfunc Repro() {"+" if true {}; if true {}; if true {}; if true {}; if true {}; if true {}; if true {}"+" }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	deps := newQualityDependencies(root, t.TempDir())
	var result contract.InspectResult
	deps.PrintJSON = func(value any) error { result = value.(contract.InspectResult); return nil }
	// Fixture packages have no coverage tests, so health may block independently
	// of a successful collection. Preserve that distinction.
	if err := qualitycli.Run([]string{"inspect", "--repo", root, "--json"}, deps); err != nil && !errors.Is(err, qualitycli.ErrQualityGateBlocked) {
		t.Fatalf("CLI: %v warnings=%v", err, result.Warnings)
	}
	if !result.OK || result.CollectionStatus != contract.CollectionStatusOK || result.Summary.BranchCandidateFunctions != 0 {
		t.Fatalf("scope/status: %+v", result)
	}
	snrFound := false
	for _, signal := range result.Signals {
		if signal.ID == "code-snr" {
			snrFound = true
			if signal.Status != "ok" || signal.Value != 1 {
				t.Fatalf("SNR: %+v", signal)
			}
		}
	}
	if !snrFound {
		t.Fatal("missing code-snr signal")
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := qualitycli.Run([]string{"inspect", "--repo", root, "--json"}, deps); !errors.Is(err, qualitycli.ErrQualityGateBlocked) {
		t.Fatalf("collection failure exit: %v", err)
	}
	if result.OK || result.CollectionStatus == contract.CollectionStatusOK || result.GateStatus != contract.GateStatusBlock {
		t.Fatalf("hidden collection failure: %+v", result)
	}
	for _, signal := range result.Signals {
		if (signal.ID == "code-snr" || signal.ID == "branch-candidate-functions") && signal.Status != "error" {
			t.Fatalf("hidden collector failure: %+v", signal)
		}
	}
}
