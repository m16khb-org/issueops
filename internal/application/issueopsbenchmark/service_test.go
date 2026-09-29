package issueopsbenchmark_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/cmd/issueops/issueopscli/benchmarkartifact"
	app "issueops/internal/application/issueopsbenchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	port "issueops/internal/port/issueopsbenchmark"
)

type runFiles struct {
	port.Files
	fixture           contract.IssueOpsBenchmarkFixture
	judge             contract.IssueOpsJudgeMap
	loadErr, judgeErr error
	trace             *[]string
}

func (f runFiles) LoadFixtures(string) ([]contract.IssueOpsBenchmarkFixture, error) {
	*f.trace = append(*f.trace, "fixtures")
	return []contract.IssueOpsBenchmarkFixture{f.fixture}, f.loadErr
}
func (f runFiles) ReadJudgeMap(string, []contract.IssueOpsBenchmarkFixture) (contract.IssueOpsJudgeMap, error) {
	*f.trace = append(*f.trace, "judge")
	return f.judge, f.judgeErr
}

type runStore struct {
	trace            *[]string
	readErr, saveErr error
	saved            []contract.IssueOpsBenchmarkRunResult
}

func (s *runStore) Read(id string) (contract.IssueOpsBenchmarkRunResult, error) {
	*s.trace = append(*s.trace, "read:"+id)
	return contract.IssueOpsBenchmarkRunResult{ID: id}, s.readErr
}
func (s *runStore) Save(result contract.IssueOpsBenchmarkRunResult) error {
	*s.trace = append(*s.trace, "save")
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, result)
	return nil
}

func TestRunRejectsJudgeBeforeSave(t *testing.T) {
	sentinel := errors.New("input failed")
	for _, tc := range []struct {
		name                       string
		judge                      contract.IssueOpsJudgeMap
		loadErr, judgeErr, readErr error
		want                       string
		trace                      []string
	}{
		{"fixture", contract.IssueOpsJudgeMap{}, sentinel, nil, nil, "input failed", []string{"fixtures"}},
		{"decode", contract.IssueOpsJudgeMap{}, nil, sentinel, nil, "input failed", []string{"fixtures", "artifact", "clock", "judge"}},
		{"self reference", contract.IssueOpsJudgeMap{SourceRunID: "issueops-benchmark-20260102T030405.000000000Z", Provenance: "recorded"}, nil, nil, nil, "scored run itself", []string{"fixtures", "artifact", "clock", "judge"}},
		{"missing provenance", contract.IssueOpsJudgeMap{SourceRunID: "prior"}, nil, nil, nil, "missing provenance", []string{"fixtures", "artifact", "clock", "judge"}},
		{"missing source", contract.IssueOpsJudgeMap{SourceRunID: "prior", Provenance: "recorded"}, nil, nil, sentinel, "does not resolve", []string{"fixtures", "artifact", "clock", "judge", "read:prior"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var trace []string
			store := &runStore{trace: &trace, readErr: tc.readErr}
			service := app.Service{Files: runFiles{fixture: contract.IssueOpsBenchmarkFixture{ID: "fixture"}, judge: tc.judge, loadErr: tc.loadErr, judgeErr: tc.judgeErr, trace: &trace}, Runs: store,
				Now: func() time.Time { trace = append(trace, "clock"); return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
				Artifact: func(f contract.IssueOpsBenchmarkFixture) contract.IssueOpsBenchmarkArtifact {
					trace = append(trace, "artifact")
					return benchmarkartifact.FromFixture(f)
				}}
			_, err := service.Run("fixtures", "file", "judge")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if len(store.saved) != 0 || !reflect.DeepEqual(trace, tc.trace) {
				t.Fatalf("effects=%v saved=%v", trace, store.saved)
			}
		})
	}
}

func TestRunFinalizesMergedScoresBeforePersisting(t *testing.T) {
	var trace []string
	fixture := contract.IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fix workflow", UserPrompt: "Improve workflow", RepoContext: "Go", CriticalFailures: []string{"missing issue"}}
	judge := contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{{Dimension: "intent_understanding", Score: 7, Evidence: "judge evidence"}}}
	store := &runStore{trace: &trace}
	service := app.Service{Files: runFiles{fixture: fixture, judge: contract.IssueOpsJudgeMap{SourceRunID: "prior", Provenance: "recorded", Scores: map[string]contract.IssueOpsBenchmarkScore{"fixture": judge}}, trace: &trace}, Runs: store,
		Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }, Artifact: benchmarkartifact.FromFixture}
	got, err := service.Run("fixtures", "file", "judge")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.FinalizeRun(contract.IssueOpsBenchmarkRunResult{ID: "issueops-benchmark-20260102T030405.000000000Z", Scores: []contract.IssueOpsBenchmarkScore{domain.MergeScoreWithJudge(domain.ScoreFixture(fixture, benchmarkartifact.FromFixture(fixture)), judge)}})
	if !reflect.DeepEqual(got, want) || len(store.saved) != 1 || !reflect.DeepEqual(store.saved[0], want) {
		t.Fatalf("result=%+v saved=%+v want=%+v", got, store.saved, want)
	}
	if !reflect.DeepEqual(trace, []string{"fixtures", "judge", "read:prior", "save"}) {
		t.Fatalf("effect order=%v", trace)
	}
}
