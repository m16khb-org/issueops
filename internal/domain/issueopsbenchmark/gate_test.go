package issueopsbenchmark

import (
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func runWith(ok bool, avg, min float64, crit int, scores ...contract.IssueOpsDimensionScore) contract.IssueOpsBenchmarkRunResult {
	return contract.IssueOpsBenchmarkRunResult{
		OK: ok, AverageScore: avg, MinimumScore: min, CriticalFailureCount: crit,
		Scores: []contract.IssueOpsBenchmarkScore{{DimensionScores: scores}},
	}
}

func TestNormalizeIssueOpsPath(t *testing.T) {
	for in, want := range map[string]string{
		" ./skills/a/ ": "skills/a",
		`skills\a\b`:    "skills/a/b",
		"/abs/":         "abs",
		"./":            "",
		"":              "",
	} {
		if got := normalizeIssueOpsPath(in); got != want {
			t.Errorf("%q: %q want %q", in, got, want)
		}
	}
}

func TestEditSurfaceViolations(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
		surface []string
		want    []string
	}{
		{"no changes", nil, []string{"a/**"}, nil},
		{"no surface means nothing checked", []string{"x"}, nil, nil},
		{"recursive glob allows nested", []string{"skills/a/b/c.md"}, []string{"skills/**"}, nil},
		{"recursive glob allows the dir itself", []string{"skills"}, []string{"skills/**"}, nil},
		{"recursive glob rejects sibling prefix", []string{"skills-extra/a"}, []string{"skills/**"}, []string{"skills-extra/a"}},
		{"star glob matches one level", []string{"docs/a.md"}, []string{"docs/*.md"}, nil},
		{"star glob does not cross dirs", []string{"docs/x/a.md"}, []string{"docs/*.md"}, []string{"docs/x/a.md"}},
		{"exact path", []string{"README.md"}, []string{"README.md"}, nil},
		{"normalizes both sides", []string{`.\skills\a.md`}, []string{"./skills/**"}, nil},
		{"blank changed path ignored", []string{" ", "./"}, []string{"a/**"}, nil},
		{"blank pattern ignored", []string{"a"}, []string{"", "b"}, []string{"a"}},
		{"reports normalized violations in order", []string{"./z", "ok/a", "y/"}, []string{"ok/**"}, []string{"z", "y"}},
	}
	for _, c := range cases {
		if got := issueOpsEditSurfaceViolations(c.changed, c.surface); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestDimensionMinimumsAndRegressions(t *testing.T) {
	base := runWith(true, 90, 80, 0,
		contract.IssueOpsDimensionScore{Dimension: "plan_quality", Score: 90},
		contract.IssueOpsDimensionScore{Dimension: "plan_quality", Score: 70},
		contract.IssueOpsDimensionScore{Dimension: "tdd_quality", Score: 100},
		contract.IssueOpsDimensionScore{Dimension: "phase_control_quality", Score: 0, NotApplicable: true})
	mins := issueOpsDimensionMinimums(base)
	if len(mins) != 2 || mins["plan_quality"] != 70 || mins["tdd_quality"] != 100 {
		t.Fatalf("%v", mins)
	}
	cand := runWith(true, 90, 80, 0,
		contract.IssueOpsDimensionScore{Dimension: "plan_quality", Score: 60},
		contract.IssueOpsDimensionScore{Dimension: "tdd_quality", Score: 100},
		contract.IssueOpsDimensionScore{Dimension: "phase_control_quality", Score: 0})
	if got := compareIssueOpsDimensionRegressions(base, cand); !reflect.DeepEqual(got, []string{"plan_quality"}) {
		t.Fatalf("n/a baseline dimension must not be compared: %v", got)
	}
	if got := issueOpsTargetDimensionRegressions([]string{" plan_quality ", "", "tdd_quality", "unknown"}, base, cand); !reflect.DeepEqual(got, []string{"plan_quality"}) {
		t.Fatalf("%v", got)
	}
}

func TestCompareRuns(t *testing.T) {
	d := func(dim string, s float64) contract.IssueOpsDimensionScore {
		return contract.IssueOpsDimensionScore{Dimension: dim, Score: s}
	}
	cases := []struct {
		name         string
		base, cand   contract.IssueOpsBenchmarkRunResult
		ok, improved bool
	}{
		{"strictly better", runWith(true, 80, 70, 1), runWith(true, 90, 70, 0), true, true},
		{"better average with equal critical failures is improved", runWith(true, 80, 70, 1), runWith(true, 90, 70, 1), true, true},
		{"equal is ok but not improved", runWith(true, 80, 70, 0), runWith(true, 80, 70, 0), true, false},
		{"minimum dropped", runWith(true, 80, 70, 0), runWith(true, 95, 60, 0), false, false},
		{"more critical failures", runWith(true, 80, 70, 0), runWith(true, 95, 70, 1), false, false},
		{"dimension regression", runWith(true, 80, 70, 0, d("plan_quality", 90)), runWith(true, 95, 70, 0, d("plan_quality", 80)), false, false},
	}
	for _, c := range cases {
		r := CompareRuns(c.base, c.cand)
		if r.OK != c.ok || r.Improved != c.improved {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
	r := CompareRuns(contract.IssueOpsBenchmarkRunResult{ID: "b", AverageScore: 50, MinimumScore: 10, CriticalFailureCount: 2},
		contract.IssueOpsBenchmarkRunResult{ID: "c", AverageScore: 70, MinimumScore: 30, CriticalFailureCount: 1})
	if r.BaselineID != "b" || r.CandidateID != "c" || r.AverageScoreDelta != 20 || r.MinimumScoreDelta != 20 || r.CriticalFailureDelta != -1 {
		t.Fatalf("%+v", r)
	}
}

func TestEvaluateAutoresearchGate(t *testing.T) {
	good := contract.IssueOpsAutoresearchCandidate{ID: " c1 ", Hypothesis: "h", TargetDimensions: []string{"plan_quality"}, EditSurface: []string{"skills/**"}}
	dim := func(s float64) contract.IssueOpsDimensionScore {
		return contract.IssueOpsDimensionScore{Dimension: "plan_quality", Score: s}
	}
	base := runWith(true, 80, 70, 0, dim(80))
	t.Run("accepts valid improvement", func(t *testing.T) {
		r := EvaluateAutoresearchGate(contract.IssueOpsAutoresearchGateRequest{
			Candidate: good, BaselineRun: base, CandidateRun: runWith(true, 90, 80, 0, dim(90)), ChangedPaths: []string{"skills/a.md"},
		})
		if !r.OK || !r.KeepCandidate || len(r.DiscardReasons) != 0 || r.CandidateID != "c1" {
			t.Fatalf("%+v", r)
		}
	})
	cases := []struct {
		name   string
		mutate func(*contract.IssueOpsAutoresearchGateRequest)
		reason []string
	}{
		{"missing id", func(r *contract.IssueOpsAutoresearchGateRequest) { r.Candidate.ID = " " }, []string{"candidate id is required"}},
		{"missing hypothesis", func(r *contract.IssueOpsAutoresearchGateRequest) { r.Candidate.Hypothesis = "" }, []string{"candidate hypothesis is required"}},
		{"missing targets", func(r *contract.IssueOpsAutoresearchGateRequest) { r.Candidate.TargetDimensions = nil }, []string{"target dimensions are required"}},
		{"unknown target", func(r *contract.IssueOpsAutoresearchGateRequest) {
			r.Candidate.TargetDimensions = []string{"plan_quality", "bogus", " "}
		}, []string{`invalid target dimension "bogus"`}},
		{"missing surface", func(r *contract.IssueOpsAutoresearchGateRequest) { r.Candidate.EditSurface = nil }, []string{"edit surface is required"}},
		{"candidate failed", func(r *contract.IssueOpsAutoresearchGateRequest) { r.CandidateRun.OK = false }, []string{"candidate benchmark did not pass"}},
		{"out of surface", func(r *contract.IssueOpsAutoresearchGateRequest) { r.ChangedPaths = []string{"src/x.go"} }, []string{"changed paths outside declared edit surface"}},
		{"target regressed", func(r *contract.IssueOpsAutoresearchGateRequest) {
			r.CandidateRun = runWith(true, 90, 80, 0, dim(60))
		}, []string{"benchmark comparison regressed", "target dimensions regressed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := contract.IssueOpsAutoresearchGateRequest{Candidate: good, BaselineRun: base, CandidateRun: runWith(true, 90, 80, 0, dim(90))}
			c.mutate(&req)
			r := EvaluateAutoresearchGate(req)
			if r.OK || r.KeepCandidate || !reflect.DeepEqual(r.DiscardReasons, c.reason) {
				t.Fatalf("%+v", r)
			}
		})
	}
	t.Run("violations are listed", func(t *testing.T) {
		r := EvaluateAutoresearchGate(contract.IssueOpsAutoresearchGateRequest{
			Candidate: good, BaselineRun: base, CandidateRun: runWith(true, 90, 80, 0, dim(90)), ChangedPaths: []string{"skills/a", "./src/x.go"},
		})
		if !reflect.DeepEqual(r.EditSurfaceViolations, []string{"src/x.go"}) || !strings.Contains(strings.Join(r.DiscardReasons, "|"), "outside declared edit surface") {
			t.Fatalf("%+v", r)
		}
	})
}
