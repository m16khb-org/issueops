package issueops

import (
	model "issueops/internal/contract/issueops"
	"reflect"
	"strings"
	"testing"
)

func TestCleanupFinishPreviewRequiresIndependentEvidence(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Phase: model.IssueOpsPhasePlan,
		BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		Execution:     &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusActive}},
		IssueLinks:    []model.IssueOpsIssueLink{{Type: "child", URL: "child"}},
	}
	inventory := model.CleanupFinishInventory{WorktreeRoot: "/worktree", WorktreePresent: true, Branch: "123-cleanup"}
	facts := CleanupFinishObservation{WorktreeIdentityConflict: true, WorkspaceMissing: []string{"workspace_processes_observable"}}
	result := BuildCleanupFinishPreview(record, model.CleanupFinishRequest{}, inventory, facts)
	want := []string{"phase_done", "lease_released", "remote_artifact_merged", "completion_reflected", "issue_closed", "merged_base_branch_unobserved", "child_tasks_closed", "worktree_identity_conflict", "cwd_unresolved", "workspace_processes_observable", "worktree_clean", "local_branch_observable", "remote_branch_absent"}
	if result.OK || !reflect.DeepEqual(result.Missing, want) {
		t.Fatalf("result=%+v", result)
	}
	if record.IssueLinks[0].CloseVerifiedAt != "" || inventory.SupersededBy != "" {
		t.Fatal("inputs mutated")
	}
}

func TestCleanupFinishTargetsPreferExecutionAndPreserveLinkedFallback(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Branch: "record-branch", WorktreePath: " /linked ", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "artifact"}}
	if got := CleanupFinishTargets(record); got.WorktreeRoot != "/linked" || got.Branch != "record-branch" || got.RemoteURL != "artifact" {
		t.Fatalf("targets=%+v", got)
	}
	record.Execution = &model.Execution{Workspace: model.Workspace{Root: " /execution ", Branch: " execution-branch "}}
	if got := CleanupFinishTargets(record); got.WorktreeRoot != "/execution" || got.Branch != "execution-branch" {
		t.Fatalf("targets=%+v", got)
	}
}

// done 전이는 draft PR 생성 직후에 일어나고 finish는 머지 이후에 실행되므로, 그
// 사이 구간에서 draft PR의 base가 바뀔 수 있다. 준비된 base가 아닌 브랜치로
// 머지된 결과를 파괴 전에 잡지 못하면 재검증 수단이 남지 않는다.
// #490: 부모 브랜치가 머지·삭제되어 provider가 PR을 기본 브랜치로 재타깃한
// 흐름은 drift가 아니다. 준비 base의 원격 부재와 기본 브랜치 일치라는 두
// 관측으로만 통과시키고, 나머지는 그대로 거부한다.
func TestClassifyMergedBaseRetarget(t *testing.T) {
	cases := []struct {
		name                              string
		prepared, observed, defaultBranch string
		preparedRemotePresent, observed_  bool
		want                              []string
	}{
		{name: "same base passes", prepared: "main", observed: "main", defaultBranch: "main", observed_: true},
		{name: "no prepared base is not judged", prepared: "", observed: "release", defaultBranch: "main", observed_: true},
		{name: "retarget to default after parent merged", prepared: "484-parent", observed: "main", defaultBranch: "main", observed_: true},
		{name: "parent branch still present is drift", prepared: "484-parent", observed: "main", defaultBranch: "main", preparedRemotePresent: true, observed_: true, want: []string{"base_branch_drifted"}},
		{name: "merged into a non-default branch is drift", prepared: "484-parent", observed: "release", defaultBranch: "main", observed_: true, want: []string{"base_branch_drifted"}},
		{name: "unobserved keeps the drift fact and names the missing observation", prepared: "484-parent", observed: "main", defaultBranch: "main", observed_: false, want: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
		{name: "empty default branch fails closed", prepared: "484-parent", observed: "main", defaultBranch: "", observed_: true, want: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyCleanupMergedBase(tc.prepared, tc.observed, tc.defaultBranch, tc.preparedRemotePresent, tc.observed_)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("classifyMergedBase = %v, want %v", got, tc.want)
			}
		})
	}
}
