package issueopscleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

type auditRecords struct {
	current          model.IssueOpsRecord
	loadErr, saveErr error
	lockErr          error
	saved            []model.IssueOpsRecord
	inLock           bool
	loadedInsideLock bool
	savedInsideLock  bool
}

func (r *auditRecords) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	if r.lockErr != nil {
		return r.lockErr
	}
	r.inLock = true
	defer func() { r.inLock = false }()
	return fn(ctx)
}
func (r *auditRecords) Load(string) (model.IssueOpsRecord, error) {
	r.loadedInsideLock = r.inLock
	return r.current, r.loadErr
}
func (r *auditRecords) Save(_ context.Context, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	r.savedInsideLock = r.inLock
	r.saved = append(r.saved, record)
	return record, r.saveErr
}

func auditRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", Branch: "1-work",
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", Branch: "1-work", BaseBranch: "main", BaseSHA: "abc"}}
}

var auditResult = model.CleanupLinkedBranchResult{State: "deleted", StateReason: "merged", LinkedBranchID: "lb-1", LinkedCount: 1, RemoteRefOID: "oid", Fingerprint: "fp", Deleted: true}

func auditRecorder(records *auditRecords) LinkedBranchAuditRecorder {
	return LinkedBranchAuditRecorder{Records: records, Now: func() time.Time { return time.Date(2026, 9, 29, 3, 4, 5, 0, time.FixedZone("KST", 9*3600)) }}
}

func TestLinkedBranchAuditRecordsDispositionUnderLockOnLatestRecord(t *testing.T) {
	current := auditRecord()
	current.UpdatedAt = "concurrent-update"
	records := &auditRecords{current: current}
	saved, message := auditRecorder(records).Record(t.Context(), auditRecord(), auditResult)
	if !saved || message != "" || len(records.saved) != 1 || !records.loadedInsideLock || !records.savedInsideLock {
		t.Fatalf("saved=%v message=%q records=%+v", saved, message, records)
	}
	got := records.saved[0]
	if got.UpdatedAt != "concurrent-update" {
		t.Fatal("audit overwrote a concurrent mutation with the earlier snapshot")
	}
	audit := got.LinkedBranchCleanup
	if audit == nil || audit.State != "deleted" || audit.LinkedBranchID != "lb-1" || !audit.Deleted || audit.Fingerprint != "fp" || audit.ObservedAt != "2026-09-28T18:04:05Z" {
		t.Fatalf("audit=%+v", audit)
	}
}

func TestLinkedBranchAuditFailuresAreReportedWithoutWriting(t *testing.T) {
	loadFailure, saveFailure, lockFailure := errors.New("record vanished"), errors.New("disk full"), errors.New("lock timeout")
	moved := auditRecord()
	moved.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", Branch: "1-work", BaseBranch: "main", BaseSHA: "different"}
	cases := []struct {
		name      string
		records   *auditRecords
		wantMsg   string
		wantSaves int
	}{
		{name: "lock unavailable", records: &auditRecords{current: auditRecord(), lockErr: lockFailure}, wantMsg: lockFailure.Error()},
		{name: "record deleted meanwhile", records: &auditRecords{loadErr: loadFailure}, wantMsg: loadFailure.Error()},
		{name: "audit target changed", records: &auditRecords{current: moved}, wantMsg: "linked branch cleanup audit target changed during observation"},
		{name: "save fails", records: &auditRecords{current: auditRecord(), saveErr: saveFailure}, wantMsg: saveFailure.Error(), wantSaves: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved, message := auditRecorder(tc.records).Record(t.Context(), auditRecord(), auditResult)
			if saved || message != tc.wantMsg || len(tc.records.saved) != tc.wantSaves {
				t.Fatalf("saved=%v message=%q saves=%d", saved, message, len(tc.records.saved))
			}
		})
	}
}
