package contractcli

import (
	fixturecontract "issueops/internal/contract/toolconformance"

	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	mcpcontract "issueops/internal/contract/mcp"
)

func TestConformanceBaselineFailsWithJSONWhenInjectedCaseFails(t *testing.T) {
	runtime := newTestConformance(ConformanceDependencies{
		EvaluateBaseline: func() (int, bool, error) { return 10, false, nil },
	})
	if err := runtime.runConformanceBaseline([]string{"--json"}); err == nil || err.Error() != "baseline_failed" {
		t.Fatalf("err=%v", err)
	}
}

func TestConformanceBaselineRunsRegressionFixturesInDeterministicOrderAndAllowsAbsentDirectory(t *testing.T) {
	root := t.TempDir()
	dir := regressionDirectory(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"z-last.json", "a-first.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var calls []string
	runtime := newTestConformance(ConformanceDependencies{
		Root: func() string { return root },
		Replay: func(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
			calls = append(calls, filepath.Base(fixturePath))
			if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("unchanged"), 0o600); err != nil {
				return ReplayOutcome{}, err
			}
			digest := sha256.Sum256([]byte("unchanged"))
			return ReplayOutcome{HandlerCalls: 0, StateBeforeSHA256: fmt.Sprintf("%x", digest), StateAfterSHA256: fmt.Sprintf("%x", digest)}, nil
		},
	})
	if err := runtime.runConformanceBaseline(nil); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a-first.json", "z-last.json"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	calls = nil
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := runtime.runConformanceBaseline(nil); err != nil {
		t.Fatalf("absent regression directory: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("absent directory replayed=%v", calls)
	}
}

func TestConformanceReplayUsesFakeHandlerAndUnchangedTemporaryStateDigest(t *testing.T) {
	called := 0
	runtime := newTestConformance(ConformanceDependencies{
		Replay: func(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
			called++
			if filepath.Base(fixturePath) != "fixture.json" {
				return ReplayOutcome{}, fmt.Errorf("unexpected fixture %s", fixturePath)
			}
			state := []byte("stable state")
			if err := os.WriteFile(filepath.Join(stateDir, "state.json"), state, 0o600); err != nil {
				return ReplayOutcome{}, err
			}
			digest := sha256.Sum256(state)
			return ReplayOutcome{HandlerCalls: 0, StateBeforeSHA256: fmt.Sprintf("%x", digest), StateAfterSHA256: fmt.Sprintf("%x", digest)}, nil
		},
	})
	if err := runtime.runConformanceReplay([]string{"--fixture", "fixture.json", "--json"}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("handler runs=%d", called)
	}
}

func TestConformanceLiveRequiresExplicitOptInBeforeInjectedProcess(t *testing.T) {
	old, had := os.LookupEnv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
	defer func() {
		if had {
			_ = os.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", old)
		} else {
			_ = os.Unsetenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
		}
	}()
	processCalls := 0
	runtime := newTestConformance(ConformanceDependencies{RunProcess: func(context.Context, LiveRequest) (fixturecontract.BenchmarkReport, error) {
		processCalls++
		return fixturecontract.BenchmarkReport{}, nil
	}})
	_ = os.Unsetenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
	if err := runtime.runConformanceLive([]string{"--hosts", "codex", "--model", "codex=default", "--profile", "clean", "--target-completed", "1", "--max-attempts-per-case", "3"}); err == nil || err.Error() != "live_opt_in_required" {
		t.Fatalf("err=%v", err)
	}
	if processCalls != 0 {
		t.Fatalf("process calls=%d", processCalls)
	}
}

func TestConformanceLivePassesFullyParsedFlagsToInjectedProcessAfterOptIn(t *testing.T) {
	old, had := os.LookupEnv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
	defer func() {
		if had {
			_ = os.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", old)
		} else {
			_ = os.Unsetenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
		}
	}()
	_ = os.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "1")
	root := t.TempDir()
	prior := fixturecontract.BenchmarkReport{
		OK: true, SchemaVersion: fixturecontract.ReportSchemaVersion, RunID: "prior",
		Profile: "context-pressure", Gate: fixturecontract.GateReport{Decision: fixturecontract.GateNeedsReproduction},
		Hosts: []fixturecontract.HostReport{}, Warnings: []string{},
	}
	priorPath := filepath.Join(root, "prior.json")
	if err := writePrivateJSONFile(priorPath, prior); err != nil {
		t.Fatal(err)
	}
	var got LiveRequest
	runtime := newTestConformance(ConformanceDependencies{
		Root: func() string { return root },
		RunProcess: func(_ context.Context, request LiveRequest) (fixturecontract.BenchmarkReport, error) {
			got = request
			return fixturecontract.BenchmarkReport{
				OK: true, SchemaVersion: fixturecontract.ReportSchemaVersion, RunID: "test-live",
				Profile: request.Profile, Gate: fixturecontract.GateReport{Decision: fixturecontract.GateDeferHardening},
				Hosts: []fixturecontract.HostReport{}, Warnings: []string{},
			}, nil
		},
	})
	args := []string{"--hosts", "codex,claude", "--model", "codex=default", "--model", "claude=test", "--profile", "context-pressure", "--only", "codex:empty_object", "--resume-report", priorPath, "--target-completed", "10", "--max-attempts-per-case", "2"}
	if err := runtime.runConformanceLive(args); err != nil {
		t.Fatal(err)
	}
	want := LiveRequest{Hosts: []string{"codex", "claude"}, Models: []string{"codex=default", "claude=test"}, Profile: "context-pressure", Only: "codex:empty_object", ResumeReport: priorPath, TargetCompleted: 10, MaxAttemptsPerCase: 2, EvidenceDir: ".issueops/evidence/tool-conformance", Previous: &prior}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestConformanceLiveDefaultsExcludeOmoAndExplicitSelectionIncludesIt(t *testing.T) {
	old, had := os.LookupEnv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
	defer func() {
		if had {
			_ = os.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", old)
		} else {
			_ = os.Unsetenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE")
		}
	}()
	_ = os.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "1")
	root := t.TempDir()
	requests := []LiveRequest{}
	runtime := newTestConformance(ConformanceDependencies{
		Root:             func() string { return root },
		EvaluateBaseline: func() (int, bool, error) { return 1, true, nil },
		RunProcess: func(_ context.Context, request LiveRequest) (fixturecontract.BenchmarkReport, error) {
			requests = append(requests, request)
			return fixturecontract.BenchmarkReport{
				OK: true, SchemaVersion: fixturecontract.ReportSchemaVersion, RunID: fmt.Sprintf("selection-%d", len(requests)),
				Profile: request.Profile, Gate: fixturecontract.GateReport{Decision: fixturecontract.GateDeferHardening},
				Hosts: []fixturecontract.HostReport{}, Warnings: []string{},
			}, nil
		},
	})

	if err := runtime.runConformanceLive(nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.runConformanceLive([]string{"--hosts", "omo", "--model", "omo=google/gemini-2.5-pro"}); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || !reflect.DeepEqual(requests[0].Hosts, []string{"codex", "claude"}) || !reflect.DeepEqual(requests[1].Hosts, []string{"omo"}) {
		t.Fatalf("live host selections = %#v", requests)
	}
}

func TestConformanceServeParsesRequiredFlags(t *testing.T) {
	runtime := newTestConformance(ConformanceDependencies{})
	if err := runtime.runConformanceServe(nil); err == nil {
		t.Fatal("serve missing flags accepted")
	}
	if err := runtime.runConformanceServe([]string{"--unknown"}); err == nil {
		t.Fatal("serve unknown flag accepted")
	}
}

func TestProductionCatalogDoesNotAdvertiseConformanceProbe(t *testing.T) {
	for _, tool := range testConformanceCatalog() {
		if len(tool.Name) >= len("harness_probe_") && tool.Name[:len("harness_probe_")] == "harness_probe_" {
			t.Fatalf("production catalog advertises probe %q", tool.Name)
		}
	}
}

func TestConformanceSourceSchemaCopiesConfiguredCatalogSource(t *testing.T) {
	source := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}}
	runtime := newTestConformance(ConformanceDependencies{Catalog: func() []mcpcontract.Tool { return []mcpcontract.Tool{{Name: "source", InputSchema: source}} }})
	copy := runtime.sourceSchema("source")
	copy["properties"].(map[string]any)["later"] = map[string]any{"type": "boolean"}
	if _, exists := source["properties"].(map[string]any)["later"]; exists {
		t.Fatalf("source schema mutated: %#v", source)
	}
}

func TestBuildCandidateRegressionRequiresRepeatedSignatureWithinOneHostFixture(t *testing.T) {
	runtime := newTestConformance(ConformanceDependencies{})
	signature := "0123456789abcdef"
	episode := func(host string, attempt int) fixturecontract.EpisodeReport {
		return fixturecontract.EpisodeReport{
			Status: "completed", Host: host, HostVersion: "test", ObservedModel: "test",
			FixtureID: "empty_object", Attempt: attempt, RawArgumentsSHA256: fmt.Sprintf("%064d", attempt), EvidenceID: fmt.Sprintf("%064x", attempt),
			Classification:      fixturecontract.Classification(fixturecontract.UnknownKey),
			Diagnostics:         []fixturecontract.Diagnostic{{Code: fixturecontract.UnknownKey, Path: "/requireUnique"}},
			DiagnosticSignature: signature,
			CanonicalArguments:  map[string]any{"requireUnique": true},
		}
	}
	report := fixturecontract.BenchmarkReport{
		Gate: fixturecontract.GateReport{Decision: fixturecontract.GateAuthorizeHardening, ConfirmedSignature: signature, ConfirmedCount: 2},
		Hosts: []fixturecontract.HostReport{
			{Host: "claude", Cases: []fixturecontract.EpisodeReport{episode("claude", 1)}},
			{Host: "codex", Cases: []fixturecontract.EpisodeReport{episode("codex", 1)}},
		},
	}
	if _, _, err := runtime.buildCandidateRegression(report); err == nil || err.Error() != "confirmed_signature_evidence_missing" {
		t.Fatalf("cross-target evidence accepted: %v", err)
	}

	report.Hosts[1].Cases = append(report.Hosts[1].Cases, episode("codex", 2))
	candidate, tracked, err := runtime.buildCandidateRegression(report)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Host != "codex" || len(candidate.ConfirmedEvidenceIDs) != 2 {
		t.Fatalf("candidate=%+v", candidate)
	}
	if want := "internal/adapter/toolconformance/testdata/regressions/codex-empty_object-0123456789ab.json"; tracked != want {
		t.Fatalf("tracked=%q want=%q", tracked, want)
	}
}
