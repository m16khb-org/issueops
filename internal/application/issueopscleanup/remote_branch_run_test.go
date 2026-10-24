package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

type flexBranchRecords struct {
	snapshot                          model.CleanupSnapshot
	events                            *[]string
	loadErr, armErr, checkErr, relErr error
}

func (r *flexBranchRecords) Load(context.Context, string) (model.CleanupSnapshot, error) {
	*r.events = append(*r.events, "load")
	return r.snapshot, r.loadErr
}
func (r *flexBranchRecords) Arm(_ context.Context, snap model.CleanupSnapshot, a model.IssueOpsCleanupAttempt) (model.CleanupSnapshot, error) {
	*r.events = append(*r.events, "arm")
	snap.Record.CleanupAttempt = &a
	return snap, r.armErr
}
func (r *flexBranchRecords) Check(context.Context, model.CleanupSnapshot) error {
	*r.events = append(*r.events, "check")
	return r.checkErr
}
func (r *flexBranchRecords) Release(_ context.Context, snap model.CleanupSnapshot, _ string) (model.CleanupSnapshot, error) {
	*r.events = append(*r.events, "release")
	return snap, r.relErr
}

type flexBranchLifetime struct {
	events   *[]string
	drainErr error
	name     string
}

func (l *flexBranchLifetime) Context(ctx context.Context) context.Context { return ctx }
func (l *flexBranchLifetime) Close() error {
	*l.events = append(*l.events, "close:"+l.name)
	return nil
}
func (l *flexBranchLifetime) Drain(context.Context) (CleanupLifetime, error) {
	*l.events = append(*l.events, "drain")
	return &flexBranchLifetime{events: l.events, name: "drained"}, l.drainErr
}

type deletingBranchEnv struct {
	previewBranchEnv
	events    *[]string
	deleteErr error
	deleted   []string
}

func (e *deletingBranchEnv) Delete(_ context.Context, repo, branch, oid string) error {
	*e.events = append(*e.events, "delete")
	e.deleted = append(e.deleted, repo+":"+branch+"@"+oid)
	return e.deleteErr
}

type branchRunFixture struct {
	cleaner  RemoteBranchCleaner
	records  *flexBranchRecords
	lifetime *flexBranchLifetime
	env      *deletingBranchEnv
	events   *[]string
}

func newBranchRunFixture() *branchRunFixture {
	events := []string{}
	f := &branchRunFixture{events: &events}
	f.records = &flexBranchRecords{events: &events, snapshot: model.CleanupSnapshot{Record: branchPreviewRecord()}}
	f.lifetime = &flexBranchLifetime{events: &events, name: "original"}
	f.env = &deletingBranchEnv{previewBranchEnv: *newPreviewBranchEnv(), events: &events}
	f.cleaner = RemoteBranchCleaner{
		Records: f.records,
		Acquire: func(context.Context, string) (CleanupLifetime, error) { return f.lifetime, nil },
		NewAttempt: func(op model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
			return model.IssueOpsCleanupAttempt{Operation: op, Token: "token", StartedAt: "now"}, nil
		},
		Preview: RemoteBranchPreviewer{Environment: f.env, VerifyMergedArtifact: mergedHeadFor("tip")},
		Now:     func() time.Time { return time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC) },
	}
	return f
}

func (f *branchRunFixture) applyRequest(t *testing.T) model.CleanupRemoteBranchRequest {
	t.Helper()
	preview, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test"})
	if err != nil || preview.Fingerprint == "" || preview.NextCommand == "" {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	*f.events = (*f.events)[:0]
	return model.CleanupRemoteBranchRequest{ID: "io-test", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint}
}

func TestRemoteBranchCleanerRefusesBeforeArming(t *testing.T) {
	loadFailure, acquireFailure := errors.New("record unreadable"), errors.New("lock busy")
	cases := []struct {
		name      string
		configure func(*branchRunFixture)
		wantErr   string
		wantIs    error
	}{
		{name: "acquire fails", configure: func(f *branchRunFixture) {
			f.cleaner.Acquire = func(context.Context, string) (CleanupLifetime, error) { return nil, acquireFailure }
		}, wantIs: acquireFailure},
		{name: "load fails", configure: func(f *branchRunFixture) { f.records.loadErr = loadFailure }, wantIs: loadFailure},
		{name: "other cleanup owns the cycle", configure: func(f *branchRunFixture) {
			f.records.snapshot.Record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationFinish}
		}, wantErr: "cleanup finish owns this cycle"},
		{name: "gates missing", configure: func(f *branchRunFixture) { f.records.snapshot.Record.Phase = model.IssueOpsPhaseImplement }, wantErr: "cleanup remote-branch is not ready; unmet gates: phase_done"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newBranchRunFixture()
			tc.configure(f)
			result, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test", Apply: true, Confirm: true, Fingerprint: "x"})
			if err == nil || result.OK || result.Deleted {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want %q", err, tc.wantErr)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("err=%v", err)
			}
			if slices.Contains(*f.events, "arm") || slices.Contains(*f.events, "delete") {
				t.Fatalf("effects before gates: %v", *f.events)
			}
		})
	}
}

func TestRemoteBranchCleanerPreviewBindsApplyCommandToFingerprint(t *testing.T) {
	f := newBranchRunFixture()
	result, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test"})
	if err != nil || !result.Preview || !strings.Contains(result.NextCommand, "--apply --confirm --fingerprint "+result.Fingerprint+" --json") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if slices.Contains(*f.events, "arm") {
		t.Fatalf("preview armed the record: %v", *f.events)
	}
}

func TestRemoteBranchCleanerRejectsUnconfirmedOrStaleApply(t *testing.T) {
	for name, req := range map[string]model.CleanupRemoteBranchRequest{
		"no confirmation":   {ID: "io-test", Apply: true, Fingerprint: "irrelevant"},
		"stale fingerprint": {ID: "io-test", Apply: true, Confirm: true, Fingerprint: "stale"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newBranchRunFixture()
			result, err := f.cleaner.Run(t.Context(), req)
			if err == nil || result.OK || result.Deleted || slices.Contains(*f.events, "arm") || len(f.env.deleted) != 0 {
				t.Fatalf("result=%+v err=%v events=%v", result, err, *f.events)
			}
		})
	}
}

func TestRemoteBranchCleanerDeletesObservedOIDAndReleasesAfterDrain(t *testing.T) {
	f := newBranchRunFixture()
	req := f.applyRequest(t)
	result, err := f.cleaner.Run(t.Context(), req)
	if err != nil || !result.OK || !result.Deleted || result.DeletedAt != "2026-09-29T01:02:03Z" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	// The remote branch is the first post-merge step; it hands off to status.
	if result.NextCommand != "issueops cleanup status --id io-test --merged --json" {
		t.Fatalf("next=%q", result.NextCommand)
	}
	if !slices.Equal(f.env.deleted, []string{"/repo:123-work@tip"}) {
		t.Fatalf("deleted=%v", f.env.deleted)
	}
	want := []string{"load", "arm", "check", "delete", "drain", "release", "close:drained"}
	if !slices.Equal(*f.events, want) {
		t.Fatalf("events=%v, want %v", *f.events, want)
	}
}

func TestRemoteBranchCleanerFailuresReleaseOnceAndPointToPreview(t *testing.T) {
	boom, releaseFailure := errors.New("boom"), errors.New("release failed")
	cases := []struct {
		name      string
		configure func(*branchRunFixture)
		wantStep  string
		wantIs    []error
		deleted   bool
		released  bool
	}{
		{name: "record check fails", configure: func(f *branchRunFixture) { f.records.checkErr = boom }, wantStep: "record_check", wantIs: []error{boom}, released: true},
		{name: "remote delete fails", configure: func(f *branchRunFixture) { f.env.deleteErr = boom }, wantStep: "remote_branch_delete", wantIs: []error{boom}, released: true},
		{name: "release fails after delete", configure: func(f *branchRunFixture) { f.records.relErr = releaseFailure }, wantStep: "record_release", wantIs: []error{releaseFailure}, deleted: true, released: true},
		{name: "delete and release both fail", configure: func(f *branchRunFixture) { f.env.deleteErr, f.records.relErr = boom, releaseFailure }, wantStep: "remote_branch_delete", wantIs: []error{boom, releaseFailure}, released: true},
		{name: "drain fails so release is withheld", configure: func(f *branchRunFixture) { f.lifetime.drainErr = boom }, wantStep: "record_release", wantIs: []error{boom}, deleted: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newBranchRunFixture()
			req := f.applyRequest(t)
			tc.configure(f)
			result, err := f.cleaner.Run(t.Context(), req)
			if err == nil || result.OK || result.FailedStep != tc.wantStep || result.Deleted != tc.deleted {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			for _, want := range tc.wantIs {
				if !errors.Is(err, want) {
					t.Fatalf("err=%v does not wrap %v", err, want)
				}
			}
			// A deleted branch whose attempt was not released reads as done-but-unreleased,
			// not as a failed deletion (an agent saw exit 1 next to deleted:true).
			if tc.deleted != strings.Contains(err.Error(), "remote branch deleted") {
				t.Fatalf("deleted=%v err=%v", tc.deleted, err)
			}
			if result.NextCommand != "issueops cleanup remote-branch --id io-test --preview --json" {
				t.Fatalf("next=%q", result.NextCommand)
			}
			if slices.Contains(*f.events, "release") != tc.released {
				t.Fatalf("release=%v, want %v: %v", slices.Contains(*f.events, "release"), tc.released, *f.events)
			}
			if got := (*f.events)[len(*f.events)-1]; got != "close:drained" {
				t.Fatalf("deferred close must target the drained lifetime, last event %q", got)
			}
		})
	}
}

func TestRemoteBranchCleanerReportsArmingFailures(t *testing.T) {
	armFailure, attemptFailure := errors.New("record changed"), errors.New("entropy")
	t.Run("arm", func(t *testing.T) {
		f := newBranchRunFixture()
		req := f.applyRequest(t)
		f.records.armErr = armFailure
		result, err := f.cleaner.Run(t.Context(), req)
		if !errors.Is(err, armFailure) || result.OK || slices.Contains(*f.events, "delete") {
			t.Fatalf("result=%+v err=%v events=%v", result, err, *f.events)
		}
	})
	t.Run("attempt token", func(t *testing.T) {
		f := newBranchRunFixture()
		req := f.applyRequest(t)
		f.cleaner.NewAttempt = func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
			return model.IssueOpsCleanupAttempt{}, attemptFailure
		}
		result, err := f.cleaner.Run(t.Context(), req)
		if !errors.Is(err, attemptFailure) || result.OK || result.ID != "io-test" || slices.Contains(*f.events, "arm") {
			t.Fatalf("result=%+v err=%v events=%v", result, err, *f.events)
		}
	})
}

func TestRemoteBranchCleanerAlreadyAbsentBranch(t *testing.T) {
	t.Run("no attempt recorded returns without arming", func(t *testing.T) {
		f := newBranchRunFixture()
		f.env.remoteOID = ""
		result, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test", Apply: true, Confirm: true})
		if err != nil || !result.AlreadyAbsent || result.Deleted || result.Fingerprint != "" || slices.Contains(*f.events, "arm") {
			t.Fatalf("result=%+v err=%v events=%v", result, err, *f.events)
		}
	})
	t.Run("crashed attempt preview points at apply without writing", func(t *testing.T) {
		f := newBranchRunFixture()
		f.env.remoteOID = ""
		f.records.snapshot.Record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationRemoteBranch, Token: "old", StartedAt: "then"}
		result, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test"})
		if err != nil || !result.AlreadyAbsent || result.NextCommand != "issueops cleanup remote-branch --id io-test --apply --json" || slices.Contains(*f.events, "arm") {
			t.Fatalf("result=%+v err=%v events=%v", result, err, *f.events)
		}
	})
	t.Run("crashed attempt apply releases without deleting", func(t *testing.T) {
		f := newBranchRunFixture()
		f.env.remoteOID = ""
		f.records.snapshot.Record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationRemoteBranch, Token: "old", StartedAt: "then"}
		result, err := f.cleaner.Run(t.Context(), model.CleanupRemoteBranchRequest{ID: "io-test", Apply: true})
		if err != nil || !result.AlreadyAbsent || result.Deleted {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		want := []string{"load", "arm", "drain", "release", "close:drained"}
		if !slices.Equal(*f.events, want) || len(f.env.deleted) != 0 {
			t.Fatalf("events=%v deleted=%v", *f.events, f.env.deleted)
		}
	})
}

func TestCleanupSupersededByFlagQuotesForTheShell(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{" https://github.com/acme/repo/pull/3 ", " --superseded-by 'https://github.com/acme/repo/pull/3'"},
		{"x'; rm -rf /; '", ` --superseded-by 'x'\''; rm -rf /; '\'''`},
	} {
		if got := cleanupSupersededByFlag(tc.in); got != tc.want {
			t.Errorf("cleanupSupersededByFlag(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
