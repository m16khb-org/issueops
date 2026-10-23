package issueopsnext

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func TestNextOutsideRepositoryNeverListsCycles(t *testing.T) {
	ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan}}, nil)
	ports.WorktreeState = func(string) (bool, string, string) { return false, "", "" }
	ports.ListCycles = func(context.Context, string, string) (issueopsinventorycontract.ListResult, error) {
		t.Fatal("a non-repository cwd must not enumerate cycles")
		return issueopsinventorycontract.ListResult{}, nil
	}

	result, err := NewService(ports).Next(t.Context(), "/state", "/tmp/elsewhere", "")

	if err != nil || !result.OK {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if result.CWDRole != issueopsnextcontract.CWDRoleOther || result.SourceRoot != "" || result.Selected != nil {
		t.Fatalf("outside a repository the cwd is other with no source or selection: %+v", result)
	}
	if !slices.Contains(result.Warnings, "not a git repository: /tmp/elsewhere") {
		t.Fatalf("missing not-a-repository warning: %v", result.Warnings)
	}
}

func TestNextActorFailureWarnsButStillClassifies(t *testing.T) {
	ports := testPorts(nil, nil)
	ports.Actor = func() (string, string, error) { return "", "", errors.New("no session") }

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || !result.OK || result.Stage.Key != issueopsnextcontract.StageNone {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if !slices.Contains(result.Warnings, "native session identity is unavailable: no session") {
		t.Fatalf("missing identity warning: %v", result.Warnings)
	}
}

func TestNextPropagatesInventoryFailure(t *testing.T) {
	boom := errors.New("inventory unreadable")
	ports := testPorts(nil, nil)
	ports.ListCycles = func(context.Context, string, string) (issueopsinventorycontract.ListResult, error) {
		return issueopsinventorycontract.ListResult{}, boom
	}

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if !errors.Is(err, boom) || result.OK {
		t.Fatalf("an inventory failure must surface as a failed result: %+v, %v", result, err)
	}
}

func TestNextInvalidInventoryEntryPointsAtStatus(t *testing.T) {
	ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-bad", Invalid: true}}, nil)
	ports.ReadRecord = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		t.Fatal("an invalid inventory entry must not be read as a record")
		return issueopscontract.IssueOpsRecord{}, nil
	}

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || result.Stage.Key != issueopsnextcontract.StageInvalid {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if result.NextCommand != "issueops status --id io-bad --json" || result.NextCommandKind != issueopsnextcontract.NextCommandKindExact {
		t.Fatalf("invalid cycles must route to an exact status command: %q (%s)", result.NextCommand, result.NextCommandKind)
	}
	if result.Selected == nil || result.Selected.ID != "io-bad" {
		t.Fatalf("the invalid cycle must stay selected: %+v", result.Selected)
	}
}

func TestNextUnreadableRecordIsInvalidWithWarning(t *testing.T) {
	ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan}}, nil)
	ports.ReadRecord = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{}, errors.New("disk gone")
	}

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || !result.OK || result.Stage.Key != issueopsnextcontract.StageInvalid {
		t.Fatalf("an unreadable record degrades to invalid, not an error: %+v, %v", result, err)
	}
	if !slices.Contains(result.Warnings, "record is unreadable: disk gone") {
		t.Fatalf("missing unreadable-record warning: %v", result.Warnings)
	}
	if result.NextCommand != "issueops status --id io-a --json" {
		t.Fatalf("next command = %q", result.NextCommand)
	}
}

func TestNextSelectsByCurrentBranchWhenNoWorkspaceMatches(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		{ID: "io-a", Phase: issueopscontract.IssueOpsPhaseImplement, Branch: "12-a"},
		{ID: "io-b", Phase: issueopscontract.IssueOpsPhaseImplement, Branch: "13-b"},
	}
	records := map[string]issueopscontract.IssueOpsRecord{
		"io-b": {ID: "io-b", Repo: "/repo", Branch: "13-b", Phase: issueopscontract.IssueOpsPhaseImplement},
	}
	ports := testPorts(entries, records)
	ports.CurrentBranch = func(string) string { return " 13-b\n" }

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || result.Selected == nil || result.Selected.ID != "io-b" {
		t.Fatalf("the cycle on the current branch must win over ambiguity: %+v, %v", result.Selected, err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("a branch match is a selection, not candidates: %+v", result.Candidates)
	}
}

func TestNextDuplicateBranchOrWorkspaceStaysAmbiguous(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		entries []issueopsinventorycontract.ListEntry
		branch  string
		cwd     string
	}{
		{
			name: "two cycles share the current branch",
			entries: []issueopsinventorycontract.ListEntry{
				{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "12-a"},
				{ID: "io-b", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "12-a"},
			},
			branch: "12-a",
			cwd:    "/repo",
		},
		{
			name: "two cycles claim the cwd workspace",
			entries: []issueopsinventorycontract.ListEntry{
				{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan, WorkspaceRoot: "/repo.worktrees/x"},
				{ID: "io-b", Phase: issueopscontract.IssueOpsPhasePlan, WorkspaceRoot: "/repo.worktrees/x/"},
			},
			cwd: "/repo.worktrees/x",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ports := testPorts(scenario.entries, nil)
			ports.CurrentBranch = func(string) string { return scenario.branch }

			result, err := NewService(ports).Next(t.Context(), "/state", scenario.cwd, "")

			if err != nil || result.Stage.Key != issueopsnextcontract.StageAmbiguous || len(result.Candidates) != 2 {
				t.Fatalf("a non-unique match must not pick a cycle: %+v, %v", result, err)
			}
			if result.NextCommand != "issueops list --repo /repo --json" || result.NextCommandKind != issueopsnextcontract.NextCommandKindExact {
				t.Fatalf("ambiguity must route to an exact list command: %q (%s)", result.NextCommand, result.NextCommandKind)
			}
		})
	}
}

func TestNextAmbiguityWithoutSourceRootOffersTemplateCommand(t *testing.T) {
	ports := testPorts([]issueopsinventorycontract.ListEntry{
		{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan},
		{ID: "io-b", Phase: issueopscontract.IssueOpsPhasePlan},
	}, nil)
	ports.SourceRoot = func(string) string { return "  " }

	result, err := NewService(ports).Next(t.Context(), "/state", "/somewhere", "")

	if err != nil || result.Stage.Key != issueopsnextcontract.StageAmbiguous {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if result.NextCommand != "issueops list --repo <source_root> --json" || result.NextCommandKind != issueopsnextcontract.NextCommandKindTemplate {
		t.Fatalf("an unknown source root must yield a template command: %q (%s)", result.NextCommand, result.NextCommandKind)
	}
	if result.CWDRole != issueopsnextcontract.CWDRoleOther {
		t.Fatalf("a blank source root means the cwd role is other, got %q", result.CWDRole)
	}
}

func TestNextClassifiesCWDRoleFromSourceRoot(t *testing.T) {
	for _, scenario := range []struct {
		cwd, want string
	}{
		{"/repo", issueopsnextcontract.CWDRoleSource},
		{"/repo/", issueopsnextcontract.CWDRoleSource},
		{"/repo.worktrees/13-b/sub", issueopsnextcontract.CWDRoleWorktree},
		{"/repository", issueopsnextcontract.CWDRoleOther},
	} {
		t.Run(scenario.cwd, func(t *testing.T) {
			result, err := NewService(testPorts(nil, nil)).Next(t.Context(), "/state", scenario.cwd, "")
			if err != nil || result.CWDRole != scenario.want {
				t.Fatalf("cwd %q role = %q (err %v), want %q", scenario.cwd, result.CWDRole, err, scenario.want)
			}
		})
	}
}

func TestNextReportsClaimedWorktreeRootBeforePrepare(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		{ID: "io-mine", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "feat/x"},
		{ID: "io-other", Phase: issueopscontract.IssueOpsPhaseDone, Branch: "old", WorkspaceRoot: "/repo.worktrees/feat-x"},
	}
	records := map[string]issueopscontract.IssueOpsRecord{
		"io-mine": {ID: "io-mine", Repo: "/repo", Branch: "feat/x", Phase: issueopscontract.IssueOpsPhasePlan},
	}
	ports := testPorts(entries, records)
	ports.CurrentBranch = func(string) string { return "feat/x" }

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || result.Stage.Key != issueopsnextcontract.StageBlockedRoot {
		t.Fatalf("a root held by another cycle must block: %+v, %v", result, err)
	}
	if !slices.Contains(result.Warnings, "canonical worktree root is already claimed by cycle io-other") {
		t.Fatalf("the blocking cycle must be named: %v", result.Warnings)
	}
}

func TestNextRootConflictIgnoresCyclesWithoutBranchOrOwnRoot(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		branch string
		others []issueopsinventorycontract.ListEntry
	}{
		{"record has no branch", " ", []issueopsinventorycontract.ListEntry{{ID: "io-other", WorkspaceRoot: "/repo.worktrees/"}}},
		{"only the cycle's own entry owns the root", "13-b", []issueopsinventorycontract.ListEntry{{ID: "io-a", WorkspaceRoot: "/repo.worktrees/13-b"}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			record := issueopscontract.IssueOpsRecord{ID: "io-a", Repo: "/repo", Branch: scenario.branch, Phase: issueopscontract.IssueOpsPhasePlan}
			if got := rootConflict(Ports{}, record, "/repo", scenario.others); got != "" {
				t.Fatalf("unexpected conflict with %q", got)
			}
		})
	}
}

func TestNextFeedsStagedArtifactsToClassifier(t *testing.T) {
	architecture := strings.Join(append(append([]string{}, issueopsdomain.RequiredArchitectureSections...), ""), "\n본문\n")
	digest, err := reviewdomain.ArchitectureDocumentDigest(architecture)
	if err != nil {
		t.Fatalf("fixture architecture must be valid: %v", err)
	}
	record := func(reviewDigest string) issueopscontract.IssueOpsRecord {
		record := issueopscontract.IssueOpsRecord{
			ID: "io-a", Repo: "/repo", Branch: "12-a", Phase: issueopscontract.IssueOpsPhasePlan,
			IssueURL:      "https://example.test/issues/12",
			BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{BaseSHA: "deadbeef", LinkVerified: true},
		}
		if reviewDigest != "" {
			record.ArchitectureReview = &reviewcontract.ArchitectureReview{Approved: true, ReviewerContext: "subagent", ReviewedArchitectureDigest: reviewDigest}
		}
		return record
	}
	entries := []issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "12-a"}}
	for _, scenario := range []struct {
		name         string
		reviewDigest string
		staged       map[string]string
		stagedErr    error
		want         string
		wantMissing  string
	}{
		{"plan staged before architecture", "", map[string]string{"plan": "# plan"}, nil, issueopsnextcontract.StagePlanArchitecture, "architecture_artifact"},
		{"architecture staged, not reviewed", "", map[string]string{"architecture": architecture}, nil, issueopsnextcontract.StagePlanArchitecture, "architecture_review"},
		{"architecture reviewed, plan not staged", digest, map[string]string{"notes": "x", "architecture": architecture}, nil, issueopsnextcontract.StagePlanWrite, "plan_artifact"},
		{"both staged and the approved digest matches", digest, map[string]string{"plan": "# plan", "architecture": architecture}, nil, issueopsnextcontract.StagePlanHandoff, ""},
		{"architecture edited after approval", digest, map[string]string{"plan": "# plan", "architecture": architecture + "edited\n"}, nil, issueopsnextcontract.StagePlanArchitecture, "architecture_review_stale"},
		{"staged lookup fails", digest, nil, errors.New("boom"), issueopsnextcontract.StagePlanArchitecture, "architecture_artifact"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ports := testPorts(entries, map[string]issueopscontract.IssueOpsRecord{"io-a": record(scenario.reviewDigest)})
			ports.StagedArtifacts = func(_, id string) (map[string]string, error) {
				if id != "io-a" {
					t.Fatalf("staged artifacts asked for %q", id)
				}
				return scenario.staged, scenario.stagedErr
			}
			result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")
			wantMissing := []string{scenario.wantMissing}
			if scenario.wantMissing == "" {
				wantMissing = []string{}
			}
			if err != nil || result.Stage.Key != scenario.want || !slices.Equal(result.Missing, wantMissing) {
				t.Fatalf("stage = %q missing=%v (err %v), want %q [%s]", result.Stage.Key, result.Missing, err, scenario.want, scenario.wantMissing)
			}
		})
	}
}

func TestNextObservesHolderProcessAndWorktreeForExecutions(t *testing.T) {
	process := issueopscontract.NativeProcessReceipt{PID: 4242, StartedAt: "t0", Executable: "agent"}
	record := issueopscontract.IssueOpsRecord{
		ID: "io-a", Repo: "/repo", Branch: "12-a", Phase: issueopscontract.IssueOpsPhaseImplement,
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/12-a"},
			Lease: issueopscontract.WriteLease{
				Generation: 3,
				Status:     issueopscontract.LeaseStatusActive,
				Holder:     &issueopscontract.NativeActor{Host: "codex", SessionID: "other", SessionProcess: &process},
			},
		},
	}
	entries := []issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: record.Phase, Branch: "12-a"}}
	for _, scenario := range []struct {
		name string
		live *bool
		want string
	}{
		{"holder alive", new(true), issueopsnextcontract.StageBlockedHolderLive},
		{"holder dead", new(false), issueopsnextcontract.StageTakeover},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ports := testPorts(entries, map[string]issueopscontract.IssueOpsRecord{"io-a": record})
			ports.Actor = func() (string, string, error) { return "claude", "me", nil }
			var probed issueopscontract.NativeProcessReceipt
			ports.ProcessLive = func(receipt issueopscontract.NativeProcessReceipt) *bool {
				probed = receipt
				return scenario.live
			}
			var worktreeRoots []string
			ports.WorktreeState = func(root string) (bool, string, string) {
				worktreeRoots = append(worktreeRoots, root)
				return true, "12-a", "abc"
			}

			result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

			if err != nil || result.Stage.Key != scenario.want {
				t.Fatalf("stage = %q (err %v), want %q", result.Stage.Key, err, scenario.want)
			}
			if probed != process {
				t.Fatalf("the holder's session process must be probed, got %+v", probed)
			}
			if !slices.Contains(worktreeRoots, "/repo.worktrees/12-a") {
				t.Fatalf("the execution workspace must be observed: %v", worktreeRoots)
			}
			if result.Selected == nil || result.Selected.WorkspaceRoot != "/repo.worktrees/12-a" {
				t.Fatalf("selected entry must carry the execution workspace: %+v", result.Selected)
			}
		})
	}
}

func TestNextSelectedEntryCarriesRecordLinks(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-a", Repo: "/repo", Branch: "12-a", Phase: issueopscontract.IssueOpsPhasePlan,
		IssueURL:       "https://example.test/issues/12",
		RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{URL: "https://example.test/pull/9"},
	}
	ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: record.Phase}}, map[string]issueopscontract.IssueOpsRecord{"io-a": record})

	result, err := NewService(ports).Next(t.Context(), "/state", "/repo", "")

	if err != nil || result.Selected == nil {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	got := *result.Selected
	if got.IssueURL != record.IssueURL || got.RemoteArtifactURL != "https://example.test/pull/9" || got.Branch != "12-a" || got.Phase != "plan" {
		t.Fatalf("selected entry must reflect the full record: %+v", got)
	}
}

func TestCommandKindTreatsPlaceholdersAsTemplates(t *testing.T) {
	for command, want := range map[string]string{
		"issueops status --id io-a --json":        issueopsnextcontract.NextCommandKindExact,
		"issueops start --repo <source_root>":     issueopsnextcontract.NextCommandKindTemplate,
		"issueops claim $ACTOR_FLAGS --id io-a":   issueopsnextcontract.NextCommandKindTemplate,
		"issueops execution replace --id <id>":    issueopsnextcontract.NextCommandKindTemplate,
		strings.Repeat("issueops ", 2) + "--json": issueopsnextcontract.NextCommandKindExact,
	} {
		if got := commandKind(command); got != want {
			t.Errorf("commandKind(%q) = %q, want %q", command, got, want)
		}
	}
}
