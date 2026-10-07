package issueopsapp

import (
	"errors"
	"issueops/cmd/issueops/qualitycli"
	qualityapp "issueops/internal/application/quality"
	qualitycontract "issueops/internal/contract/quality"
	contract "issueops/internal/contract/qualitycatalog"
	"os"
	"path/filepath"
	"testing"
)

func qualitycliInspectDepsForContract() qualityapp.InspectDeps {
	return qualityapp.InspectDeps{
		Now: func() string { return "2000-01-01T00:00:00Z" },
		Coverage: func(string) (string, error) {
			return "ok  \tissueops/internal/adapter/guard\t0.011s\tcoverage: 54.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 10, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		PioneerCoverage: func(string) (qualitycontract.PioneerCoverage, error) {
			return qualitycontract.PioneerCoverage{
				Expected:             12,
				BenchmarkObserved:    12,
				ReproductionObserved: 12,
			}, nil
		},
		CodeSNR: func(string) (qualitycontract.SNRResult, error) {
			return qualitycontract.SNRResult{SignalLines: 70, NoiseLines: 30, TotalLines: 100, Ratio: 0.7}, nil
		},
	}
}

func TestQualityAuditCollectorCLIStates(t *testing.T) {
	for _, name := range []string{"missing", "malformed", "missing leading pipe", "unreadable", "valid zero", "current document"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			switch name {
			case "valid zero":
				writeValidZeroAudit(t, root)
			case "current document":
				root = filepath.Join("..", "..", "..")
			case "missing leading pipe":
				writeValidZeroAudit(t, root)
				path := filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")
				if err := os.WriteFile(path, []byte("| ID | Area | Title | Priority | Size |\n|---|---|---|---|---|\nA1 | Core | Fix | P0 | Small |\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "malformed", "unreadable":
				writeValidZeroAudit(t, root)
				path := filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")
				if name == "malformed" {
					if err := os.WriteFile(path, []byte("| broken | audit |\n"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			deps := qualitycliInspectDepsForContract()
			deps.Coverage = func(string) (string, error) { return "", nil }
			deps.BranchFunctions = func(string) ([]qualitycontract.BranchFunction, []string) { return nil, nil }
			deps.Candidates = func(string) []contract.Candidate { return nil }
			var result qualitycontract.InspectResult
			err := qualitycli.Run([]string{"inspect", "--repo", root, "--json"}, qualitycli.Deps{
				Inspect:   func(root string) qualitycontract.InspectResult { return inspectQualityForTest(root, deps) },
				PrintJSON: func(value any) error { result = value.(qualitycontract.InspectResult); return nil },
			})
			valid := name == "valid zero" || name == "current document"
			if valid {
				if err != nil || !result.OK || result.CollectionStatus != qualitycontract.CollectionStatusOK || len(result.Warnings) != 0 || result.Summary.AuditP1P2Items != 0 {
					t.Fatalf("valid audit: err=%v result=%+v", err, result)
				}
			} else if !errors.Is(err, qualitycli.ErrQualityGateBlocked) || result.OK || result.CollectionStatus != qualitycontract.CollectionStatusError || result.HealthStatus != qualitycontract.HealthStatusUnknown || result.GateStatus != qualitycontract.GateStatusBlock || len(result.Warnings) == 0 {
				t.Fatalf("invalid audit: err=%v result=%+v", err, result)
			}
			found := false
			for _, signal := range result.Signals {
				if signal.ID == "audit-p0-p1-p2-items" {
					found = true
					want := "error"
					if valid {
						want = "ok"
					}
					if signal.Status != want || signal.Value != 0 {
						t.Fatalf("audit signal=%+v want=%s", signal, want)
					}
				}
			}
			if !found {
				t.Fatal("audit signal missing")
			}
		})
	}
}

func TestQualityContractFixtureSeparatesHealthFromCollection(t *testing.T) {
	root := t.TempDir()
	writeValidZeroAudit(t, root)
	result := inspectQualityForTest(root, qualitycliInspectDepsForContract())

	if !result.OK || result.CollectionStatus != qualitycontract.CollectionStatusOK {
		t.Fatalf("collection status = ok=%v status=%q warnings=%v", result.OK, result.CollectionStatus, result.Warnings)
	}
	if result.HealthStatus != qualitycontract.HealthStatusNeedsAttention || result.GateStatus != qualitycontract.GateStatusReportOnly {
		t.Fatalf("quality statuses = health=%q gate=%q", result.HealthStatus, result.GateStatus)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "low-coverage-packages" {
		t.Fatalf("findings = %+v", result.Findings)
	}
}
