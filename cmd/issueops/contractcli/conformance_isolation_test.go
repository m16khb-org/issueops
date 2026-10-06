package contractcli

import (
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	fixtureapp "issueops/internal/application/toolconformance"

	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/toolconformance"
	domain "issueops/internal/domain/toolconformance"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"issueops/internal/adapter/toolconformance"
	mcpcontract "issueops/internal/contract/mcp"
)

func TestConformanceInstancesKeepCatalogAndRegressionRoot(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	dir := regressionDirectory(roots[0])
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Only the first instance sees this rejected fixture. A shared root would
	// incorrectly turn one of the independently evaluated baselines green/red.
	if err := os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	instances := make([]*Conformance, 2)
	for i := range instances {
		catalog := append(mcpcatalog.AdvertisedTools(), mcpcontract.Tool{Name: roots[i], InputSchema: map[string]any{"type": "object"}})
		instances[i] = NewConformance(ConformanceDependencies{
			Catalog:               func() []mcpcontract.Tool { return catalog },
			Root:                  func() string { return roots[i] },
			LoadManifest:          (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).LoadManifest,
			LoadRegressionFixture: (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).LoadRegressionFixture,
			ReplayRegression:      (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).ReplayRegression,
		})
	}
	var wg sync.WaitGroup
	for i, instance := range instances {
		wg.Go(func() {
			for range 3 {
				if instance.sourceSchema(roots[i]) == nil || instance.sourceSchema(roots[1-i]) != nil {
					t.Errorf("instance %d used another catalog", i)
				}
				report, err := instance.evaluateBaselineReport()
				if err != nil || report.OK != (i == 1) || report.CaseCount == 0 {
					t.Errorf("instance %d baseline=%+v err=%v", i, report, err)
				}
			}
		})
	}
	wg.Wait()
}

func TestConformanceInstanceReplaysActualInvalidArgumentsBeforeEffects(t *testing.T) {
	runtime := newTestConformance(ConformanceDependencies{})
	fixtures, _, err := (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).LoadManifest(runtime.conformanceDescriptors())
	if err != nil {
		t.Fatal(err)
	}
	fixture := fixtures[0]
	raw := []byte(`{"requireUnique":true}`)
	classified, err := domain.Classify(contract.CallObservation{RawArguments: raw, CallCount: 1}, runtime.sourceSchema(fixture.SourceTool), fixture.ExpectedArguments)
	if err != nil {
		t.Fatal(err)
	}
	regression := contract.RegressionFixture{
		SchemaVersion: 1, FixtureID: fixture.ID, SourceTool: fixture.SourceTool, ProbeTool: fixture.ProbeTool,
		SourceSchemaSHA256: fixture.SchemaSHA256, Host: "claude", HostVersion: "test", ModelLabel: "test",
		CanonicalArguments: map[string]any{"requireUnique": true}, RawArgumentsSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)),
		ExpectedClassification: classified.Classification, ExpectedDiagnostics: classified.Diagnostics,
		ExpectedDiagnosticSignature: domain.DiagnosticSignature(classified.Classification, classified.Diagnostics),
		ConfirmedEvidenceIDs:        []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}, ExpectedHandlerCallCount: 0,
		ExpectedFinalResult: domain.InvalidToolArgumentsResult(fixture.SourceTool, classified.Diagnostics), ExpectedStateUnchanged: true,
	}
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := writePrivateJSONFile(path, regression); err != nil {
		t.Fatal(err)
	}
	outcome, err := runtime.replayFixture(path)
	if err != nil || outcome.HandlerCalls != 0 || outcome.StateBeforeSHA256 == "" || outcome.StateBeforeSHA256 != outcome.StateAfterSHA256 {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if err := runtime.Run([]string{"replay", "--fixture", path, "--json"}); err != nil {
		t.Fatal(err)
	}
	regression.ExpectedHandlerCallCount = 1
	if err := writePrivateJSONFile(path, regression); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run([]string{"replay", "--fixture", path}); err == nil {
		t.Fatal("unsafe regression accepted")
	}
}

func TestConformanceInstancesPersistLiveReportsInTheirOwnRoots(t *testing.T) {
	t.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "1")
	roots := []string{t.TempDir(), t.TempDir()}
	runtimes := make([]*Conformance, 2)
	for i := range runtimes {
		runtimes[i] = newTestConformance(ConformanceDependencies{
			Root: func() string { return roots[i] },
			RunProcess: func(_ context.Context, req LiveRequest) (contract.BenchmarkReport, error) {
				if len(req.Models) != 1 || req.Models[0] != fmt.Sprint(i) {
					return contract.BenchmarkReport{}, fmt.Errorf("wrong process dependencies: %v", req.Models)
				}
				return contract.BenchmarkReport{OK: true, SchemaVersion: contract.ReportSchemaVersion, RunID: fmt.Sprint(i), Profile: req.Profile, Gate: contract.GateReport{Decision: contract.GateDeferHardening}, Hosts: []contract.HostReport{}, Warnings: []string{}}, nil
			},
		})
	}
	var wg sync.WaitGroup
	for i, runtime := range runtimes {
		wg.Go(func() {
			if err := runtime.Run([]string{"live", "--model", fmt.Sprint(i), "--evidence-dir", "reports"}); err != nil {
				t.Error(err)
				return
			}
			path := filepath.Join(roots[i], "reports", fmt.Sprint(i), "report.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
				return
			}
			var report contract.BenchmarkReport
			if err := json.Unmarshal(data, &report); err != nil || report.RunID != fmt.Sprint(i) {
				t.Errorf("report=%+v err=%v", report, err)
			}
		})
	}
	wg.Wait()
	for i := range roots {
		if _, err := os.Stat(filepath.Join(roots[i], "reports", fmt.Sprint(1-i))); !os.IsNotExist(err) {
			t.Fatalf("report leaked into root %d: %v", i, err)
		}
	}
}
