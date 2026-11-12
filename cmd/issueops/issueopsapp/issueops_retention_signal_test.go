package issueopsapp

import (
	"encoding/json"
	"flag"
	"reflect"
	"testing"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

// TestReviewMetricsRetentionCountsRealCleanup drives the production wiring end to
// end: a started cycle, a real `cleanup abandon` preview and apply, and the
// review-metrics retention signal before and after. The retained row is only ever
// written by the cleanup transaction; the test reads it back independently.
func TestReviewMetricsRetentionCountsRealCleanup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", root)
	root = issueOpsStateRoot()
	repo := makeGitRepoForContract(t)
	record, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "992-retention-signal"})
	if err != nil {
		t.Fatal(err)
	}

	retention := func() map[string]any {
		t.Helper()
		var raw map[string]json.RawMessage
		out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"review-metrics", "--repo", repo, "--json"})
		})
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			t.Fatalf("review-metrics JSON %q: %v", out, err)
		}
		field, ok := raw["retention"]
		if !ok {
			t.Fatalf("review-metrics --repo has no retention field: %s", out)
		}
		var got map[string]any
		if err := json.Unmarshal(field, &got); err != nil {
			t.Fatalf("retention %s: %v", field, err)
		}
		return got
	}

	// (1) Cleanup has not run: the signal is present and says zero.
	if got, want := retention(), map[string]any{"finished": float64(0), "abandoned": float64(0), "pruned": float64(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("before cleanup retention=%v want %v", got, want)
	}

	// (2) A real abandon removes the cycle. The fake observes only local reads and
	// fails on anything that would change a repository or reach a provider.
	var result model.CleanupAbandonResult
	deps := feedbackcleanup.Deps{
		ParseFlags: func(fs *flag.FlagSet, args []string) (bool, error) { return false, fs.Parse(args) },
		PrintJSON: func(v any) error {
			if r, ok := v.(model.CleanupAbandonResult); ok {
				result = r
			}
			return nil
		},
		PrintError: func(error) error { return nil },
		ObserveArtifactMerged: func(model.RemoteArtifactVerification) (bool, error) {
			t.Error("unexpected provider observation: the cycle has no remote artifact")
			return false, nil
		},
		CleanupFinishGit: func(dir string, args ...string) (int, string) {
			// The only call a worktreeless cycle makes is the branch probe; "not
			// found" is its honest answer. Anything else is unexpected.
			if len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "--quiet" && args[3] == "refs/heads/"+record.Branch {
				return 1, ""
			}
			t.Errorf("unexpected git call: %v (dir %s)", args, dir)
			return 1, ""
		},
	}
	cleanup := newIssueOpsCleanup(root)
	base := []string{"abandon", "--id", record.ID, "--reason", "retention signal"}
	if err := cleanup.RunCleanup(append(append([]string{}, base...), "--preview", "--json"), deps); err != nil {
		t.Fatalf("abandon preview refused: %v", err)
	}
	if result.Fingerprint == "" {
		t.Fatalf("abandon preview returned no fingerprint: %+v", result)
	}
	if err := cleanup.RunCleanup(append(append([]string{}, base...), "--apply", "--confirm", "--fingerprint", result.Fingerprint, "--json"), deps); err != nil {
		t.Fatalf("abandon apply refused: %v", err)
	}
	if !result.RecordDeleted {
		t.Fatalf("abandon apply did not delete the record: %+v", result)
	}

	// (3) The record is gone and the retained row is visible through the
	// read-only lister, independent of review-metrics.
	if _, err := core.ReadIssueOps(root, record.ID); err == nil {
		t.Fatal("record still readable after abandon")
	}
	outcomes, unreadable, err := core.ListCycleOutcomes(root)
	if err != nil || len(unreadable) != 0 {
		t.Fatalf("list outcomes: %v unreadable=%v", err, unreadable)
	}
	var retainedAt string
	for _, outcome := range outcomes {
		if outcome.ID == record.ID {
			if outcome.Outcome != model.CycleOutcomeAbandoned {
				t.Fatalf("retained outcome=%q want %q", outcome.Outcome, model.CycleOutcomeAbandoned)
			}
			retainedAt = outcome.RetainedAt
		}
	}
	if retainedAt == "" {
		t.Fatalf("no retained row for %s in %+v", record.ID, outcomes)
	}

	// (4) The signal now counts that row and carries its retained_at verbatim.
	want := map[string]any{"finished": float64(0), "abandoned": float64(1), "pruned": float64(0), "last_retained_at": retainedAt}
	if got := retention(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after abandon retention=%v want %v", got, want)
	}
}
