package issueopsretention

import (
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestIsPrunableRequiresResolvedIssueCreateIntent(t *testing.T) {
	cutoff := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	record := issueopscontract.Record{
		Phase:     issueopscontract.PhaseDone,
		UpdatedAt: cutoff.Add(-time.Hour).Format(time.RFC3339Nano),
		IssueCreateIntent: &issueopscontract.IssueCreateIntent{
			Status: issueopscontract.IssueCreateIntentInvokedUnknown,
		},
	}
	if IsPrunable(record, cutoff) {
		t.Fatal("unresolved issue-create intent must retain reconciliation authority")
	}
	record.IssueCreateIntent.Status = issueopscontract.IssueCreateIntentCompleted
	if !IsPrunable(record, cutoff) {
		t.Fatal("completed issue-create intent may be pruned after the cutoff")
	}
}

func TestPruneRetainsFinishAttemptRegardlessOfAge(t *testing.T) {
	cutoff := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	record := issueopscontract.Record{Phase: issueopscontract.PhaseDone, UpdatedAt: "2000-01-01T00:00:00Z", CleanupAttempt: &issueopscontract.CleanupAttempt{Operation: "finish", Token: "pending", StartedAt: "2000-01-01T00:00:00Z"}}
	if IsPrunable(record, cutoff) {
		t.Fatal("retention discarded finish recovery authority")
	}
	record.CleanupAttempt = nil
	if !IsPrunable(record, cutoff) {
		t.Fatal("drained old record cannot be pruned")
	}
}

// pruneAgeCases covers the age rule: a record is old only when its last record
// instant (the latest stored timestamp, the one `issueops list` shows) is
// before cutoff, and a record whose UpdatedAt is not a valid instant is kept.
func pruneAgeCases(cutoff time.Time) []struct {
	name   string
	record issueopscontract.Record
	want   bool
} {
	old := cutoff.Add(-90 * 24 * time.Hour).Format(time.RFC3339Nano)
	older := cutoff.Add(-91 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := cutoff.Add(time.Hour).Format(time.RFC3339Nano)
	at := cutoff.Format(time.RFC3339Nano)
	released := func(releasedAt string) *issueopscontract.Execution {
		return &issueopscontract.Execution{Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusReleased, ClaimedAt: older, ReleasedAt: releasedAt,
		}}
	}
	done := func(updatedAt string, execution *issueopscontract.Execution) issueopscontract.Record {
		return issueopscontract.Record{Phase: issueopscontract.PhaseDone, UpdatedAt: updatedAt, Execution: execution}
	}
	completed := done(old, released(old))
	completed.Execution.Completion = &issueopscontract.ExecutionCompletion{CompletedAt: recent}
	verified := done(old, nil)
	verified.RemoteArtifact = &issueopscontract.RemoteArtifactVerification{VerifiedAt: recent}
	verified.RemoteCompletion = &issueopscontract.RemoteCompletion{ReflectedAt: old}
	cleanup := done(old, released(old))
	cleanup.CleanupAttempt = &issueopscontract.CleanupAttempt{Operation: "finish", Token: "pending", StartedAt: old}
	active := done(old, released(old))
	active.Execution.Lease.Status = issueopscontract.LeaseStatusActive
	unreflected := done(old, nil)
	unreflected.RemoteArtifact = &issueopscontract.RemoteArtifactVerification{VerifiedAt: old}
	intent := done(old, nil)
	intent.IssueCreateIntent = &issueopscontract.IssueCreateIntent{Status: issueopscontract.IssueCreateIntentInvokedUnknown, UpdatedAt: old}
	plan := done(old, nil)
	plan.Phase = issueopscontract.PhasePlan

	return []struct {
		name   string
		record issueopscontract.Record
		want   bool
	}{
		{"(a) lease released after cutoff keeps an old record", done(old, released(recent)), false},
		{"(a) completion after cutoff keeps an old record", completed, false},
		{"(a) remote artifact verified after cutoff keeps an old record", verified, false},
		{"(b) every timestamp before cutoff is prunable", done(old, released(old)), true},
		{"(c) unparseable UpdatedAt with old timestamps is kept", done("not-a-time", released(old)), false},
		{"(c) zero UpdatedAt with old timestamps is kept", done("0001-01-01T00:00:00Z", released(old)), false},
		{"(c) empty UpdatedAt with old timestamps is kept", done("", released(old)), false},
		{"(e) unparseable lease release is ignored", done(old, released("yesterday")), true},
		{"(f) last record instant equal to cutoff is kept", done(old, released(at)), false},
		{"(g) +09:00 UpdatedAt before cutoff as an instant is prunable", done(
			cutoff.Add(-30*time.Minute).In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339Nano),
			released(cutoff.Add(-15*time.Minute).Format(time.RFC3339Nano)),
		), true},
		{"(g) +09:00 UpdatedAt with a UTC release after cutoff is kept", done(
			cutoff.Add(-30*time.Minute).In(time.FixedZone("KST", 9*60*60)).Format(time.RFC3339Nano),
			released(cutoff.Add(30*time.Minute).Format(time.RFC3339Nano)),
		), false},
		{"(d) cleanup attempt is kept", cleanup, false},
		{"(d) phase other than done is kept", plan, false},
		{"(d) active lease is kept", active, false},
		{"(d) unreflected remote artifact is kept", unreflected, false},
		{"(d) unresolved issue-create intent is kept", intent, false},
		{"(d) UpdatedAt after cutoff is kept", done(recent, nil), false},
	}
}

func TestIsPrunableDecidesAgeFromTheLastRecordInstant(t *testing.T) {
	cutoff := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range pruneAgeCases(cutoff) {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPrunable(tc.record, cutoff); got != tc.want {
				t.Fatalf("IsPrunable() = %v, want %v (UpdatedAt %q)", got, tc.want, tc.record.UpdatedAt)
			}
		})
	}
}

// baseIsPrunable is IsPrunable as it was at 25906292a, when age came from
// UpdatedAt alone. The age rule may keep more records than it did, never fewer.
func baseIsPrunable(record issueopscontract.Record, cutoff time.Time) bool {
	if record.CleanupAttempt != nil {
		return false
	}
	if record.Phase != issueopscontract.PhaseDone {
		return false
	}
	if record.Execution != nil && record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased {
		return false
	}
	if record.IssueCreateIntent != nil &&
		record.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		return false
	}
	if record.RemoteArtifact != nil &&
		(record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt == "") {
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	return err == nil && !updatedAt.IsZero() && updatedAt.Before(cutoff)
}

func TestIsPrunableNeverPrunesARecordTheUpdatedAtRuleKept(t *testing.T) {
	cutoff := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range pruneAgeCases(cutoff) {
		if IsPrunable(tc.record, cutoff) && !baseIsPrunable(tc.record, cutoff) {
			t.Errorf("%s: IsPrunable() = true where the UpdatedAt rule kept the record", tc.name)
		}
	}
}
