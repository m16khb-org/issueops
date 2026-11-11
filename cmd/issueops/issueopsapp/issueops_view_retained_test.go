package issueopsapp

import (
	"context"
	"encoding/json"
	"flag"
	"reflect"
	"testing"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// 실제 cleanup abandon이 record를 지우면서 남긴 보존 행을 `issueops view`가 읽어
// 지워지기 직전의 타임라인을 그대로 돌려준다. 보존 행은 테스트가 직접 쓰지 않고
// abandon 실행 경로가 쓴다.
func TestViewTimelineAfterRealAbandon(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	root = issueOpsStateRoot()

	record, err := startIssueOpsFixture(root, model.StartRequest{Repo: makeGitRepoForContract(t), Branch: "992-view-abandon"})
	if err != nil {
		t.Fatal(err)
	}
	record.RegressEvents = []model.RegressEvent{{Reason: "scope too broad", FromPhase: model.PhasePlan, At: "2026-09-08T00:00:00Z"}}
	if _, err := (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	saved, err := core.ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := issueopsdomain.BuildCycleTimeline(saved)
	if !hasTimelineEvent(want, model.TimelineKindRegressed) || !hasTimelineEvent(want, model.TimelineKindCycleStarted) {
		t.Fatalf("expected timeline lacks the seeded events: %+v", want)
	}

	var gitCalls [][]string
	var output model.CleanupAbandonResult
	deps := feedbackcleanup.Deps{
		ParseFlags: func(fs *flag.FlagSet, args []string) (bool, error) { return false, fs.Parse(args) },
		PrintJSON: func(v any) error {
			if result, ok := v.(model.CleanupAbandonResult); ok {
				output = result
			}
			return nil
		},
		PrintError: func(error) error { return nil },
		// Fail closed: only the read-only probes of an unlinked branch are known.
		CleanupFinishGit: func(dir string, args ...string) (int, string) {
			gitCalls = append(gitCalls, append([]string{dir}, args...))
			if len(args) > 0 && args[0] == "rev-parse" {
				return 1, ""
			}
			t.Errorf("unexpected git call in %s: %v", dir, args)
			return 2, "unexpected git call"
		},
	}
	command := newIssueOpsCleanup(root)
	args := []string{"abandon", "--id", record.ID, "--reason", "view timeline retention", "--preview", "--json"}
	if err := command.RunCleanup(args, deps); err != nil || output.Fingerprint == "" {
		t.Fatalf("abandon preview: %+v err=%v git=%v", output, err, gitCalls)
	}
	args = []string{"abandon", "--id", record.ID, "--reason", "view timeline retention", "--apply", "--confirm", "--fingerprint", output.Fingerprint, "--json"}
	if err := command.RunCleanup(args, deps); err != nil || !output.RecordDeleted {
		t.Fatalf("abandon apply: %+v err=%v git=%v", output, err, gitCalls)
	}
	if _, err := core.ReadIssueOps(root, record.ID); err == nil {
		t.Fatal("the record must be gone after a real abandon")
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"view", "--id", record.ID, "--view", "timeline", "--json"})
	})
	var result issueopsviewcontract.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("view --json is not JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Source != issueopsviewcontract.SourceRetained || result.Outcome != model.CycleOutcomeAbandoned || result.ID != record.ID || result.RetainedAt == "" {
		t.Fatalf("view after abandon = %+v\n%s", result, out)
	}
	if result.Timeline == nil {
		t.Fatalf("retained view has no timeline: %s", out)
	}
	// abandon arms the record with an "applying" receipt before deleting it; that
	// arm marker is not a failure, so the retained timeline equals the one built
	// just before the abandon.
	if !reflect.DeepEqual(*result.Timeline, want) {
		t.Fatalf("retained timeline = %+v, want %+v", *result.Timeline, want)
	}
}

func hasTimelineEvent(timeline model.CycleTimeline, kind string) bool {
	for _, event := range timeline.Events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}
