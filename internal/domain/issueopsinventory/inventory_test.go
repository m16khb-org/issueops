package issueopsinventory

import (
	"strconv"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// ProjectEntry는 list CLI가 렌더하는 전부다. 레코드의 실행/원격/정리 상태가
// 엔트리 필드로 정확히 투영되는지 잠근다.
func TestProjectEntryProjectsExecutionAndRemoteState(t *testing.T) {
	record := issueopscontract.Record{
		ID: "io-proj", Repo: "/repo", Branch: "12-proj",
		Phase: issueopscontract.PhaseImplement, UpdatedAt: "2026-08-21T00:00:00Z",
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/12-proj"},
			Lease: issueopscontract.WriteLease{
				Status: issueopscontract.LeaseStatusClaimable,
			},
			Pending: &issueopscontract.ExternalIntent{Kind: "gh_run", StartedAt: "2026-08-21T01:00:00Z"},
			Failure: &issueopscontract.ExecutionFailure{Code: "E_TEST", At: "2026-08-21T02:00:00Z"},
		},
		RemoteArtifact: &issueopscontract.RemoteArtifactVerification{
			URL: "https://github.com/acme/repo/pull/12",
		},
	}
	entry := ProjectEntry(record)
	if entry.ID != "io-proj" || entry.Repo != "/repo" || entry.Branch != "12-proj" ||
		string(entry.Phase) != "implement" || entry.UpdatedAt != "2026-08-21T00:00:00Z" {
		t.Fatalf("base projection wrong: %+v", entry)
	}
	if entry.LeaseStatus != "claimable" || !entry.Claimable {
		t.Fatalf("lease projection wrong: %+v", entry)
	}
	if entry.WorkspaceRoot != "/repo.worktrees/12-proj" {
		t.Fatalf("workspace projection wrong: %+v", entry)
	}
	if entry.PendingKind != "gh_run" || entry.PendingSince != "2026-08-21T01:00:00Z" {
		t.Fatalf("pending projection wrong: %+v", entry)
	}
	if entry.FailureCode != "E_TEST" || entry.FailureAt != "2026-08-21T02:00:00Z" {
		t.Fatalf("failure projection wrong: %+v", entry)
	}
	if entry.RemoteArtifactURL != "https://github.com/acme/repo/pull/12" || !entry.CompletionUnreflected {
		t.Fatalf("remote artifact projection wrong: %+v", entry)
	}
	if entry.CleanupCandidate {
		t.Fatalf("implement phase must not be a cleanup candidate: %+v", entry)
	}
	// 가장 늦은 기록 시각은 UpdatedAt(00:00)이 아니라 Failure.At(02:00)이다.
	if entry.LastRecordAt != "2026-08-21T02:00:00Z" {
		t.Fatalf("last record at = %q, want the failure time", entry.LastRecordAt)
	}
}

func TestProjectEntryProjectsHolderAndCompletionReflection(t *testing.T) {
	record := issueopscontract.Record{
		ID: "io-holder", Phase: issueopscontract.PhasePR,
		Execution: &issueopscontract.Execution{
			Lease: issueopscontract.WriteLease{
				Status: issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{Host: "omo", SessionID: "sess-1"},
			},
		},
		RemoteArtifact:   &issueopscontract.RemoteArtifactVerification{URL: "u"},
		RemoteCompletion: &issueopscontract.RemoteCompletion{ReflectedAt: "2026-08-21T03:00:00Z"},
	}
	entry := ProjectEntry(record)
	if entry.HolderHost != "omo" || entry.HolderSession != "sess-1" {
		t.Fatalf("holder projection wrong: %+v", entry)
	}
	if entry.CompletionUnreflected || entry.Claimable {
		t.Fatalf("reflected completion must clear unreflected flag: %+v", entry)
	}

	record.Phase = issueopsinventorycontract.PhaseDone
	if entry := ProjectEntry(record); !entry.CleanupCandidate {
		t.Fatalf("done phase must be a cleanup candidate: %+v", entry)
	}
	if entry.LastRecordAt != "" {
		t.Fatalf("record without timestamps must have no last record at: %+v", entry)
	}
}

func TestProjectEntryProjectsCleanupFailuresAndIssueIntent(t *testing.T) {
	record := issueopscontract.Record{
		ID: "io-cleanup", Phase: issueopscontract.PhasePR,
		CleanupFinishFailure: &issueopscontract.CleanupFinishFailure{
			Step: "close_issue", At: "t1",
		},
		IssueCreateIntent: &issueopscontract.IssueCreateIntent{Status: "pending"},
	}
	entry := ProjectEntry(record)
	if entry.CleanupFailureStep != "close_issue" || entry.CleanupFailureAt != "t1" {
		t.Fatalf("cleanup failure projection wrong: %+v", entry)
	}
	if entry.IssueCreateStatus != "pending" {
		t.Fatalf("issue create status projection wrong: %+v", entry)
	}
	// "t1"은 RFC3339가 아니라서 last_record_at에 반영되지 않는다.
	if entry.LastRecordAt != "" {
		t.Fatalf("unparseable times must leave last record at empty: %+v", entry)
	}
	record.CleanupFinishFailure = nil
	record.CleanupAbandonFailure = &issueopscontract.CleanupAbandonFailure{
		Step: "branch_delete", At: "t2",
	}
	entry = ProjectEntry(record)
	if entry.CleanupFailureStep != "branch_delete" || entry.CleanupFailureAt != "t2" {
		t.Fatalf("abandon failure projection wrong: %+v", entry)
	}
	if entry.LastRecordAt != "" {
		t.Fatalf("unparseable times must leave last record at empty: %+v", entry)
	}
	// 완료된 issue create intent은 list에 노출하지 않는다.
	record.IssueCreateIntent.Status = "completed"
	if entry := ProjectEntry(record); entry.IssueCreateStatus != "" {
		t.Fatalf("completed intent must not surface: %+v", entry)
	}
}

// lastRecordAtFull은 11개 기록 시각이 모두 early인 record다. 각 행은 필드 하나만
// late로 바꿔 그 필드가 결과를 정하는지 잠근다.
func lastRecordAtFull(early string) issueopscontract.Record {
	return issueopscontract.Record{
		UpdatedAt: early,
		Execution: &issueopscontract.Execution{
			Lease: issueopscontract.WriteLease{
				ClaimedAt: early, ReleasedAt: early, ReplacedAt: early,
			},
			Pending:    &issueopscontract.ExternalIntent{StartedAt: early},
			Completion: &issueopscontract.ExecutionCompletion{CompletedAt: early},
			Failure:    &issueopscontract.ExecutionFailure{At: early},
		},
		IssueCreateIntent:     &issueopscontract.IssueCreateIntent{UpdatedAt: early},
		CleanupFinishFailure:  &issueopscontract.CleanupFinishFailure{At: early},
		CleanupAbandonFailure: &issueopscontract.CleanupAbandonFailure{At: early},
		RemoteArtifact:        &issueopscontract.RemoteArtifactVerification{VerifiedAt: early},
	}
}

func TestLastRecordAt(t *testing.T) {
	const early = "2026-11-01T00:00:00Z"
	const late = "2026-11-02T00:00:00Z"
	withLate := func(mutate func(*issueopscontract.Record)) issueopscontract.Record {
		record := lastRecordAtFull(early)
		mutate(&record)
		return record
	}
	leaseClaimed := func(updatedAt, claimedAt string) issueopscontract.Record {
		return issueopscontract.Record{
			UpdatedAt: updatedAt,
			Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{ClaimedAt: claimedAt}},
		}
	}
	tests := []struct {
		name   string
		record issueopscontract.Record
		want   string
	}{
		{"UpdatedAt", withLate(func(r *issueopscontract.Record) { r.UpdatedAt = late }), late},
		{"Lease.ClaimedAt", withLate(func(r *issueopscontract.Record) { r.Execution.Lease.ClaimedAt = late }), late},
		{"Lease.ReleasedAt", withLate(func(r *issueopscontract.Record) { r.Execution.Lease.ReleasedAt = late }), late},
		{"Lease.ReplacedAt", withLate(func(r *issueopscontract.Record) { r.Execution.Lease.ReplacedAt = late }), late},
		{"Pending.StartedAt", withLate(func(r *issueopscontract.Record) { r.Execution.Pending.StartedAt = late }), late},
		{"Completion.CompletedAt", withLate(func(r *issueopscontract.Record) { r.Execution.Completion.CompletedAt = late }), late},
		{"Failure.At", withLate(func(r *issueopscontract.Record) { r.Execution.Failure.At = late }), late},
		{"IssueCreateIntent.UpdatedAt", withLate(func(r *issueopscontract.Record) { r.IssueCreateIntent.UpdatedAt = late }), late},
		{"CleanupFinishFailure.At", withLate(func(r *issueopscontract.Record) { r.CleanupFinishFailure.At = late }), late},
		{"CleanupAbandonFailure.At", withLate(func(r *issueopscontract.Record) { r.CleanupAbandonFailure.At = late }), late},
		{"RemoteArtifact.VerifiedAt", withLate(func(r *issueopscontract.Record) { r.RemoteArtifact.VerifiedAt = late }), late},
		{
			"fraction and offset normalize to UTC",
			issueopscontract.Record{UpdatedAt: "2026-11-01T19:00:00.5+09:00"},
			"2026-11-01T10:00:00.5Z",
		},
		{
			"compares instants not strings",
			leaseClaimed("2026-11-01T18:59:59+09:00", "2026-11-01T10:00:00Z"),
			"2026-11-01T10:00:00Z",
		},
		{
			"UpdatedAt later than lease",
			leaseClaimed("2026-11-03T00:00:00Z", "2026-11-01T10:00:00Z"),
			"2026-11-03T00:00:00Z",
		},
		{"all empty", issueopscontract.Record{Execution: &issueopscontract.Execution{}}, ""},
		{"all garbage", leaseClaimed("garbage", "garbage"), ""},
		{"garbage beside a valid value", leaseClaimed("garbage", "2026-11-01T10:00:00Z"), "2026-11-01T10:00:00Z"},
		{"nil execution", issueopscontract.Record{UpdatedAt: "2026-11-01T10:00:00Z"}, "2026-11-01T10:00:00Z"},
		{"execution with nil pointers", issueopscontract.Record{Execution: &issueopscontract.Execution{}}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LastRecordAt(test.record); got != test.want {
				t.Fatalf("LastRecordAt = %q, want %q", got, test.want)
			}
		})
	}
}

func TestIdleDays(t *testing.T) {
	now := time.Date(2026, time.November, 12, 3, 0, 0, 0, time.UTC)
	intPtr := func(value int) *int { return &value }
	tests := []struct {
		name string
		last string
		want *int
	}{
		{"exactly 24h", "2026-11-11T03:00:00Z", intPtr(1)},
		{"23h59m59s", "2026-11-11T03:00:01Z", intPtr(0)},
		{"47h59m", "2026-11-10T03:01:00Z", intPtr(1)},
		{"72h", "2026-11-09T03:00:00Z", intPtr(3)},
		{"offset 23h apart", "2026-11-10T23:00:00-05:00", intPtr(0)},
		{"future", "2026-11-12T03:00:01Z", nil},
		{"empty", "", nil},
		{"garbage", "garbage", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IdleDays(test.last, now)
			if (got == nil) != (test.want == nil) || (got != nil && *got != *test.want) {
				t.Fatalf("IdleDays(%q) = %s, want %s", test.last, formatDays(got), formatDays(test.want))
			}
		})
	}
}

func formatDays(days *int) string {
	if days == nil {
		return "nil"
	}
	return strconv.Itoa(*days)
}
