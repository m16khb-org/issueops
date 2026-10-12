package issueopscleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

type linkedBranchTestRecords struct {
	record model.IssueOpsRecord
	saved  []model.IssueOpsRecord
}

func (r *linkedBranchTestRecords) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *linkedBranchTestRecords) Load(string) (model.IssueOpsRecord, error) { return r.record, nil }
func (r *linkedBranchTestRecords) Save(_ context.Context, rec model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	r.saved = append(r.saved, rec)
	return rec, nil
}

type linkedBranchFixture struct {
	cleaner LinkedBranchCleaner
	records *linkedBranchTestRecords
	nodes   []linkedbranch.Node
	remote  string
	deleted []string
}

func newLinkedBranchFixture() *linkedBranchFixture {
	f := &linkedBranchFixture{}
	f.records = &linkedBranchTestRecords{record: model.IssueOpsRecord{
		ID: "io-linked", Repo: "/repo",
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/o/r/issues/1", Branch: "1-feature", BaseSHA: "base-sha"},
	}}
	f.nodes = []linkedbranch.Node{{ID: "LB_orphan"}}
	f.cleaner = LinkedBranchCleaner{
		Records: f.records,
		RemoteRef: func(context.Context, string, string) (string, error) {
			return f.remote, nil
		},
		ObserveLinkedBranches: func(context.Context, string) (linkedbranch.Observation, error) {
			return linkedbranch.Observation{TotalCount: len(f.nodes), Nodes: f.nodes}, nil
		},
		DeleteLinkedBranch: func(_ context.Context, _, id string) error {
			f.deleted = append(f.deleted, id)
			return nil
		},
		Now: func() time.Time { return time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC) },
	}
	return f
}

func TestLinkedBranchCleanerPreviewNamesOrphanWithoutDeleting(t *testing.T) {
	f := newLinkedBranchFixture()
	result, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Preview || result.Deleted || result.LinkedBranchID != "LB_orphan" || result.State != string(linkedbranch.StateOrphan) || result.Fingerprint == "" {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(result.NextCommand, "--apply --confirm --fingerprint "+result.Fingerprint) || len(f.deleted) != 0 {
		t.Fatalf("next=%q deleted=%v", result.NextCommand, f.deleted)
	}
}

func TestLinkedBranchCleanerApplyDeletesOnlyWithConfirmedCurrentFingerprint(t *testing.T) {
	f := newLinkedBranchFixture()
	preview, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	if err != nil {
		t.Fatal(err)
	}

	step := func(req model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
		return f.cleaner.Run(context.Background(), req)
	}
	res, err := step(model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Fingerprint: preview.Fingerprint})
	if err == nil || res.FailedStep != "confirm" || res.OK || len(f.deleted) != 0 {
		t.Fatalf("unconfirmed apply: res=%+v err=%v deleted=%v", res, err, f.deleted)
	}
	res, err = step(model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: "stale"})
	if err == nil || res.FailedStep != "stale_fingerprint" || len(f.deleted) != 0 {
		t.Fatalf("stale apply: res=%+v err=%v deleted=%v", res, err, f.deleted)
	}
	res, err = step(model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint})
	if err != nil || !res.Deleted || res.DeletedAt != "2026-09-29T01:02:03Z" || len(f.deleted) != 1 || f.deleted[0] != "LB_orphan" {
		t.Fatalf("apply: res=%+v err=%v deleted=%v", res, err, f.deleted)
	}
}

func TestLinkedBranchCleanerRefusesWhenRemoteBranchAppearsAfterPreview(t *testing.T) {
	f := newLinkedBranchFixture()
	preview, _ := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	f.remote = "late-oid" // a ref converging after preview makes the orphan ambiguous
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint})
	if err == nil || res.FailedStep != "classify_linked_branch" || len(f.deleted) != 0 {
		t.Fatalf("res=%+v err=%v deleted=%v", res, err, f.deleted)
	}
}

func TestLinkedBranchCleanerRejectsStaleFingerprintWhenOrphanIdentityChanges(t *testing.T) {
	f := newLinkedBranchFixture()
	preview, _ := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	f.nodes = []linkedbranch.Node{{ID: "LB_other"}}
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint})
	if err == nil || res.FailedStep != "stale_fingerprint" || len(f.deleted) != 0 {
		t.Fatalf("res=%+v err=%v deleted=%v", res, err, f.deleted)
	}
}

func TestLinkedBranchCleanerAbsentIsIdempotentSuccessBeforeFingerprintCheck(t *testing.T) {
	f := newLinkedBranchFixture()
	f.nodes = nil
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: "anything"})
	if err != nil || !res.OK || !res.AlreadyAbsent || res.Deleted || len(f.deleted) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestLinkedBranchCleanerRefusesNonOrphanStates(t *testing.T) {
	f := newLinkedBranchFixture()
	f.nodes = []linkedbranch.Node{{ID: "a"}, {ID: "b"}} // two null refs cannot be attributed
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true})
	if err == nil || !strings.Contains(err.Error(), "refuses state ambiguous") || res.OK || res.FailedStep != "classify_linked_branch" || len(f.deleted) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestLinkedBranchCleanerReportsNotReadyGates(t *testing.T) {
	f := newLinkedBranchFixture()
	f.cleaner.DeleteLinkedBranch = nil
	f.records.record.BranchPrepare.Provider = "gitlab"
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	if err == nil || !strings.Contains(err.Error(), "linked_branch_deletion_unavailable") || !strings.Contains(err.Error(), "linked_branch_cleanup_is_github_only") || res.OK || len(res.Missing) != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestLinkedBranchCleanerMapsObservationAndDeleteFailuresToSteps(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name  string
		setup func(*linkedBranchFixture)
		step  string
		apply bool
	}{
		{"observe linked", func(f *linkedBranchFixture) {
			f.cleaner.ObserveLinkedBranches = func(context.Context, string) (linkedbranch.Observation, error) {
				return linkedbranch.Observation{}, boom
			}
		}, "observe_linked_branches", false},
		{"observe remote", func(f *linkedBranchFixture) {
			f.cleaner.RemoteRef = func(context.Context, string, string) (string, error) { return "", boom }
		}, "observe_remote_ref", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLinkedBranchFixture()
			tc.setup(f)
			res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
			if !errors.Is(err, boom) || res.OK || res.FailedStep != tc.step || res.ObserveError != "boom" {
				t.Fatalf("res=%+v err=%v", res, err)
			}
		})
	}

	f := newLinkedBranchFixture()
	preview, _ := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	f.cleaner.DeleteLinkedBranch = func(context.Context, string, string) error { return boom }
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint})
	if !errors.Is(err, boom) || res.OK || res.FailedStep != "delete_linked_branch" || res.Deleted {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestLinkedBranchCleanerAuditsStaleFingerprintButNotUnconfirmedApply(t *testing.T) {
	f := newLinkedBranchFixture()
	preview, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Fingerprint: preview.Fingerprint})
	if err == nil || res.FailedStep != "confirm" || res.AuditRecorded || len(f.records.saved) != 0 {
		t.Fatalf("unconfirmed apply must not be audited: res=%+v err=%v saved=%d", res, err, len(f.records.saved))
	}
	res, err = f.cleaner.Run(context.Background(), model.CleanupLinkedBranchRequest{ID: "io-linked", Apply: true, Confirm: true, Fingerprint: "stale"})
	if err == nil || res.FailedStep != "stale_fingerprint" || !res.AuditRecorded || len(f.records.saved) != 1 {
		t.Fatalf("stale apply must be audited once: res=%+v err=%v saved=%d", res, err, len(f.records.saved))
	}
}
