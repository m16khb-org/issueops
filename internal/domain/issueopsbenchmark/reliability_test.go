package issueopsbenchmark

import (
	"math"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func run(id, prov string, outcomes map[string]bool) contract.RecordedRun {
	return contract.RecordedRun{RunID: id, Provenance: prov, Outcomes: outcomes}
}

func TestComputeReliabilityRejectsInvalidInput(t *testing.T) {
	ok := map[string]bool{"a": true}
	cases := []struct {
		name  string
		runs  []contract.RecordedRun
		alpha float64
		want  string
	}{
		{"single run", []contract.RecordedRun{run("1", "p", ok)}, 0.05, "at least 2"},
		{"alpha zero", []contract.RecordedRun{run("1", "p", ok), run("2", "p", ok)}, 0, "alpha"},
		{"alpha one", []contract.RecordedRun{run("1", "p", ok), run("2", "p", ok)}, 1, "alpha"},
		{"empty run id", []contract.RecordedRun{run(" ", "p", ok), run("2", "p", ok)}, 0.05, "empty run_id"},
		{"duplicate run id", []contract.RecordedRun{run("1", "p", ok), run(" 1", "p", ok)}, 0.05, "duplicate run_id"},
		{"empty provenance", []contract.RecordedRun{run("1", "p", ok), run("2", " ", ok)}, 0.05, "empty provenance"},
		{"no outcomes", []contract.RecordedRun{run("1", "p", ok), run("2", "p", nil)}, 0.05, "no outcomes"},
		{"fixture count mismatch", []contract.RecordedRun{run("1", "p", ok), run("2", "p", map[string]bool{"a": true, "b": true})}, 0.05, "covers 2 fixtures, expected 1"},
		{"fixture set mismatch", []contract.RecordedRun{run("1", "p", ok), run("2", "p", map[string]bool{"z": true})}, 0.05, `unexpected fixture "z"`},
	}
	for _, c := range cases {
		_, err := ComputeReliability(contract.RecordedOutcomes{Runs: c.runs}, c.alpha)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	runs := make([]contract.RecordedRun, MaxReliabilityTrials+1)
	for i := range runs {
		runs[i] = run(string(rune('A'+i%26))+strings.Repeat("x", i/26), "p", ok)
	}
	if _, err := ComputeReliability(contract.RecordedOutcomes{Runs: runs}, 0.05); err == nil {
		t.Fatal("more than MaxReliabilityTrials runs must be rejected")
	}
}

func TestComputeReliabilityAggregates(t *testing.T) {
	rep, err := ComputeReliability(contract.RecordedOutcomes{Runs: []contract.RecordedRun{
		run("1", "bucket-a", map[string]bool{"b": true, "a": true}),
		run("2", "bucket-a", map[string]bool{"b": false, "a": true}),
	}}, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Runs != 2 || rep.MaxK != 2 || rep.Alpha != 0.05 || rep.MacroPassAt1 != 0.75 {
		t.Fatalf("%+v", rep)
	}
	if !reflect.DeepEqual(rep.Provenance, []string{"bucket-a"}) {
		t.Fatalf("shared provenance must be deduplicated: %v", rep.Provenance)
	}
	if len(rep.Fixtures) != 2 || rep.Fixtures[0].FixtureID != "a" || rep.Fixtures[1].FixtureID != "b" {
		t.Fatalf("fixtures must be sorted: %+v", rep.Fixtures)
	}
	a, b := rep.Fixtures[0], rep.Fixtures[1]
	if a.Successes != 2 || a.Trials != 2 || a.PassAt1 != 1 || a.IntervalHigh != 1 || a.IntervalLow >= 1 {
		t.Fatalf("%+v", a)
	}
	if b.Successes != 1 || b.PassAt1 != 0.5 || !(b.IntervalLow > 0 && b.IntervalHigh < 1) || b.IntervalWidth != Round4(b.IntervalHigh-b.IntervalLow) {
		t.Fatalf("%+v", b)
	}
	want := []contract.PassPowKPoint{{K: 1, PassPowK: 0.75}, {K: 2, PassPowK: 0.5}}
	if !reflect.DeepEqual(rep.PassPowKCurve, want) {
		t.Fatalf("curve %+v", rep.PassPowKCurve)
	}
}

func TestPassPowK(t *testing.T) {
	cases := []struct {
		name    string
		s, n, k int
		want    float64
		wantErr bool
	}{
		{"k zero", 0, 3, 0, 1, false},
		{"all pass", 3, 3, 3, 1, false},
		{"hypergeometric", 3, 5, 2, 0.3, false},
		{"k above successes", 1, 5, 2, 0, false},
		{"k equals one", 2, 4, 1, 0.5, false},
		{"trials at cap", 1, MaxReliabilityTrials, 1, 1.0 / MaxReliabilityTrials, false},
		{"zero trials", 0, 0, 0, 0, true},
		{"trials above cap", 1, MaxReliabilityTrials + 1, 1, 0, true},
		{"negative successes", -1, 3, 1, 0, true},
		{"successes above trials", 4, 3, 1, 0, true},
		{"negative k", 1, 3, -1, 0, true},
		{"k above trials", 1, 3, 4, 0, true},
	}
	for _, c := range cases {
		got, err := PassPowK(c.s, c.n, c.k)
		if (err != nil) != c.wantErr || (!c.wantErr && !approx(got, c.want)) {
			t.Errorf("%s: %v %v", c.name, got, err)
		}
	}
}

func TestClopperPearson(t *testing.T) {
	lo, hi, err := ClopperPearson(0, 1, 0.05)
	if err != nil || lo != 0 || !approx(Round4(hi), 0.975) {
		t.Fatalf("0/1: %v %v %v", lo, hi, err)
	}
	lo, hi, err = ClopperPearson(1, 1, 0.05)
	if err != nil || hi != 1 || !approx(Round4(lo), 0.025) {
		t.Fatalf("1/1: %v %v %v", lo, hi, err)
	}
	lo, hi, err = ClopperPearson(5, 10, 0.05)
	if err != nil || !approx(Round4(lo), 0.1871) || !approx(Round4(hi), 0.8129) {
		t.Fatalf("5/10: %v %v %v", lo, hi, err)
	}
	if lo, hi, err := ClopperPearson(0, MaxReliabilityTrials, 0.05); err != nil || lo != 0 || hi <= 0 || hi >= 1 {
		t.Fatalf("0/%d must be accepted: %v %v %v", MaxReliabilityTrials, lo, hi, err)
	}
	for _, c := range []struct {
		c, n int
		a    float64
	}{{0, 0, .05}, {0, MaxReliabilityTrials + 1, .05}, {-1, 3, .05}, {4, 3, .05}, {1, 3, 0}, {1, 3, 1}} {
		if _, _, err := ClopperPearson(c.c, c.n, c.a); err == nil {
			t.Errorf("%+v must error", c)
		}
	}
}

func TestBinomialHelpers(t *testing.T) {
	for _, c := range []struct {
		n, k int
		want float64
	}{{5, 2, 10}, {5, 0, 1}, {5, 5, 1}, {5, 6, 0}, {5, -1, 0}, {50, 25, 126410606437752}} {
		if got := binomCoeff(c.n, c.k); math.Abs(got-c.want) > 1e-3*math.Max(1, c.want)*1e-6 {
			t.Errorf("C(%d,%d)=%v", c.n, c.k, got)
		}
	}
	if !approx(binomTailGE(4, 0, 0.3), 1) || !approx(binomTailLE(4, 4, 0.3), 1) {
		t.Fatal("full tails must equal 1")
	}
	if !approx(binomTailGE(2, 2, 0.5), 0.25) || !approx(binomTailLE(2, 0, 0.5), 0.25) {
		t.Fatal("tail values")
	}
}

func TestScoreSpreadAndRound4(t *testing.T) {
	if lo, hi, w := ScoreSpread(nil); lo != 0 || hi != 0 || w != 0 {
		t.Fatal("empty")
	}
	if lo, hi, w := ScoreSpread([]float64{5}); lo != 5 || hi != 5 || w != 0 {
		t.Fatal("single")
	}
	if lo, hi, w := ScoreSpread([]float64{3, 9, -1, 4}); lo != -1 || hi != 9 || w != 10 {
		t.Fatalf("%v %v %v", lo, hi, w)
	}
	if Round4(0.123456) != 0.1235 || Round4(0.12344) != 0.1234 {
		t.Fatal("Round4")
	}
}

func TestConsensusJudgeVerdict(t *testing.T) {
	s := func(id, prov string, avg float64, pass bool) contract.JudgeSample {
		return contract.JudgeSample{SampleID: id, Provenance: prov, Score: contract.IssueOpsBenchmarkScore{AverageScore: avg, Passed: pass}}
	}
	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			name    string
			samples []contract.JudgeSample
			want    string
		}{
			{"too few", []contract.JudgeSample{s("a", "p", 1, true)}, "at least 2"},
			{"empty id", []contract.JudgeSample{s(" ", "p", 1, true), s("b", "p", 1, true)}, "empty sample_id"},
			{"empty provenance", []contract.JudgeSample{s("a", "p", 1, true), s("b", "", 1, true)}, "empty provenance"},
		}
		for _, c := range cases {
			if _, err := ConsensusJudgeVerdict(c.samples); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: %v", c.name, err)
			}
		}
	})
	t.Run("majority pass", func(t *testing.T) {
		v, err := ConsensusJudgeVerdict([]contract.JudgeSample{s("a", "p", 100, true), s("b", "p", 100, true), s("c", "q", 50, false)})
		if err != nil {
			t.Fatal(err)
		}
		if !v.MajorityPassed || v.Samples != 3 || v.PassAgreement != 0.6667 || v.MedianAverageScore != 100 ||
			v.ScoreMin != 50 || v.ScoreMax != 100 || v.ScoreSpread != 50 || v.SampleVariance != 555.5556 {
			t.Fatalf("%+v", v)
		}
		if !reflect.DeepEqual(v.Provenance, []string{"p", "q"}) || v.Caveat == "" {
			t.Fatalf("%+v", v)
		}
	})
	t.Run("tie fails closed", func(t *testing.T) {
		v, err := ConsensusJudgeVerdict([]contract.JudgeSample{s("a", "p", 100, true), s("b", "p", 0, false)})
		if err != nil || v.MajorityPassed || v.PassAgreement != 0.5 || v.MedianAverageScore != 50 {
			t.Fatalf("%+v %v", v, err)
		}
	})
	t.Run("majority fail agreement", func(t *testing.T) {
		v, _ := ConsensusJudgeVerdict([]contract.JudgeSample{s("a", "p", 10, false), s("b", "p", 20, false), s("c", "p", 90, true)})
		if v.MajorityPassed || v.PassAgreement != 0.6667 || v.MedianAverageScore != 20 {
			t.Fatalf("%+v", v)
		}
	})
}

func TestMedianAndVarianceEdgeCases(t *testing.T) {
	if medianFloat(nil) != 0 || sampleVariance(nil) != 0 {
		t.Fatal("empty")
	}
	in := []float64{3, 1, 2}
	if medianFloat(in) != 2 || !reflect.DeepEqual(in, []float64{3, 1, 2}) {
		t.Fatal("odd median must not mutate input")
	}
	if medianFloat([]float64{4, 1, 3, 2}) != 2.5 {
		t.Fatal("even median")
	}
	if sampleVariance([]float64{2, 2, 2}) != 0 || !approx(sampleVariance([]float64{1, 3}), 1) {
		t.Fatal("variance")
	}
}
