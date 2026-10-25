package contractcli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mcpcontract "issueops/internal/contract/mcp"
	tc "issueops/internal/contract/toolconformance"
	"issueops/internal/testsupport"
)

const goodDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func okOutcome() ReplayOutcome {
	return ReplayOutcome{StateBeforeSHA256: goodDigest, StateAfterSHA256: goodDigest}
}

func TestNewConformanceDefaultsFailClosedWhenDependenciesAreMissing(t *testing.T) {
	c := NewConformance(ConformanceDependencies{})
	if got := c.deps.Catalog(); got != nil {
		t.Fatalf("default catalog=%v", got)
	}
	wantRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.deps.Root(); got != wantRoot {
		t.Fatalf("default root=%q want %q", got, wantRoot)
	}
	if _, err := c.deps.RunProcess(t.Context(), LiveRequest{}); err == nil || err.Error() != "live_runner_unavailable" {
		t.Fatalf("RunProcess err=%v", err)
	}
	if _, _, err := c.deps.LoadManifest(nil); err == nil || !strings.Contains(err.Error(), "manifest loader is not configured") {
		t.Fatalf("LoadManifest err=%v", err)
	}
	if _, err := c.deps.LoadRegressionFixture("x"); err == nil || !strings.Contains(err.Error(), "fixture loader is not configured") {
		t.Fatalf("LoadRegressionFixture err=%v", err)
	}
	if _, err := c.deps.ReplayRegression(tc.RegressionFixture{}, nil, ""); err == nil || !strings.Contains(err.Error(), "replay is not configured") {
		t.Fatalf("ReplayRegression err=%v", err)
	}
	if err := c.deps.ServeProbe(t.Context(), nil, nil, mcpcontract.ConformanceProbeConfig{}); err == nil || !strings.Contains(err.Error(), "probe is not configured") {
		t.Fatalf("ServeProbe err=%v", err)
	}
}

func TestConformanceRunRejectsMissingAndUnknownSubcommandsAndDispatches(t *testing.T) {
	c := NewConformance(ConformanceDependencies{})
	if err := c.Run(nil); err == nil || err.Error() != "missing conformance subcommand" {
		t.Fatalf("missing: %v", err)
	}
	if err := c.Run([]string{"bogus"}); err == nil || err.Error() != `unknown conformance subcommand "bogus" (want baseline|live|replay|serve)` {
		t.Fatalf("unknown: %v", err)
	}
	// Each known subcommand reaches its own handler (distinguished by its error).
	for sub, want := range map[string]string{
		"replay": "fixture_required",
		"serve":  "fixture-id, result-file, and run-token are required",
		"live":   "live_opt_in_required",
	} {
		t.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "")
		if err := c.Run([]string{sub}); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Fatalf("%s: err=%v want %q", sub, err, want)
		}
	}
	c = NewConformance(ConformanceDependencies{EvaluateBaseline: func() (int, bool, error) { return 3, true, nil }, Root: func() string { return t.TempDir() }})
	out := testsupport.CaptureStdout(t, func() error { return c.Run([]string{"baseline"}) })
	if out != "cases=3 ok=true\n" {
		t.Fatalf("baseline out=%q", out)
	}
}

func TestReplayRegressionFixtureMapsOutcomeAndPropagatesFailures(t *testing.T) {
	loadErr := errors.New("load boom")
	replayErr := errors.New("replay boom")
	diags := []tc.Diagnostic{{Code: tc.UnknownKey, Path: "/x"}}
	var gotFixture tc.RegressionFixture
	var gotState string
	deps := ConformanceDependencies{
		LoadRegressionFixture: func(path string) (tc.RegressionFixture, error) {
			if path == "bad" {
				return tc.RegressionFixture{}, loadErr
			}
			return tc.RegressionFixture{FixtureID: path}, nil
		},
		ReplayRegression: func(f tc.RegressionFixture, _ []tc.ToolDescriptor, state string) (tc.ReplayResult, error) {
			gotFixture, gotState = f, state
			switch f.FixtureID {
			case "replay-error":
				return tc.ReplayResult{}, replayErr
			case "not-ok":
				return tc.ReplayResult{OK: false}, nil
			}
			return tc.ReplayResult{OK: true, HandlerCalls: 2, StateBeforeSHA256: "a", StateAfterSHA256: "b", Classification: tc.UnknownKey, Diagnostics: diags, FinalResult: map[string]any{"k": "v"}}, nil
		},
	}
	c := NewConformance(deps)
	if _, err := c.replayRegressionFixture(t.Context(), "bad", "s"); !errors.Is(err, loadErr) {
		t.Fatalf("load err=%v", err)
	}
	if _, err := c.replayRegressionFixture(t.Context(), "replay-error", "s"); !errors.Is(err, replayErr) {
		t.Fatalf("replay err=%v", err)
	}
	if _, err := c.replayRegressionFixture(t.Context(), "not-ok", "s"); err == nil || err.Error() != "replay_expectation_failed" {
		t.Fatalf("not ok err=%v", err)
	}
	got, err := c.replayRegressionFixture(t.Context(), "good", "state-dir")
	if err != nil {
		t.Fatal(err)
	}
	want := ReplayOutcome{HandlerCalls: 2, StateBeforeSHA256: "a", StateAfterSHA256: "b", Classification: tc.UnknownKey, Diagnostics: diags, FinalResult: map[string]any{"k": "v"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
	if gotFixture.FixtureID != "good" || gotState != "state-dir" {
		t.Fatalf("replay inputs fixture=%q state=%q", gotFixture.FixtureID, gotState)
	}
}

func TestEvaluateSyntheticBaselineComparesExpectedClassifications(t *testing.T) {
	catalog := func() []mcpcontract.Tool {
		return []mcpcontract.Tool{{Name: "src", InputSchema: map[string]any{
			"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}},
		}}}
	}
	fixtures := []tc.Fixture{{ID: "f", SourceTool: "src", ExpectedArguments: map[string]any{"value": "x"}}}
	build := func(cases []tc.BaselineCase, loadErr error) *Conformance {
		return NewConformance(ConformanceDependencies{
			Catalog: catalog,
			LoadManifest: func([]tc.ToolDescriptor) ([]tc.Fixture, []tc.BaselineCase, error) {
				return fixtures, cases, loadErr
			},
		})
	}
	good := []tc.BaselineCase{
		{FixtureID: "f", ExpectedClassification: tc.ExactValid, Arguments: map[string]any{"value": "x"}},
		{FixtureID: "f", ExpectedClassification: tc.ValidButSemanticallyDifferent, Arguments: map[string]any{"value": "y"}},
	}
	count, ok, err := build(good, nil).evaluateSyntheticBaseline()
	if err != nil || count != 2 || !ok {
		t.Fatalf("good: count=%d ok=%t err=%v", count, ok, err)
	}
	bad := append([]tc.BaselineCase{}, good...)
	bad[1].ExpectedClassification = tc.ExactValid
	count, ok, err = build(bad, nil).evaluateSyntheticBaseline()
	if err != nil || count != 2 || ok {
		t.Fatalf("mismatch must fail the baseline: count=%d ok=%t err=%v", count, ok, err)
	}
	if _, _, err := build(good, errors.New("manifest boom")).evaluateSyntheticBaseline(); err == nil || err.Error() != "manifest boom" {
		t.Fatalf("load err=%v", err)
	}
}

func baselineConformance(root string, eval func() (int, bool, error), replay func(context.Context, string, string) (ReplayOutcome, error)) *Conformance {
	return NewConformance(ConformanceDependencies{Root: func() string { return root }, EvaluateBaseline: eval, Replay: replay})
}

func writeRegression(t *testing.T, root string, names ...string) {
	t.Helper()
	dir := regressionDirectory(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBaselineReportFailsWhenAnyRegressionReplayIsUnsafe(t *testing.T) {
	cases := map[string]struct {
		outcome ReplayOutcome
		err     error
	}{
		"replay error":       {err: errors.New("boom")},
		"handler was called": {outcome: ReplayOutcome{HandlerCalls: 1, StateBeforeSHA256: goodDigest, StateAfterSHA256: goodDigest}},
		"empty digest":       {outcome: ReplayOutcome{}},
		"state changed":      {outcome: ReplayOutcome{StateBeforeSHA256: goodDigest, StateAfterSHA256: "other"}},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeRegression(t, root, "one.json")
			c := baselineConformance(root, func() (int, bool, error) { return 4, true, nil }, func(context.Context, string, string) (ReplayOutcome, error) {
				return tt.outcome, tt.err
			})
			report, err := c.evaluateBaselineReport()
			if err != nil {
				t.Fatal(err)
			}
			if report.OK || report.Gate.Decision != tc.GateInconclusive || report.CaseCount != 5 {
				t.Fatalf("report=%+v", report)
			}
		})
	}
	root := t.TempDir()
	writeRegression(t, root, "one.json")
	c := baselineConformance(root, func() (int, bool, error) { return 4, true, nil }, func(context.Context, string, string) (ReplayOutcome, error) { return okOutcome(), nil })
	report, err := c.evaluateBaselineReport()
	if err != nil || !report.OK || report.Gate.Decision != tc.GateBaselinePassed || report.CaseCount != 5 || report.Counts.Completed != 5 {
		t.Fatalf("safe replay report=%+v err=%v", report, err)
	}
}

func TestBaselineReportPropagatesBaselineAndDirectoryErrors(t *testing.T) {
	boom := errors.New("eval boom")
	c := baselineConformance(t.TempDir(), func() (int, bool, error) { return 0, false, boom }, nil)
	if _, err := c.evaluateBaselineReport(); !errors.Is(err, boom) {
		t.Fatalf("eval err=%v", err)
	}
	// A regressions path whose parent is a regular file is not "absent": it must surface.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "adapter"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "adapter", "toolconformance"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	c = baselineConformance(root, func() (int, bool, error) { return 1, true, nil }, nil)
	if _, err := c.evaluateBaselineReport(); err == nil {
		t.Fatal("unreadable regression directory accepted")
	}
}

func TestRegressionFixturesIgnoresDirectoriesAndNonJSON(t *testing.T) {
	root := t.TempDir()
	writeRegression(t, root, "b.json", "a.json", "notes.txt")
	if err := os.Mkdir(filepath.Join(regressionDirectory(root), "sub.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := regressionFixtures(regressionDirectory(root))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range got {
		names = append(names, filepath.Base(p))
	}
	if !reflect.DeepEqual(names, []string{"a.json", "b.json"}) {
		t.Fatalf("names=%v", names)
	}
}

func TestRunConformanceBaselineOutputsAndErrors(t *testing.T) {
	c := baselineConformance(t.TempDir(), func() (int, bool, error) { return 7, true, nil }, nil)
	if err := c.runConformanceBaseline([]string{"--nope"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	out := testsupport.CaptureStdout(t, func() error { return c.runConformanceBaseline(nil) })
	if out != "cases=7 ok=true\n" {
		t.Fatalf("text out=%q", out)
	}
	out = testsupport.CaptureStdout(t, func() error { return c.runConformanceBaseline([]string{"--json"}) })
	var report tc.BenchmarkReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("json out %q: %v", out, err)
	}
	if !report.OK || report.CaseCount != 7 || report.RunID != "deterministic-baseline" {
		t.Fatalf("report=%+v", report)
	}
	boom := errors.New("eval boom")
	c = baselineConformance(t.TempDir(), func() (int, bool, error) { return 0, false, boom }, nil)
	if err := c.runConformanceBaseline(nil); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	printErr := errors.New("print boom")
	old := printJSON
	printJSON = func(any) error { return printErr }
	t.Cleanup(func() { printJSON = old })
	c = baselineConformance(t.TempDir(), func() (int, bool, error) { return 1, true, nil }, nil)
	if err := c.runConformanceBaseline([]string{"--json"}); !errors.Is(err, printErr) {
		t.Fatalf("print err=%v", err)
	}
}

func TestParseLiveRequestValidatesFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--nope"}, "flag provided but not defined"},
		{"profile", []string{"--profile", "wild"}, `invalid profile "wild"`},
		{"target", []string{"--target-completed", "5"}, "invalid target-completed 5"},
		{"attempts low", []string{"--max-attempts-per-case", "0"}, "invalid max-attempts-per-case 0"},
		{"attempts high", []string{"--max-attempts-per-case", "4"}, "invalid max-attempts-per-case 4"},
		{"empty hosts", []string{"--hosts", " , "}, "hosts_required"},
		{"empty model", []string{"--model", ""}, "empty value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseLiveRequest(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v want %q", err, tt.want)
			}
		})
	}
	req, jsonOut, err := parseLiveRequest([]string{"--hosts", " codex , ,claude", "--json", "--target-completed", "20", "--max-attempts-per-case", "1", "--evidence-dir", "ev"})
	if err != nil || !jsonOut {
		t.Fatalf("err=%v json=%t", err, jsonOut)
	}
	want := LiveRequest{Hosts: []string{"codex", "claude"}, Models: nil, Profile: "clean", TargetCompleted: 20, MaxAttemptsPerCase: 1, EvidenceDir: "ev"}
	if !reflect.DeepEqual(req, want) {
		t.Fatalf("req=%+v want=%+v", req, want)
	}
}

func liveReport(runID string, ok bool, decision tc.GateDecision) tc.BenchmarkReport {
	return tc.BenchmarkReport{OK: ok, SchemaVersion: tc.ReportSchemaVersion, RunID: runID, Profile: "clean", Gate: tc.GateReport{Decision: decision}, Hosts: []tc.HostReport{}, Warnings: []string{}}
}

func TestRunConformanceLiveStopsOnEachFailureBeforeOrAfterProcess(t *testing.T) {
	t.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "1")
	root := t.TempDir()
	calls := 0
	run := func(report tc.BenchmarkReport, err error) func(context.Context, LiveRequest) (tc.BenchmarkReport, error) {
		return func(context.Context, LiveRequest) (tc.BenchmarkReport, error) { calls++; return report, err }
	}
	newC := func(baselineOK bool, evalErr error, proc func(context.Context, LiveRequest) (tc.BenchmarkReport, error)) *Conformance {
		return NewConformance(ConformanceDependencies{
			Root: func() string { return root }, RunProcess: proc,
			EvaluateBaseline: func() (int, bool, error) { return 1, baselineOK, evalErr },
		})
	}
	if err := newC(true, nil, nil).runConformanceLive([]string{"--profile", "bad"}); err == nil || !strings.Contains(err.Error(), "invalid profile") {
		t.Fatalf("parse err=%v", err)
	}
	for name, c := range map[string]*Conformance{
		"baseline not ok": newC(false, nil, run(liveReport("r", true, tc.GateDeferHardening), nil)),
		"baseline error":  newC(true, errors.New("x"), run(liveReport("r", true, tc.GateDeferHardening), nil)),
	} {
		if err := c.runConformanceLive(nil); err == nil || !strings.HasPrefix(err.Error(), "baseline_failed_before_live: ") {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("process ran despite failing baseline: %d", calls)
	}
	missing := filepath.Join(root, "missing.json")
	if err := newC(true, nil, run(liveReport("r", true, tc.GateDeferHardening), nil)).runConformanceLive([]string{"--resume-report", missing}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resume err=%v", err)
	}
	procErr := errors.New("proc boom")
	if err := newC(true, nil, run(tc.BenchmarkReport{}, procErr)).runConformanceLive(nil); !errors.Is(err, procErr) {
		t.Fatalf("proc err=%v", err)
	}
	if err := newC(true, nil, run(liveReport("r", true, tc.GateDeferHardening), nil)).runConformanceLive([]string{"--evidence-dir", "../escape"}); err == nil || err.Error() != "evidence_dir_outside_issueops_root" {
		t.Fatalf("outside err=%v", err)
	}
	// An inconclusive report is still persisted, but the command fails.
	stdout := testsupport.CaptureStdout(t, func() error {
		err := newC(true, nil, run(liveReport("inconclusive-run", false, tc.GateInconclusive), nil)).runConformanceLive(nil)
		if err == nil || err.Error() != "conformance_inconclusive" {
			t.Fatalf("inconclusive err=%v", err)
		}
		return nil
	})
	if !strings.Contains(stdout, "gate=inconclusive") || !strings.Contains(stdout, filepath.Join(".issueops/evidence/tool-conformance", "inconclusive-run", "report.json")) {
		t.Fatalf("stdout=%q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".issueops/evidence/tool-conformance/inconclusive-run/report.json")); err != nil {
		t.Fatalf("report not persisted: %v", err)
	}
}

func TestRunConformanceLiveJSONPrintsReportWithEvidencePath(t *testing.T) {
	t.Setenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE", "1")
	root := t.TempDir()
	c := NewConformance(ConformanceDependencies{
		Root:             func() string { return root },
		EvaluateBaseline: func() (int, bool, error) { return 1, true, nil },
		RunProcess: func(context.Context, LiveRequest) (tc.BenchmarkReport, error) {
			return liveReport("json-run", true, tc.GateDeferHardening), nil
		},
	})
	out := testsupport.CaptureStdout(t, func() error { return c.runConformanceLive([]string{"--json"}) })
	var got tc.BenchmarkReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("out=%q: %v", out, err)
	}
	if want := filepath.Join(".issueops/evidence/tool-conformance", "json-run", "report.json"); got.Evidence.ReportPath != want {
		t.Fatalf("report path=%q want %q", got.Evidence.ReportPath, want)
	}
	printErr := errors.New("print boom")
	old := printJSON
	printJSON = func(any) error { return printErr }
	t.Cleanup(func() { printJSON = old })
	if err := c.runConformanceLive([]string{"--json"}); !errors.Is(err, printErr) {
		t.Fatalf("print err=%v", err)
	}
}

func TestLoadBenchmarkReportRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := loadBenchmarkReport(filepath.Join(dir, "none.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := loadBenchmarkReport(write("bad.json", "{not json")); err == nil {
		t.Fatal("malformed json accepted")
	}
	if _, err := loadBenchmarkReport(write("old.json", `{"schema_version":1,"gate":{"decision":"baseline_passed"}}`)); err == nil || err.Error() != "unsupported report schema version 1" {
		t.Fatalf("old schema: %v", err)
	}
	got, err := loadBenchmarkReport(write("ok.json", `{"schema_version":2,"run_id":"r","gate":{"decision":"baseline_passed"}}`))
	if err != nil || got.RunID != "r" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func hardeningReport(signature string, ids ...string) tc.BenchmarkReport {
	var cases []tc.EpisodeReport
	for i, id := range ids {
		cases = append(cases, tc.EpisodeReport{
			Status: tc.EpisodeCompleted, Host: "codex", HostVersion: "1", RequestedModel: "req", FixtureID: "empty_object", Attempt: i + 1,
			EvidenceID: id, Classification: tc.Classification(tc.UnknownKey), DiagnosticSignature: signature,
			Diagnostics:        []tc.Diagnostic{{Code: tc.UnknownKey, Path: "/x"}},
			CanonicalArguments: map[string]any{"x": true},
		})
	}
	r := liveReport("harden", true, tc.GateAuthorizeHardening)
	r.Gate.ConfirmedSignature = signature
	r.Hosts = []tc.HostReport{{Host: "codex", Cases: cases}}
	return r
}

func TestPersistLiveReportWritesPrivateReportAndCandidateForAuthorizedHardening(t *testing.T) {
	root := t.TempDir()
	c := newTestConformance(ConformanceDependencies{Root: func() string { return root }})
	report := hardeningReport("0123456789abcdef", goodDigest, strings.Repeat("a", 64))
	req := LiveRequest{EvidenceDir: ".issueops/evidence/tool-conformance"}
	if err := c.persistLiveReport(&report, req); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".issueops/evidence/tool-conformance/harden")
	for _, name := range []string{"report.json", "candidate-regression.json"} {
		info, err := os.Stat(filepath.Join(runDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode=%v", name, info.Mode().Perm())
		}
	}
	if info, _ := os.Stat(runDir); info.Mode().Perm() != 0o700 {
		t.Fatalf("run dir mode=%v", info.Mode().Perm())
	}
	if want := filepath.Join(".issueops/evidence/tool-conformance/harden", "candidate-regression.json"); report.Evidence.CandidateRegressionPath != want {
		t.Fatalf("candidate path=%q", report.Evidence.CandidateRegressionPath)
	}
	if want := "internal/adapter/toolconformance/testdata/regressions/codex-empty_object-0123456789ab.json"; report.Evidence.TrackedFixturePath != want {
		t.Fatalf("tracked=%q", report.Evidence.TrackedFixturePath)
	}
	var cand tc.RegressionFixture
	data, err := os.ReadFile(filepath.Join(runDir, "candidate-regression.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cand); err != nil {
		t.Fatal(err)
	}
	if cand.ModelLabel != "req" || cand.ExpectedDiagnosticSignature != "0123456789abcdef" || !reflect.DeepEqual(cand.ConfirmedEvidenceIDs, []string{goodDigest, strings.Repeat("a", 64)}) {
		t.Fatalf("candidate=%+v", cand)
	}
	// Directory entries other than the two reports (temp files) must not leak.
	entries, _ := os.ReadDir(runDir)
	if len(entries) != 2 {
		t.Fatalf("entries=%v", entries)
	}
}

func TestPersistLiveReportRejectsEvidenceDirEscapeAndUnwritableRoot(t *testing.T) {
	root := t.TempDir()
	c := newTestConformance(ConformanceDependencies{Root: func() string { return root }})
	r := liveReport("x", true, tc.GateDeferHardening)
	for _, dir := range []string{"..", "../out", filepath.Join(filepath.Dir(root), "elsewhere")} {
		if err := c.persistLiveReport(&r, LiveRequest{EvidenceDir: dir}); err == nil || err.Error() != "evidence_dir_outside_issueops_root" {
			t.Fatalf("dir %q err=%v", dir, err)
		}
	}
	// An absolute evidence dir inside the root is accepted as is.
	inside := filepath.Join(root, "abs-evidence")
	if err := c.persistLiveReport(&r, LiveRequest{EvidenceDir: inside}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inside, "x", "report.json")); err != nil {
		t.Fatal(err)
	}
	// Evidence root that is a regular file cannot host a run directory.
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.persistLiveReport(&r, LiveRequest{EvidenceDir: "file"}); err == nil {
		t.Fatal("file evidence root accepted")
	}
}

func TestBuildCandidateRegressionRejectsUnprovableEvidence(t *testing.T) {
	sig := "0123456789abcdef"
	c := newTestConformance(ConformanceDependencies{})
	if _, _, err := c.buildCandidateRegression(hardeningReport(sig, goodDigest, "not-a-digest")); err == nil || err.Error() != "confirmed_evidence_id_invalid" {
		t.Fatalf("bad evidence id: %v", err)
	}
	unknown := hardeningReport(sig, goodDigest, strings.Repeat("b", 64))
	for i := range unknown.Hosts[0].Cases {
		unknown.Hosts[0].Cases[i].FixtureID = "no_such_fixture"
	}
	if _, _, err := c.buildCandidateRegression(unknown); err == nil || err.Error() != "confirmed_fixture_missing" {
		t.Fatalf("unknown fixture: %v", err)
	}
	boom := errors.New("manifest boom")
	failing := NewConformance(ConformanceDependencies{LoadManifest: func([]tc.ToolDescriptor) ([]tc.Fixture, []tc.BaselineCase, error) { return nil, nil, boom }})
	if _, _, err := failing.buildCandidateRegression(hardeningReport(sig, goodDigest, strings.Repeat("b", 64))); !errors.Is(err, boom) {
		t.Fatalf("manifest err=%v", err)
	}
	// With no observed model the requested model labels the candidate; with one, the observed wins.
	rep := hardeningReport(sig, goodDigest, strings.Repeat("b", 64))
	rep.Hosts[0].Cases[0].ObservedModel = "obs"
	cand, _, err := c.buildCandidateRegression(rep)
	if err != nil || cand.ModelLabel != "obs" {
		t.Fatalf("cand=%+v err=%v", cand, err)
	}
}

func TestConfirmedEpisodeMatchesIgnoresIncompleteAndOtherSignatures(t *testing.T) {
	sig := "sig"
	ep := func(status tc.EpisodeStatus, fixture, s string) tc.EpisodeReport {
		return tc.EpisodeReport{Status: status, FixtureID: fixture, DiagnosticSignature: s}
	}
	report := tc.BenchmarkReport{Hosts: []tc.HostReport{{Host: "h", Cases: []tc.EpisodeReport{
		ep(tc.EpisodeIncomplete, "a", sig), ep(tc.EpisodeIncomplete, "a", sig),
		ep(tc.EpisodeCompleted, "a", "other"), ep(tc.EpisodeCompleted, "a", "other"),
		ep(tc.EpisodeCompleted, "b", sig), ep(tc.EpisodeCompleted, "b", sig), ep(tc.EpisodeCompleted, "b", sig),
	}}}}
	got := confirmedEpisodeMatches(report, sig)
	if len(got) != 3 || got[0].FixtureID != "b" {
		t.Fatalf("got=%+v", got)
	}
	if got := confirmedEpisodeMatches(report, "none"); len(got) != 0 {
		t.Fatalf("no-match=%+v", got)
	}
}

func TestSafeRunIDFiltersUnsafeCharacters(t *testing.T) {
	for in, want := range map[string]string{
		"Run_1.2-x":    "Run_1.2-x",
		"../../etc":    "....etc",
		"a/b c\\d\x00": "abcd",
		"///":          "run",
		"":             "run",
		"한글":           "run",
	} {
		if got := safeRunID(in); got != want {
			t.Fatalf("safeRunID(%q)=%q want %q", in, got, want)
		}
	}
}

func TestWritePrivateJSONFileCreatesPrivateAtomicFileAndReportsErrors(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "out.json")
	if err := writePrivateJSONFile(target, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v", info.Mode().Perm())
	}
	if err := writePrivateJSONFile(target, map[string]int{"a": 2}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(target)
	if !strings.Contains(string(data), `"a": 2`) {
		t.Fatalf("overwrite data=%q", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Fatalf("temp file leaked: %v", entries)
	}
	if err := writePrivateJSONFile(target, func() {}); err == nil {
		t.Fatal("unmarshalable value accepted")
	}
	if err := writePrivateJSONFile(filepath.Join(target, "child.json"), 1); err == nil {
		t.Fatal("parent that is a file accepted")
	}
	// Destination that is a non-empty directory makes the final rename fail and cleans the temp file.
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateJSONFile(blocked, 1); err == nil {
		t.Fatal("rename over directory accepted")
	}
	entries, _ = os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".report-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestSmallHelpers(t *testing.T) {
	if !validSHA256Text(goodDigest) {
		t.Fatal("valid digest rejected")
	}
	for _, bad := range []string{"", "abc", goodDigest[:63], goodDigest + "0", strings.Repeat("z", 64)} {
		if validSHA256Text(bad) {
			t.Fatalf("%q accepted", bad)
		}
	}
	if firstN("abcdef", 3) != "abc" || firstN("ab", 3) != "ab" || firstN("abc", 3) != "abc" {
		t.Fatal("firstN")
	}
	var s stringSlice
	if err := s.Set(""); err == nil {
		t.Fatal("empty value accepted")
	}
	_ = s.Set("a")
	_ = s.Set("b")
	if s.String() != "a,b" {
		t.Fatalf("String=%q", s.String())
	}
	if got := splitNonEmpty(" a, ,b ,,"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("split=%v", got)
	}
}

func TestSourceSchemaReturnsNilForUnknownOrUncloneableSchemas(t *testing.T) {
	c := NewConformance(ConformanceDependencies{Catalog: func() []mcpcontract.Tool {
		return []mcpcontract.Tool{{Name: "bad", InputSchema: map[string]any{"f": func() {}}}, {Name: "good", InputSchema: map[string]any{"type": "object"}}}
	}})
	if c.sourceSchema("missing") != nil || c.sourceSchema("bad") != nil {
		t.Fatal("expected nil schema")
	}
	if got := c.sourceSchema("good"); !reflect.DeepEqual(got, map[string]any{"type": "object"}) {
		t.Fatalf("good=%v", got)
	}
	if _, err := cloneJSONMap(map[string]any{"c": make(chan int)}); err == nil {
		t.Fatal("unmarshalable map cloned")
	}
}

func TestRunConformanceReplayReportsUnsafeOutcomesAndPrintsJSON(t *testing.T) {
	newC := func(o ReplayOutcome, err error) *Conformance {
		return NewConformance(ConformanceDependencies{Replay: func(context.Context, string, string) (ReplayOutcome, error) { return o, err }})
	}
	if err := newC(okOutcome(), nil).runConformanceReplay([]string{"--nope"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	if err := newC(okOutcome(), nil).runConformanceReplay(nil); err == nil || !strings.HasPrefix(err.Error(), "fixture_required: ") {
		t.Fatalf("missing fixture: %v", err)
	}
	boom := errors.New("replay boom")
	if err := newC(ReplayOutcome{}, boom).runConformanceReplay([]string{"--fixture", "f"}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	for name, o := range map[string]ReplayOutcome{
		"handler called": {HandlerCalls: 1, StateBeforeSHA256: goodDigest, StateAfterSHA256: goodDigest},
		"empty digest":   {},
		"state changed":  {StateBeforeSHA256: goodDigest, StateAfterSHA256: "x"},
	} {
		if err := newC(o, nil).runConformanceReplay([]string{"--fixture", "f"}); err == nil || err.Error() != "replay_failed" {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	out := testsupport.CaptureStdout(t, func() error {
		return newC(okOutcome(), nil).runConformanceReplay([]string{"--fixture", "f", "--json"})
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil || got["ok"] != true || got["state_digest"] != goodDigest {
		t.Fatalf("out=%q err=%v", out, err)
	}
	printErr := errors.New("print boom")
	old := printJSON
	printJSON = func(any) error { return printErr }
	t.Cleanup(func() { printJSON = old })
	if err := newC(okOutcome(), nil).runConformanceReplay([]string{"--fixture", "f", "--json"}); !errors.Is(err, printErr) {
		t.Fatalf("print err=%v", err)
	}
}

func TestRunConformanceServePassesSelectedFixtureToProbe(t *testing.T) {
	fixtures := []tc.Fixture{
		{ID: "other", ProbeTool: "p0", SourceTool: "s0"},
		{ID: "want", ProbeTool: "probe", SourceTool: "src", SchemaSHA256: "sha", ExpectedArguments: map[string]any{"k": 1}},
	}
	var got mcpcontract.ConformanceProbeConfig
	probeErr := errors.New("probe boom")
	var loadErr error
	c := NewConformance(ConformanceDependencies{
		Catalog: func() []mcpcontract.Tool {
			return []mcpcontract.Tool{{Name: "src", InputSchema: map[string]any{"type": "object"}}}
		},
		LoadManifest: func([]tc.ToolDescriptor) ([]tc.Fixture, []tc.BaselineCase, error) { return fixtures, nil, loadErr },
		ServeProbe: func(_ context.Context, _ io.Reader, _ io.Writer, cfg mcpcontract.ConformanceProbeConfig) error {
			got = cfg
			return probeErr
		},
	})
	args := []string{"--fixture-id", "want", "--result-file", "/r.json", "--run-token", "tok"}
	if err := c.runConformanceServe(args); !errors.Is(err, probeErr) {
		t.Fatalf("serve err=%v", err)
	}
	want := mcpcontract.ConformanceProbeConfig{FixtureID: "want", ProbeTool: "probe", Schema: map[string]any{"type": "object"}, SchemaSHA: "sha", ExpectedArguments: map[string]any{"k": 1}, ResultPath: "/r.json", RunToken: "tok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cfg=%+v want=%+v", got, want)
	}
	if err := c.runConformanceServe([]string{"--fixture-id", "nope", "--result-file", "/r", "--run-token", "t"}); err == nil || err.Error() != "unknown fixture nope" {
		t.Fatalf("unknown: %v", err)
	}
	loadErr = errors.New("manifest boom")
	if err := c.runConformanceServe(args); !errors.Is(err, loadErr) {
		t.Fatalf("load err=%v", err)
	}
}
