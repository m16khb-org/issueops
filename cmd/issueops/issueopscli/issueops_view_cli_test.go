package issueopscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

// seedViewTimelineRecord는 시각이 있는 이력 네 건(시작, 회귀, 계획 리뷰 두 라운드)을
// 가진 record를 임시 state에 넣고 state root와 id를 돌려준다.
func seedViewTimelineRecord(t *testing.T) (root, id string) {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root = issueOpsStateRootForTest()
	id = "io-view-timeline"
	repo := testsupport.AbsPath("repo")
	seedListRecord(t, root, issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id, Repo: repo, Phase: issueopscontract.PhaseProblem,
		CreatedAt: "2026-09-07T00:00:00Z",
		UpdatedAt: "2026-09-08T02:00:00Z",
		RegressEvents: []issueopscontract.RegressEvent{
			{At: "2026-09-08T00:00:00Z", FromPhase: issueopscontract.PhasePlan},
		},
		DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "2026-09-08T02:00:00Z",
			History: []issueopscontract.DevilsAdvocateRound{
				{Verdict: "revise", RecordedAt: "2026-09-08T01:00:00Z"},
			},
		},
	})
	return root, id
}

// view --view timeline은 live record의 이력을 시각 순 이벤트로 돌려주고, 잘못된
// 인자는 다른 issueops 명령과 같은 JSON 오류 형태로 거부한다.
func TestCLIIssueOpsViewTimelineReadsRecord(t *testing.T) {
	_, id := seedViewTimelineRecord(t)

	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"view", "--id", id, "--view", "timeline", "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops view --json: %v\n%s", runErr, out)
	}
	var result issueopsviewcontract.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("view --json is not JSON: %v\n%s", err, out)
	}
	if !result.OK || result.View != "timeline" || result.ID != id || result.Source != "record" || result.GeneratedAt == "" {
		t.Fatalf("unexpected result header: %+v\n%s", result, out)
	}
	if result.Outcome != "" || result.RetainedAt != "" {
		t.Fatalf("a live view must not carry retained fields: %+v", result)
	}
	want := []issueopsviewcontract.TimelineEvent{
		{At: "2026-09-07T00:00:00Z", Kind: "cycle_started"},
		{At: "2026-09-08T00:00:00Z", Kind: "regressed", Phase: "plan"},
		{At: "2026-09-08T01:00:00Z", Kind: "review", Subject: "plan", Verdict: "revise", Round: 1},
		{At: "2026-09-08T02:00:00Z", Kind: "review", Subject: "plan", Verdict: "pass", Round: 2},
	}
	if result.Timeline == nil || !reflect.DeepEqual(result.Timeline.Events, want) {
		t.Fatalf("timeline events = %+v, want %+v\n%s", result.Timeline, want, out)
	}
	if result.Timeline.EventsTotal != 4 || result.Timeline.EventsShown != 4 || result.Timeline.Undated != 0 {
		t.Fatalf("timeline counts = %+v", result.Timeline)
	}

	text := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"view", "--id", id, "--view", "timeline"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops view: %v", runErr)
	}
	if first, _, _ := strings.Cut(text, "\n"); first != "timeline: shown 4 of 4 (undated 0) source=record" {
		t.Fatalf("first text line = %q\n%s", first, text)
	}
	if !strings.Contains(text, "  2026-09-08T01:00:00Z  review  -  plan  -  revise\n") {
		t.Fatalf("text lacks the review event row:\n%s", text)
	}

	for _, args := range [][]string{
		{"view", "--id", id, "--view", "bogus", "--json"},
		{"view", "--id", id, "--json"},
	} {
		badOut, badErr := captureStdoutAndErrorForIssueOps(t, func() error { return runIssueOps(args) })
		assertIssueOpsJSONErrorContains(t, badOut, badErr, "--view must be timeline")
	}
	noIDOut, noIDErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"view", "--view", "timeline", "--json"})
	})
	assertIssueOpsJSONErrorContains(t, noIDOut, noIDErr, "view requires --id")

	missingOut, missingErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"view", "--id", "io-missing", "--view", "timeline", "--json"})
	})
	assertIssueOpsJSONErrorContains(t, missingOut, missingErr, "io-missing")
}

// view는 읽기 전용이다. record 경로와 보존 행 경로 모두 state의 모든 row와 state
// 디렉터리의 파일을 그대로 두고 원격 CLI를 부르지 않는다.
func TestIssueOpsViewIsReadOnly(t *testing.T) {
	root, id := seedViewTimelineRecord(t)
	outcome, err := json.Marshal(issueopscontract.CycleOutcome{
		SchemaVersion: 1, ID: "io-gone", Repo: testsupport.AbsPath("repo"),
		Outcome: "abandoned", RetainedAt: "2026-09-09T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "issueops_cycle_outcome_v1", ID: "io-gone", Data: outcome},
	}); err != nil {
		t.Fatal(err)
	}

	buckets := snapshotBuckets(t, root)
	files := dirFileNames(t, root)
	for _, prefix := range []string{"issueops_v1/" + id + " ", "issueops_cycle_outcome_v1/io-gone "} {
		if !slices.ContainsFunc(buckets, func(line string) bool { return strings.HasPrefix(line, prefix) }) {
			t.Fatalf("the seed lacks a row starting %q:\n%v", prefix, buckets)
		}
	}

	markers := t.TempDir()
	bin := t.TempDir()
	for _, tool := range []string{"gh", "glab"} {
		marker := filepath.ToSlash(filepath.Join(markers, tool+".called"))
		testsupport.WriteFakeCommand(t, bin, tool, "#!/bin/sh\n: > '"+marker+"'\nexit 1\n")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, target := range []struct{ id, source string }{{id, "record"}, {"io-gone", "retained"}} {
		var runErr error
		out := captureStdoutForContract(t, func() error {
			runErr = runIssueOps([]string{"view", "--id", target.id, "--view", "timeline", "--json"})
			return runErr
		})
		if runErr != nil {
			t.Fatalf("issueops view --id %s: %v\n%s", target.id, runErr, out)
		}
		var result issueopsviewcontract.Result
		if err := json.Unmarshal([]byte(out), &result); err != nil || result.Source != target.source {
			t.Fatalf("view --id %s source = %q (want %s) err=%v\n%s", target.id, result.Source, target.source, err, out)
		}
	}

	if after := snapshotBuckets(t, root); !slices.Equal(after, buckets) {
		t.Fatalf("view changed the state records:\nbefore=%v\nafter=%v", buckets, after)
	}
	if after := dirFileNames(t, root); !slices.Equal(after, files) {
		t.Fatalf("view changed the state files:\nbefore=%v\nafter=%v", files, after)
	}
	for _, tool := range []string{"gh", "glab"} {
		if _, err := os.Stat(filepath.Join(markers, tool+".called")); !os.IsNotExist(err) {
			t.Fatalf("view invoked %s (stat err=%v)", tool, err)
		}
	}
}
