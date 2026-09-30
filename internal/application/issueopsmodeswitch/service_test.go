package issueopsmodeswitch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

type switchFixture struct {
	record         model.IssueOpsRecord
	events         []string
	locked         bool
	drift          bool
	failAt         string
	clean          bool
	remoteCount    string
	remoteReadable bool
	baseCount      string
	baseReadable   bool
	remoteBranch   bool
}

func newSwitchFixture() *switchFixture {
	return &switchFixture{record: model.IssueOpsRecord{ID: "io-0123456789ab", Repo: "repo", Branch: "branch", WorktreePath: "worktree", PlanPath: "plan", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{Root: "worktree", Branch: "branch", BaseHead: "base"}, Lease: model.WriteLease{Generation: 3, Status: model.LeaseStatusReleased}}}, clean: true, baseCount: "0", baseReadable: true}
}
func (f *switchFixture) service() Service {
	return Service{Records: f, Workspace: f, Now: func() string { f.events = append(f.events, "now"); return "now" }}
}
func (f *switchFixture) Load(string) (model.IssueOpsRecord, error) {
	f.events = append(f.events, "load")
	record := f.record
	if f.locked && f.drift {
		record.UpdatedAt = "changed"
	}
	return record, nil
}
func (f *switchFixture) Save(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	f.events = append(f.events, "save")
	if !f.locked {
		return record, errors.New("save outside lock")
	}
	if f.failAt == "save" {
		return record, errors.New("save failed")
	}
	f.record = record
	return record, nil
}
func (f *switchFixture) WithinLock(_ context.Context, _ string, fn func() error) error {
	f.events = append(f.events, "lock")
	f.locked = true
	defer func() { f.locked = false; f.events = append(f.events, "unlock") }()
	return fn()
}
func (f *switchFixture) Present(string) bool { return true }
func (f *switchFixture) Clean(string) bool   { return f.clean }
func (f *switchFixture) CommitCount(_, ref string) (string, bool) {
	f.events = append(f.events, "count:"+ref)
	if strings.HasPrefix(ref, "refs/remotes/") {
		return f.remoteCount, f.remoteReadable
	}
	return f.baseCount, f.baseReadable
}
func (f *switchFixture) BranchOID(_, _ string, remote bool) (string, bool) {
	if remote {
		return "remote", f.remoteBranch
	}
	return "local", true
}
func (f *switchFixture) RemoveWorktree(context.Context, string, string) error {
	f.events = append(f.events, "worktree")
	if f.locked {
		return errors.New("Git mutation under lock")
	}
	if f.failAt == "worktree" {
		return errors.New("worktree failed")
	}
	return nil
}
func (f *switchFixture) RemoveBranch(context.Context, string, string) error {
	f.events = append(f.events, "branch")
	if f.locked {
		return errors.New("Git mutation under lock")
	}
	if f.failAt == "branch" {
		return errors.New("branch failed")
	}
	return nil
}

func TestServiceApplyPreservesEffectOrderAndFencesTheReset(t *testing.T) {
	for _, tt := range []struct {
		name, failAt string
		drift        bool
		want         []string
	}{
		{name: "success", want: []string{"worktree", "branch", "lock", "load", "save", "unlock", "now"}},
		{name: "worktree failure", failAt: "worktree", want: []string{"worktree"}},
		{name: "branch failure", failAt: "branch", want: []string{"worktree", "branch"}},
		{name: "record drift", drift: true, want: []string{"worktree", "branch", "lock", "load", "unlock"}},
		{name: "save failure", failAt: "save", want: []string{"worktree", "branch", "lock", "load", "save", "unlock"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSwitchFixture()
			s := f.service()
			req := model.ExecutionSwitchModeRequest{ID: f.record.ID, Mode: "orca"}
			preview, err := s.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			f.events = nil
			f.failAt = tt.failAt
			f.drift = tt.drift
			req.Apply = true
			req.Confirm = true
			req.Fingerprint = preview.Fingerprint
			got, err := s.Run(context.Background(), req)
			want := append([]string{"load", "count:refs/remotes/origin/branch", "count:base"}, tt.want...)
			if !reflect.DeepEqual(f.events, want) {
				t.Fatalf("events=%v want=%v", f.events, want)
			}
			success := tt.failAt == "" && !tt.drift
			if success {
				if err != nil || !got.OK || got.SwitchedAt != "now" || got.NextCommand != "" || got.NextAction == "" || got.WorktreePresent || got.BranchPresent {
					t.Fatalf("result=%+v err=%v", got, err)
				}
				if f.record.Execution != nil || f.record.WorktreePath != "" || f.record.PlanPath != "" {
					t.Fatalf("reset=%+v", f.record)
				}
			} else {
				if err == nil || got.OK || got.SwitchedAt != "" || f.record.Execution == nil || f.record.WorktreePath != "worktree" {
					t.Fatalf("failed apply lost record: %+v err=%v", got, err)
				}
				if tt.drift && !strings.Contains(err.Error(), "authority changed") {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestServiceRejectsBeforeMutationAndOnlyFallsBackOnUnreadableRemote(t *testing.T) {
	for _, tt := range []struct {
		name       string
		configure  func(*switchFixture, *model.ExecutionSwitchModeRequest)
		wantError  string
		wantCounts []string
	}{
		{name: "confirmation before stale fingerprint", configure: func(_ *switchFixture, r *model.ExecutionSwitchModeRequest) {
			r.Confirm = false
			r.Fingerprint = "stale"
		}, wantError: "requires --confirm", wantCounts: []string{"count:refs/remotes/origin/branch", "count:base"}},
		{name: "stale fingerprint", configure: func(_ *switchFixture, r *model.ExecutionSwitchModeRequest) { r.Fingerprint = "stale" }, wantError: "stale switch-mode fingerprint", wantCounts: []string{"count:refs/remotes/origin/branch", "count:base"}},
		{name: "dirty workspace before confirmation", configure: func(f *switchFixture, r *model.ExecutionSwitchModeRequest) { f.clean = false; r.Confirm = false }, wantError: "worktree_clean", wantCounts: []string{"count:refs/remotes/origin/branch", "count:base"}},
		{name: "remote commits cannot use clean base", configure: func(f *switchFixture, _ *model.ExecutionSwitchModeRequest) {
			f.remoteReadable = true
			f.remoteCount = "1"
		}, wantError: "worktree_commits_pushed", wantCounts: []string{"count:refs/remotes/origin/branch"}},
		{name: "both refs unreadable", configure: func(f *switchFixture, _ *model.ExecutionSwitchModeRequest) { f.baseReadable = false }, wantError: "worktree_commits_pushed", wantCounts: []string{"count:refs/remotes/origin/branch", "count:base"}},
		{name: "Orca remote collision", configure: func(f *switchFixture, _ *model.ExecutionSwitchModeRequest) { f.remoteBranch = true }, wantError: "orca_branch_name_free", wantCounts: []string{"count:refs/remotes/origin/branch", "count:base"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSwitchFixture()
			s := f.service()
			req := model.ExecutionSwitchModeRequest{ID: f.record.ID, Mode: "orca"}
			preview, err := s.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			f.events = nil
			req.Apply = true
			req.Confirm = true
			req.Fingerprint = preview.Fingerprint
			tt.configure(f, &req)
			got, err := s.Run(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) || got.OK {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			want := append([]string{"load"}, tt.wantCounts...)
			if !reflect.DeepEqual(f.events, want) {
				t.Fatalf("denial performed effects: %v want %v", f.events, want)
			}
			if f.record.Execution == nil {
				t.Fatal("denial cleared execution")
			}
		})
	}
}
