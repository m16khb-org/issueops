package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadSamplesFromFileAndStdin(t *testing.T) {
	body := `{"samples":[{"sample_id":"s1","provenance":"p"}]}`
	fromFile, err := Files{}.ReadSamples(writeTemp(t, "s.json", body))
	if err != nil || len(fromFile) != 1 || fromFile[0].SampleID != "s1" {
		t.Fatalf("file: %+v %v", fromFile, err)
	}
	fromStdin, err := Files{Stdin: strings.NewReader(body)}.ReadSamples("  ")
	if err != nil || len(fromStdin) != 1 || fromStdin[0].Provenance != "p" {
		t.Fatalf("stdin: %+v %v", fromStdin, err)
	}
}

func TestReadSamplesErrors(t *testing.T) {
	if _, err := (Files{}).ReadSamples(filepath.Join(t.TempDir(), "absent.json")); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
	if _, err := (Files{}).ReadSamples(writeTemp(t, "bad.json", "{")); err == nil || !strings.Contains(err.Error(), "parse judge samples") {
		t.Fatalf("bad json err = %v", err)
	}
}

func TestReadOutcomesFromFileAndStdin(t *testing.T) {
	body := `{"runs":[{"run_id":"r1","provenance":"p","outcomes":{"f":true}}]}`
	for name, got := range map[string]func() (contract.RecordedOutcomes, error){
		"file": func() (contract.RecordedOutcomes, error) { return Files{}.ReadOutcomes(writeTemp(t, "o.json", body)) },
		"stdin": func() (contract.RecordedOutcomes, error) {
			return Files{Stdin: strings.NewReader(body)}.ReadOutcomes("")
		},
	} {
		rec, err := got()
		if err != nil || len(rec.Runs) != 1 || rec.Runs[0].RunID != "r1" || !rec.Runs[0].Outcomes["f"] {
			t.Fatalf("%s: %+v %v", name, rec, err)
		}
	}
}

func TestReadOutcomesErrors(t *testing.T) {
	if _, err := (Files{}).ReadOutcomes(filepath.Join(t.TempDir(), "absent.json")); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
	if _, err := (Files{}).ReadOutcomes(writeTemp(t, "bad.json", "[]")); err == nil || !strings.Contains(err.Error(), "parse recorded outcomes") {
		t.Fatalf("bad json err = %v", err)
	}
}

func TestReadCandidate(t *testing.T) {
	got, err := Files{}.ReadCandidate(" " + writeTemp(t, "c.json", `{"id":"c1","hypothesis":"h","edit_surface":["a/"]}`) + " ")
	if err != nil || got.ID != "c1" || got.Hypothesis != "h" || len(got.EditSurface) != 1 {
		t.Fatalf("candidate = %+v %v", got, err)
	}
	if _, err := (Files{}).ReadCandidate("   "); err == nil || err.Error() != "candidate-file is required" {
		t.Fatalf("empty path err = %v", err)
	}
	if _, err := (Files{}).ReadCandidate(filepath.Join(t.TempDir(), "absent.json")); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
	bad := writeTemp(t, "bad.json", "nope")
	if _, err := (Files{}).ReadCandidate(bad); err == nil || !strings.Contains(err.Error(), "parse candidate file "+bad) {
		t.Fatalf("bad json err = %v", err)
	}
}

func TestReadJudgeMap(t *testing.T) {
	fixtures := []contract.IssueOpsBenchmarkFixture{{ID: "fixture"}}
	body := `{"source_run_id":"prior","provenance":"recorded","scores":{"fixture":` + validBenchmarkJudgeJSON + `}}`
	got, err := Files{Stdin: strings.NewReader(body)}.ReadJudgeMap("", fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceRunID != "prior" || got.Provenance != "recorded" || !got.Scores["fixture"].Passed {
		t.Fatalf("judge map = %+v", got)
	}
	fromFile, err := Files{}.ReadJudgeMap(writeTemp(t, "j.json", body), fixtures)
	if err != nil || fromFile.SourceRunID != "prior" {
		t.Fatalf("file: %+v %v", fromFile, err)
	}
}

func TestReadJudgeMapRejectsMalformedInput(t *testing.T) {
	fixtures := []contract.IssueOpsBenchmarkFixture{{ID: "fixture"}}
	for name, tc := range map[string]struct{ body, want string }{
		"unknown field":   {`{"scores":{},"extra":1}`, "parse judge map"},
		"not json":        {`{`, "parse judge map"},
		"missing scores":  {`{"source_run_id":"p"}`, `missing "scores"`},
		"missing fixture": {`{"scores":{}}`, `missing fixture "fixture"`},
		"unknown fixture": {`{"scores":{"fixture":` + validBenchmarkJudgeJSON + `,"zzz":{}}}`, `unknown fixture "zzz"`},
		"bad score":       {`{"scores":{"fixture":"x"}}`, `judge score for fixture "fixture"`},
	} {
		_, err := Files{Stdin: strings.NewReader(tc.body)}.ReadJudgeMap("", fixtures)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
	if _, err := (Files{}).ReadJudgeMap(filepath.Join(t.TempDir(), "absent.json"), fixtures); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
}

func TestStoreSaveThenReadRoundTripsAndValidatesID(t *testing.T) {
	store := Store{Directory: t.TempDir()}
	run := contract.IssueOpsBenchmarkRunResult{OK: true, ID: "run-1", FixtureCount: 2, AverageScore: 88.5}
	if err := store.Save(run); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(" run-1 ")
	if err != nil || got.ID != "run-1" || got.FixtureCount != 2 || got.AverageScore != 88.5 || !got.OK {
		t.Fatalf("read = %+v %v", got, err)
	}
	if _, err := store.Read("  "); err == nil || err.Error() != "benchmark id is required" {
		t.Fatalf("empty id err = %v", err)
	}
	if _, err := store.Read("absent"); !os.IsNotExist(err) {
		t.Fatalf("absent err = %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Directory, "issueops-benchmarks", "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("broken"); err == nil {
		t.Fatal("corrupt run file must fail to decode")
	}
}

func TestLoadFixturesOrdersByIDRegardlessOfFileName(t *testing.T) {
	dir := t.TempDir()
	for name, id := range map[string]string{"a.json": "zeta", "b.json": "alpha", "c.json": "mid"} {
		body := `{"id":"` + id + `","title":"t","user_prompt":"p","repo_context":"r","critical_failures":["f"]}`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixtures, err := (Files{}).LoadFixtures(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range fixtures {
		got = append(got, f.ID)
	}
	if strings.Join(got, ",") != "alpha,mid,zeta" {
		t.Fatalf("fixtures must be ordered by ID, got %v", got)
	}
}
