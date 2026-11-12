package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

// 프로덕션 배선으로 `issueops prune`을 끝까지 돌려, 지운 record가 같은 commit에
// 남긴 `pruned` 보존 행을 review-metrics, `view`, `status`가 읽는지 확인한다.
// 대상 record는 직접 쓰고, 보존 행은 prune 실행 경로만 쓴다.
func TestIssueOpsPruneRetainsOutcomeThroughCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	root = issueOpsStateRoot()
	repo := makeGitRepoForContract(t)
	ctx := context.Background()

	target, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "993-prune-target"})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "993-prune-kept"})
	if err != nil {
		t.Fatal(err)
	}

	// prune 조건을 채운 대상 record를 record bucket에 직접 덮어쓴다.
	target.Phase = model.PhaseDone
	target.UpdatedAt = time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339Nano)
	target.RegressEvents = []model.RegressEvent{{Reason: "scope too broad", FromPhase: model.PhasePlan, At: "2026-09-08T00:00:00Z"}}
	if target.Execution != nil {
		target.Execution.Lease.Status = model.LeaseStatusReleased
	}
	data, err := issueopsrecord.Encode(target)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(ctx, []port.RecordMutation{{Bucket: port.IssueOpsRecordBucket, ID: target.ID, Data: data}}); err != nil {
		t.Fatal(err)
	}

	before, err := (issueopsrecord.Store{}).Read(ctx, root, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := issueopsdomain.BuildCycleTimeline(before)
	if !hasTimelineEvent(want, model.TimelineKindRegressed) || !hasTimelineEvent(want, model.TimelineKindCycleStarted) {
		t.Fatalf("expected timeline lacks the seeded events: %+v", want)
	}

	reviewMetrics := func() (map[string]any, []model.ReviewMetricsCycle) {
		t.Helper()
		out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"review-metrics", "--repo", repo, "--json"})
		})
		var raw struct {
			Retention map[string]any             `json:"retention"`
			Cycles    []model.ReviewMetricsCycle `json:"cycles"`
		}
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			t.Fatalf("review-metrics JSON %q: %v", out, err)
		}
		if raw.Retention == nil {
			t.Fatalf("review-metrics --repo has no retention field: %s", out)
		}
		return raw.Retention, raw.Cycles
	}
	prune := func(args ...string) issueopsretentioncontract.Result {
		t.Helper()
		out := captureStdoutForContract(t, func() error {
			return runIssueOps(append([]string{"prune"}, args...))
		})
		var result issueopsretentioncontract.Result
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("prune JSON %q: %v", out, err)
		}
		return result
	}

	startRetention, _ := reviewMetrics()
	if want := map[string]any{"finished": float64(0), "abandoned": float64(0), "pruned": float64(0)}; !reflect.DeepEqual(startRetention, want) {
		t.Fatalf("before prune retention=%v want %v", startRetention, want)
	}

	// 미리보기는 모든 행과 state 파일을 그대로 둔다.
	rows := snapshotViewGraphBuckets(t, root)
	files := viewGraphStateFiles(t, root)
	preview := prune("--json")
	if !preview.DryRun || !reflect.DeepEqual(preview.Pruned, []string{target.ID}) || !slices.Contains(preview.Kept, kept.ID) {
		t.Fatalf("prune preview = %+v", preview)
	}
	if got := snapshotViewGraphBuckets(t, root); !reflect.DeepEqual(got, rows) {
		t.Fatalf("prune preview changed rows:\nbefore %v\nafter  %v", rows, got)
	}
	if got := viewGraphStateFiles(t, root); !reflect.DeepEqual(got, files) {
		t.Fatalf("prune preview changed state files: before %v after %v", files, got)
	}
	if _, found, err := core.ReadCycleOutcome(root, target.ID); err != nil || found {
		t.Fatalf("preview left a retained row: found=%v err=%v", found, err)
	}

	// 확정은 record를 지우고 같은 commit에 pruned 보존 행을 남긴다.
	result := prune("--confirm", "--json")
	if !result.OK || result.DryRun || !reflect.DeepEqual(result.Pruned, []string{target.ID}) || !slices.Contains(result.Kept, kept.ID) {
		t.Fatalf("prune confirm = %+v", result)
	}
	if _, err := (issueopsrecord.Store{}).Read(ctx, root, target.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("pruned record read err = %v, want fs.ErrNotExist", err)
	}
	if _, err := core.ReadIssueOps(root, kept.ID); err != nil {
		t.Fatalf("kept record unreadable: %v", err)
	}
	if _, found, err := core.ReadCycleOutcome(root, kept.ID); err != nil || found {
		t.Fatalf("kept record has a retained row: found=%v err=%v", found, err)
	}

	outcome, found, err := core.ReadCycleOutcome(root, target.ID)
	if err != nil || !found {
		t.Fatalf("pruned record has no retained row: found=%v err=%v", found, err)
	}
	if outcome.Outcome != model.CycleOutcomePruned {
		t.Fatalf("retained outcome = %q, want %q", outcome.Outcome, model.CycleOutcomePruned)
	}
	if _, err := time.Parse(time.RFC3339, outcome.RetainedAt); err != nil {
		t.Fatalf("retained_at %q is not RFC3339: %v", outcome.RetainedAt, err)
	}

	retention, cycles := reviewMetrics()
	if want := map[string]any{"finished": float64(0), "abandoned": float64(0), "pruned": float64(1), "last_retained_at": outcome.RetainedAt}; !reflect.DeepEqual(retention, want) {
		t.Fatalf("after prune retention=%v want %v", retention, want)
	}
	pruned := 0
	for _, cycle := range cycles {
		if cycle.ID == target.ID && cycle.Outcome == model.CycleOutcomePruned {
			pruned++
		}
	}
	if pruned != 1 {
		t.Fatalf("review-metrics cycles hold %d pruned rows for %s: %+v", pruned, target.ID, cycles)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"view", "--id", target.ID, "--view", "timeline", "--json"})
	})
	var view issueopsviewcontract.Result
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("view --json is not JSON: %v\n%s", err, out)
	}
	if !view.OK || view.Source != issueopsviewcontract.SourceRetained || view.Outcome != model.CycleOutcomePruned || view.ID != target.ID || view.RetainedAt != outcome.RetainedAt {
		t.Fatalf("view after prune = %+v\n%s", view, out)
	}
	if view.Timeline == nil || !reflect.DeepEqual(*view.Timeline, want) {
		t.Fatalf("retained timeline = %+v, want %+v", view.Timeline, want)
	}

	// 사라진 record의 status --json은 실패를 돌려주고, 안내는 다른 --json 오류처럼
	// stdout의 {"ok":false,"error":...} 객체에 실린다.
	stdout, statusErr := testsupport.CaptureStdoutAndError(t, func() error {
		return runIssueOps([]string{"status", "--id", target.ID, "--json"})
	})
	if statusErr == nil {
		t.Fatalf("status of a pruned record succeeded: %s", stdout)
	}
	var statusOut struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &statusOut); err != nil {
		t.Fatalf("status --json stdout is not JSON: %v\n%s", err, stdout)
	}
	wantNotice := "prune removed it (outcome pruned at " + outcome.RetainedAt + ")"
	if statusOut.OK || !strings.Contains(statusOut.Error, wantNotice) {
		t.Fatalf("status --json error = %q (ok=%v), want it to contain %q", statusOut.Error, statusOut.OK, wantNotice)
	}
}
