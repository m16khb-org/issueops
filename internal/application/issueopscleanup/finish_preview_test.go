package issueopscleanup_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

type finishPreviewEnvironment struct {
	calls        []string
	branchCode   int
	branchOID    string
	directoryErr error
	directory    bool
	run          func([]string) (int, string)
}

func (e *finishPreviewEnvironment) Git(_ string, args ...string) (int, string) {
	e.calls = append(e.calls, args[0])
	if e.run != nil {
		return e.run(args)
	}
	if args[0] == "rev-parse" {
		return e.branchCode, e.branchOID
	}
	return 0, ""
}
func (e *finishPreviewEnvironment) Directory(string) (bool, error) {
	return e.directory, e.directoryErr
}
func (*finishPreviewEnvironment) SamePath(a, b string) bool      { return a == b }
func (*finishPreviewEnvironment) PathWithin(string, string) bool { return false }

func TestFinishPreviewRequiresObservableLocalBranch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		oid     string
		blocked bool
	}{
		{"present", 0, "head", false}, {"absent", 1, "", false},
		{"repository failure", 128, "", true}, {"process failure", -1, "", true},
		{"empty success", 0, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := &finishPreviewEnvironment{branchCode: tc.code, branchOID: tc.oid}
			record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Branch: "123-cleanup", Phase: model.IssueOpsPhaseDone}
			_, result := (app.FinishPreviewer{Environment: env}).Plan(context.Background(), record, model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true})
			if slices.Contains(result.Missing, "local_branch_observable") != tc.blocked || result.OK == tc.blocked {
				t.Fatalf("result=%+v", result)
			}
			if !reflect.DeepEqual(env.calls, []string{"rev-parse", "ls-remote"}) {
				t.Fatalf("calls=%v", env.calls)
			}
		})
	}
}

func TestFinishPreviewRequiresObservableWorktree(t *testing.T) {
	env := &finishPreviewEnvironment{branchCode: 1, directoryErr: errors.New("permission denied")}
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", WorktreePath: "/workspace", Phase: model.IssueOpsPhaseDone}
	_, result := (app.FinishPreviewer{Environment: env}).Plan(context.Background(), record, model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true})
	if result.OK || !slices.Contains(result.Missing, "worktree_observable") {
		t.Fatalf("result=%+v", result)
	}
}

func TestFinishPreviewPreservesObservationOrderAndBoundTargets(t *testing.T) {
	env := &finishPreviewEnvironment{directory: true, run: func(args []string) (int, string) {
		if args[0] == "rev-parse" {
			return 0, "head"
		}
		if args[0] == "ls-remote" && args[1] == "--symref" {
			return 0, "ref: refs/heads/main\tHEAD\n"
		}
		return 0, ""
	}}
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Branch: "record-branch", Phase: model.IssueOpsPhaseDone,
		BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "parent"},
		Execution:     &model.Execution{Workspace: model.Workspace{Root: "/worktree", Branch: "123-cleanup"}, Lease: model.WriteLease{Status: model.LeaseStatusReleased}},
	}
	type requestContextKey struct{}
	ctx := context.WithValue(context.Background(), requestContextKey{}, "request")
	service := app.FinishPreviewer{Environment: env, Workspace: func(got context.Context, current model.IssueOpsRecord, root string) (app.FinishWorkspaceObservation, []string) {
		if got != ctx || current.ID != record.ID || root != "/worktree" {
			t.Fatal("workspace observation lost context or target")
		}
		env.calls = append(env.calls, "workspace")
		return app.FinishWorkspaceObservation{Occupants: []model.CleanupWorkspaceProcess{{PID: 12}}, Receipts: []model.NativeProcessReceipt{{PID: 12}}, Terminals: []string{"terminal"}, RuntimeReady: true, AppPID: 42}, nil
	}}
	inventory, result := service.Plan(ctx, record, model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true, CWD: "/outside", MergedBaseBranch: "main"})
	if !result.OK || result.RetargetedBase == nil || inventory.Branch != "123-cleanup" || inventory.BranchOID != "head" || inventory.OrcaAppPID != 42 || !inventory.OrcaRuntimeReady || len(inventory.WorkspaceProcesses) != 1 || len(result.WorkspaceProcesses) != 1 || !reflect.DeepEqual(inventory.OrcaTerminals, []string{"terminal"}) {
		t.Fatalf("inventory=%+v result=%+v", inventory, result)
	}
	if !reflect.DeepEqual(env.calls, []string{"ls-remote", "ls-remote", "workspace", "status", "rev-parse", "ls-remote"}) {
		t.Fatalf("calls=%v", env.calls)
	}
}

type withinFinishPreviewEnvironment struct {
	finishPreviewEnvironment
	within []string
}

func (e *withinFinishPreviewEnvironment) PathWithin(path, root string) bool {
	e.within = append(e.within, path+"|"+root)
	return true
}

func TestFinishPreviewFlagsRequesterCWDInsideWorktreeOnlyWhenCWDIsGiven(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Branch: "123-cleanup", WorktreePath: "/worktree", Phase: model.IssueOpsPhaseDone}
	for _, tc := range []struct {
		name       string
		cwd        string
		wantWithin []string
		want       string
	}{
		{"cwd inside worktree", " /worktree/src ", []string{"/worktree/src|/worktree"}, "cwd_outside_worktree"},
		{"blank cwd", "   ", nil, "cwd_unresolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := &withinFinishPreviewEnvironment{finishPreviewEnvironment: finishPreviewEnvironment{directory: true, branchCode: 1}}
			service := app.FinishPreviewer{Environment: env, Workspace: func(context.Context, model.IssueOpsRecord, string) (app.FinishWorkspaceObservation, []string) {
				return app.FinishWorkspaceObservation{}, nil
			}}
			_, result := service.Plan(context.Background(), record, model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true, CWD: tc.cwd})
			if !slices.Contains(result.Missing, tc.want) || !slices.Equal(env.within, tc.wantWithin) {
				t.Fatalf("missing=%v within=%v", result.Missing, env.within)
			}
			if tc.want == "cwd_unresolved" && slices.Contains(result.Missing, "cwd_outside_worktree") {
				t.Fatalf("blank cwd judged against the worktree: %v", result.Missing)
			}
		})
	}
}
