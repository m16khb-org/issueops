package issueopscleanup_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type childMergeRecords struct {
	*lockedChildCleanupRecords
	beforeLock func()
	readErr    error
}

func (r childMergeRecords) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	if r.beforeLock != nil {
		r.beforeLock()
	}
	return r.lockedChildCleanupRecords.WithinLock(ctx, id, fn)
}
func (r childMergeRecords) Load(id string) (model.IssueOpsRecord, error) {
	if r.readErr != nil {
		return model.IssueOpsRecord{}, r.readErr
	}
	return r.lockedChildCleanupRecords.Load(id)
}

func TestChildrenCloserBindsUnlockedMergeObservationToCurrentRecord(t *testing.T) {
	for _, tc := range []struct {
		name       string
		noArtifact bool
		change     func(*model.IssueOpsRecord)
	}{
		{name: "unchanged"},
		{name: "artifact mutated in place", change: func(r *model.IssueOpsRecord) { r.RemoteArtifact.URL = "replacement" }},
		{name: "child link mutated in place", change: func(r *model.IssueOpsRecord) { r.IssueLinks[0].URL = "https://github.com/acme/repo/issues/4" }},
		{name: "artifact appeared", noArtifact: true, change: func(r *model.IssueOpsRecord) {
			r.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{URL: "new"}
		}},
		{name: "branch changed", change: func(r *model.IssueOpsRecord) { r.Branch = "other" }},
		{name: "artifact changed", change: func(r *model.IssueOpsRecord) {
			r.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{URL: "replacement"}
		}},
		{name: "artifact removed", change: func(r *model.IssueOpsRecord) { r.RemoteArtifact = nil }},
		{name: "parent changed", change: func(r *model.IssueOpsRecord) { r.IssueURL = "https://github.com/acme/repo/issues/3" }},
		{name: "repo changed", change: func(r *model.IssueOpsRecord) { r.Repo = "/other" }},
		{name: "children changed", change: func(r *model.IssueOpsRecord) { r.IssueLinks = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "parent", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "observed"}, IssueLinks: []model.IssueOpsIssueLink{{Type: "child", Provider: "github", URL: "https://github.com/acme/repo/issues/2"}}}
			if tc.noArtifact {
				record.RemoteArtifact = nil
			}
			records := &lockedChildCleanupRecords{t: t, record: record}
			current := childMergeRecords{lockedChildCleanupRecords: records, beforeLock: func() {
				if tc.change != nil {
					tc.change(&records.record)
				}
			}}
			var calls []string
			closer := app.ChildrenCloser{
				Records: current, Now: func() time.Time { return time.Unix(123, 0) },
				VerifyMerged: func(artifact model.IssueOpsRemoteArtifactVerification) error {
					if records.locked || artifact.URL != "observed" {
						t.Fatalf("merge observation held the cycle lock or used an unobserved artifact: %+v", artifact)
					}
					calls = append(calls, "merge")
					return nil
				},
				Provider: func(string) (port.IssueProvider, error) {
					calls = append(calls, "provider")
					return childCloseFunction{close: func(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
						if !req.Confirm {
							t.Error("expected confirmed close")
						}
						return port.IssueProviderCloseChildResult{Closed: true, HierarchyVerified: true}, nil
					}}, nil
				},
			}
			result, err := closer.Close(context.Background(), record.ID, true, true)
			if tc.change != nil {
				expectedCalls := []string{"merge"}
				if tc.noArtifact {
					expectedCalls = nil
				}
				if err == nil || records.saves != 0 || !reflect.DeepEqual(calls, expectedCalls) {
					t.Fatalf("stale evidence accepted: result=%+v err=%v calls=%v saves=%d", result, err, calls, records.saves)
				}
				return
			}
			if err != nil || !result.Merged || result.EvidenceBasis != "parent_merge_verified" || result.ClosedCount != 1 || records.saves != 1 {
				t.Fatalf("result=%+v err=%v saves=%d", result, err, records.saves)
			}
			if !reflect.DeepEqual(calls, []string{"merge", "provider"}) {
				t.Fatalf("calls=%v", calls)
			}
		})
	}
}

func TestChildrenCloserMergeReadbackRefusals(t *testing.T) {
	for _, tc := range []struct {
		name              string
		requested         bool
		readErr, mergeErr error
		wantCalls         int
	}{
		{name: "not requested"},
		{name: "missing record", requested: true, readErr: errors.New("missing")},
		{name: "unmerged artifact", requested: true, mergeErr: errors.New("not merged"), wantCalls: 1},
		{name: "provider unavailable", requested: true, mergeErr: errors.New("readback unavailable"), wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "parent", IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "artifact"}, IssueLinks: []model.IssueOpsIssueLink{{Type: "child", Provider: "github", URL: "child"}}}
			records := &lockedChildCleanupRecords{t: t, record: record}
			calls := 0
			closer := app.ChildrenCloser{Records: childMergeRecords{lockedChildCleanupRecords: records, readErr: tc.readErr}, VerifyMerged: func(model.IssueOpsRemoteArtifactVerification) error { calls++; return tc.mergeErr }, Provider: func(string) (port.IssueProvider, error) {
				t.Error("child provider must not be resolved on refused merge evidence")
				return nil, errors.New("unexpected child provider")
			}}
			result, err := closer.Close(context.Background(), record.ID, tc.requested, true)
			if err == nil || calls != tc.wantCalls || records.saves != 0 {
				t.Fatalf("result=%+v err=%v calls=%d saves=%d", result, err, calls, records.saves)
			}
			if tc.readErr != nil {
				if !errors.Is(err, tc.readErr) {
					t.Fatalf("read error lost: %v", err)
				}
			} else if !reflect.DeepEqual(result.Missing, []string{"merge_evidence"}) {
				t.Fatalf("missing=%v", result.Missing)
			}
		})
	}
}
