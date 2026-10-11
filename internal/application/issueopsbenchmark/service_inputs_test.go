package issueopsbenchmark_test

import (
	"errors"
	"testing"

	app "issueops/internal/application/issueopsbenchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
	port "issueops/internal/port/issueopsbenchmark"
)

type inputFiles struct {
	port.Files
	candidate contract.IssueOpsAutoresearchCandidate
	outcomes  contract.RecordedOutcomes
	samples   []contract.JudgeSample
	err       error
	gotPath   string
}

func (f *inputFiles) ReadCandidate(p string) (contract.IssueOpsAutoresearchCandidate, error) {
	f.gotPath = p
	return f.candidate, f.err
}
func (f *inputFiles) ReadOutcomes(p string) (contract.RecordedOutcomes, error) {
	f.gotPath = p
	return f.outcomes, f.err
}
func (f *inputFiles) ReadSamples(p string) ([]contract.JudgeSample, error) {
	f.gotPath = p
	return f.samples, f.err
}

type mapRuns struct {
	port.Runs
	runs  map[string]contract.IssueOpsBenchmarkRunResult
	reads []string
}

func (m *mapRuns) Read(id string) (contract.IssueOpsBenchmarkRunResult, error) {
	m.reads = append(m.reads, id)
	r, ok := m.runs[id]
	if !ok {
		return contract.IssueOpsBenchmarkRunResult{}, errors.New("no run " + id)
	}
	return r, nil
}

func TestCompareFailsOnUnreadableRunWithoutReadingCandidateFirst(t *testing.T) {
	runs := &mapRuns{runs: map[string]contract.IssueOpsBenchmarkRunResult{"cand": {ID: "cand"}}}
	if _, err := (app.Service{Runs: runs}).Compare("missing", "cand"); err == nil || err.Error() != "no run missing" {
		t.Fatalf("baseline error = %v", err)
	}
	if len(runs.reads) != 1 {
		t.Fatalf("candidate must not be read after baseline failure: %v", runs.reads)
	}
	runs = &mapRuns{runs: map[string]contract.IssueOpsBenchmarkRunResult{"base": {ID: "base"}}}
	if _, err := (app.Service{Runs: runs}).Compare("base", "missing"); err == nil || err.Error() != "no run missing" {
		t.Fatalf("candidate error = %v", err)
	}
}

func TestGateDiscardsCandidateTouchingPathsOutsideEditSurface(t *testing.T) {
	files := &inputFiles{candidate: contract.IssueOpsAutoresearchCandidate{ID: "c1", EditSurface: []string{"internal/a/"}}}
	runs := &mapRuns{runs: map[string]contract.IssueOpsBenchmarkRunResult{
		"base": {ID: "base", AverageScore: 70, MinimumScore: 50},
		"cand": {ID: "cand", AverageScore: 80, MinimumScore: 60},
	}}
	svc := app.Service{Files: files, Runs: runs}
	got, err := svc.Gate("cand.json", "base", "cand", []string{"cmd/other.go"})
	if err != nil {
		t.Fatal(err)
	}
	if files.gotPath != "cand.json" || got.CandidateID != "c1" {
		t.Fatalf("path=%q result=%+v", files.gotPath, got)
	}
	if got.KeepCandidate || len(got.EditSurfaceViolations) == 0 {
		t.Fatalf("out-of-surface edit must be discarded: %+v", got)
	}
}

func TestGateStopsAtFirstInputError(t *testing.T) {
	sentinel := errors.New("candidate unreadable")
	runs := &mapRuns{}
	if _, err := (app.Service{Files: &inputFiles{err: sentinel}, Runs: runs}).Gate("c", "b", "n", nil); !errors.Is(err, sentinel) || len(runs.reads) != 0 {
		t.Fatalf("err=%v reads=%v", err, runs.reads)
	}
	runs = &mapRuns{runs: map[string]contract.IssueOpsBenchmarkRunResult{"b": {ID: "b"}}}
	if _, err := (app.Service{Files: &inputFiles{}, Runs: runs}).Gate("c", "b", "n", nil); err == nil || err.Error() != "no run n" {
		t.Fatalf("candidate run error = %v", err)
	}
	if _, err := (app.Service{Files: &inputFiles{}, Runs: &mapRuns{}}).Gate("c", "b", "n", nil); err == nil || err.Error() != "no run b" {
		t.Fatalf("baseline run error = %v", err)
	}
}

func TestReliabilityPropagatesReadAndDomainErrors(t *testing.T) {
	sentinel := errors.New("read failed")
	if _, err := (app.Service{Files: &inputFiles{err: sentinel}}).Reliability("p", 0.05); !errors.Is(err, sentinel) {
		t.Fatalf("read error = %v", err)
	}
	one := contract.RecordedOutcomes{Runs: []contract.RecordedRun{{RunID: "r1", Provenance: "p", Outcomes: map[string]bool{"f": true}}}}
	if _, err := (app.Service{Files: &inputFiles{outcomes: one}}).Reliability("p", 0.05); err == nil {
		t.Fatal("a single recorded run must be rejected")
	}
}

func TestConsensusRejectsSingleSampleAndPropagatesReadError(t *testing.T) {
	one := []contract.JudgeSample{{SampleID: "s1", Provenance: "a"}}
	if _, err := (app.Service{Files: &inputFiles{samples: one}}).Consensus("p"); err == nil {
		t.Fatal("a single sample has no consensus")
	}
	sentinel := errors.New("boom")
	if _, err := (app.Service{Files: &inputFiles{err: sentinel}}).Consensus("p"); !errors.Is(err, sentinel) {
		t.Fatalf("read error = %v", err)
	}
}
