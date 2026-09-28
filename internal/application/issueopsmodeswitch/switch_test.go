package issueopsmodeswitch

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type switchEffects struct {
	steps  []string
	failAt string
}

func (f *switchEffects) RemoveWorktree(context.Context, string, string) error {
	f.steps = append(f.steps, "worktree")
	if f.failAt == "worktree" {
		return errors.New("worktree failed")
	}
	return nil
}

func (f *switchEffects) RemoveBranch(context.Context, string, string) error {
	f.steps = append(f.steps, "branch")
	if f.failAt == "branch" {
		return errors.New("branch failed")
	}
	return nil
}

func (f *switchEffects) ResetExecution(context.Context, string, string) error {
	f.steps = append(f.steps, "record")
	if f.failAt == "record" {
		return errors.New("record failed")
	}
	return nil
}

func TestApplyOrdersWorkspaceRemovalBeforeRecordReset(t *testing.T) {
	for _, tt := range []struct {
		name, failAt string
		request      ApplyRequest
		wantSteps    []string
		wantError    bool
	}{
		{name: "all present", request: ApplyRequest{ID: "id", Repo: "repo", WorktreeRoot: "root", WorktreePresent: true, Branch: "branch", BranchPresent: true, ExpectedRecordSHA: "sha"}, wantSteps: []string{"worktree", "branch", "record"}},
		{name: "nothing to remove", request: ApplyRequest{ID: "id", Repo: "repo", ExpectedRecordSHA: "sha"}, wantSteps: []string{"record"}},
		{name: "worktree failure", failAt: "worktree", request: ApplyRequest{ID: "id", Repo: "repo", WorktreeRoot: "root", WorktreePresent: true, Branch: "branch", BranchPresent: true, ExpectedRecordSHA: "sha"}, wantSteps: []string{"worktree"}, wantError: true},
		{name: "branch failure", failAt: "branch", request: ApplyRequest{ID: "id", Repo: "repo", WorktreeRoot: "root", WorktreePresent: true, Branch: "branch", BranchPresent: true, ExpectedRecordSHA: "sha"}, wantSteps: []string{"worktree", "branch"}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			effects := &switchEffects{failAt: tt.failAt}
			err := Apply(context.Background(), tt.request, effects)
			if (err != nil) != tt.wantError || !reflect.DeepEqual(effects.steps, tt.wantSteps) {
				t.Fatalf("steps=%v err=%v, want steps=%v error=%v", effects.steps, err, tt.wantSteps, tt.wantError)
			}
		})
	}
}
