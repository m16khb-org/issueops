package benchmarkcmd

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	statestore "issueops/internal/adapter/outbound/state"
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
)

// captureStdout runs fn while os.Stdout is redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()
	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// captureJSON replaces printJSON for the test and returns a pointer to the value handed to it.
func captureJSON(t *testing.T) *any {
	t.Helper()
	var got any
	original := printJSON
	printJSON = func(value any) error { got = value; return nil }
	t.Cleanup(func() { printJSON = original })
	return &got
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const consensusSamples = `{"samples":[
	{"sample_id":"s1","provenance":"judge-a","score":{"average_score":90,"passed":true}},
	{"sample_id":"s2","provenance":"judge-b","score":{"average_score":70,"passed":true}},
	{"sample_id":"s3","provenance":"judge-a","score":{"average_score":50,"passed":false}}
]}`

func TestBenchmarkConsensusAggregatesSamplesAsJSON(t *testing.T) {
	got := captureJSON(t)
	path := writeFile(t, "samples.json", consensusSamples)
	if err := testBenchmarkCommand().Run([]string{"consensus", "--samples", path, "--json"}); err != nil {
		t.Fatal(err)
	}
	verdict, ok := (*got).(benchmarkcontract.ConsensusVerdict)
	if !ok {
		t.Fatalf("printJSON received %T, want ConsensusVerdict", *got)
	}
	if verdict.Samples != 3 || !verdict.MajorityPassed || verdict.MedianAverageScore != 70 || verdict.ScoreMin != 50 || verdict.ScoreMax != 90 {
		t.Fatalf("verdict = %+v", verdict)
	}
	// provenance is de-duplicated in first-seen order.
	if len(verdict.Provenance) != 2 || verdict.Provenance[0] != "judge-a" || verdict.Provenance[1] != "judge-b" {
		t.Fatalf("provenance = %v", verdict.Provenance)
	}
}

func TestBenchmarkConsensusTextOutputReportsAgreementAndCaveat(t *testing.T) {
	path := writeFile(t, "samples.json", consensusSamples)
	var err error
	out := captureStdout(t, func() {
		err = testBenchmarkCommand().Run([]string{"consensus", "--samples", path})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "samples=3 majority_passed=true pass_agreement=0.6667 median=70.0000") {
		t.Fatalf("summary line missing: %q", out)
	}
	if !strings.Contains(out, "caveat: descriptive over the supplied samples") {
		t.Fatalf("caveat line missing: %q", out)
	}
}

func TestBenchmarkConsensusSurfacesInputErrors(t *testing.T) {
	cmd := testBenchmarkCommand()
	single := writeFile(t, "single.json", `{"samples":[{"sample_id":"s1","provenance":"p","score":{}}]}`)
	if err := cmd.Run([]string{"consensus", "--samples", single}); err == nil || !strings.Contains(err.Error(), "need at least 2 judge samples") {
		t.Fatalf("single sample err = %v", err)
	}
	dup := writeFile(t, "dup.json", `{"samples":[{"sample_id":"s","provenance":"p","score":{}},{"sample_id":"s","provenance":"p","score":{}}]}`)
	if err := cmd.Run([]string{"consensus", "--samples", dup}); err == nil || !strings.Contains(err.Error(), "duplicate sample_id") {
		t.Fatalf("duplicate err = %v", err)
	}
	if err := cmd.Run([]string{"consensus", "--samples", filepath.Join(t.TempDir(), "absent.json")}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file err = %v", err)
	}
	bad := writeFile(t, "bad.json", "{bad")
	if err := cmd.Run([]string{"consensus", "--samples", bad}); err == nil || !strings.Contains(err.Error(), "parse judge samples") {
		t.Fatalf("bad json err = %v", err)
	}
}

func TestBenchmarkSubcommandsReturnNilOnHelpFlagAndErrorOnUnknownFlag(t *testing.T) {
	cmd := testBenchmarkCommand()
	got := captureJSON(t)
	for _, sub := range []string{"run", "compare", "gate", "reliability", "consensus"} {
		var err error
		captureStdout(t, func() { err = cmd.Run([]string{sub, "-h"}) })
		if err != nil {
			t.Fatalf("%s -h err = %v, want nil", sub, err)
		}
		captureStdout(t, func() { err = cmd.Run([]string{sub, "--no-such-flag"}) })
		if err == nil || !strings.Contains(err.Error(), "no-such-flag") {
			t.Fatalf("%s unknown flag err = %v", sub, err)
		}
	}
	if *got != nil {
		t.Fatalf("help/flag errors must not reach the service or print results, got %v", *got)
	}
}

func TestBenchmarkRunWithoutConfiguredServiceFails(t *testing.T) {
	err := Command{}.Run([]string{"run"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v", err)
	}
	// unknown subcommands are reported before the missing-service check.
	if err := (Command{}).Run([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "unknown issueops benchmark subcommand") {
		t.Fatalf("err = %v", err)
	}
}

func TestBenchmarkCompareAndGateSurfaceMissingRunsAndPrintText(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	cmd := testBenchmarkCommand()
	if err := cmd.Run([]string{"compare", "--baseline", "ghost", "--candidate", "ghost2"}); err == nil {
		t.Fatal("compare of unsaved runs must fail")
	}
	candidateFile := writeFile(t, "c.json", `{"id":"c1","hypothesis":"h","target_dimensions":["intent_understanding"],"edit_surface":["cmd/**"]}`)
	if err := cmd.Run([]string{"gate", "--baseline", "ghost", "--candidate", "ghost2", "--candidate-file", candidateFile}); err == nil {
		t.Fatal("gate with unsaved runs must fail")
	}
	if err := cmd.Run([]string{"gate", "--baseline", "a", "--candidate", "b", "--candidate-file", ""}); err == nil || !strings.Contains(err.Error(), "candidate-file is required") {
		t.Fatalf("gate without candidate file err = %v", err)
	}

	baseline := benchmarkRunResult("base", 90, true)
	candidate := benchmarkRunResult("cand", 80, true)
	for _, run := range []benchmarkcontract.IssueOpsBenchmarkRunResult{baseline, candidate} {
		if err := saveBenchmarkRunForTest(statestore.StateDir(), run); err != nil {
			t.Fatal(err)
		}
	}

	var err error
	out := captureStdout(t, func() {
		err = cmd.Run([]string{"compare", "--baseline", "base", "--candidate", "cand"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "improved=false average_delta=-10.00 minimum_delta=-10.00 critical_failure_delta=0") {
		t.Fatalf("compare text = %q", out)
	}

	out = captureStdout(t, func() {
		err = cmd.Run([]string{"gate", "--baseline", "base", "--candidate", "cand", "--candidate-file", candidateFile, "--changed-path", "internal/x.go"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "keep_candidate=false") {
		t.Fatalf("regressed candidate must be discarded: %q", out)
	}
	for _, reason := range []string{"- discard: benchmark comparison regressed", "- discard: changed paths outside declared edit surface", "- discard: target dimensions regressed"} {
		if !strings.Contains(out, reason) {
			t.Fatalf("missing %q in %q", reason, out)
		}
	}
}

func TestBenchmarkRunTextOutputAndUnsupportedJudge(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	fixtures := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixtures, "f.json"), []byte(`{"id":"fixture-1","title":"t","user_prompt":"p","repo_context":"r","expected_issue":["IssueOps"],"expected_plan":["x"],"expected_tasks":["y"],"expected_tdd":["z"],"expected_subagents":["w"],"expected_pr":["v"],"critical_failures":["c"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := testBenchmarkCommand()
	if err := cmd.Run([]string{"run", "--fixtures", fixtures, "--judge", "bogus"}); err == nil || !strings.Contains(err.Error(), `unsupported issueops benchmark judge "bogus"`) {
		t.Fatalf("bogus judge err = %v", err)
	}
	var err error
	out := captureStdout(t, func() { err = cmd.Run([]string{"run", "--fixtures", fixtures}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "issueops-benchmark-") || !strings.Contains(out, " fixtures=1 average=") {
		t.Fatalf("run text = %q", out)
	}
	if ids := benchmarkRunIDs(t); len(ids) != 1 || !strings.HasPrefix(ids[0], "issueops-benchmark-") {
		t.Fatalf("saved runs = %v (a rejected judge must not persist a run)", ids)
	}
}
