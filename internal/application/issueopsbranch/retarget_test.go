package issueopsbranch

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func retargetTestRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID:       "io-1",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "2819-child",
		IssueURL: "https://gitlab.example/group/project/-/issues/2819",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "gitlab", Branch: "2819-child", BaseBranch: "release/stg", BaseSHA: "abc123",
		},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			Provider: "gitlab", Kind: "mr", URL: "https://gitlab.example/group/project/-/merge_requests/5606",
			TargetBranch: "release/stg",
		},
	}
}

type retargetRepository struct {
	record     model.IssueOpsRecord
	locked     bool
	saves      int
	beforeLock func()
}

func (s *retargetRepository) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	if s.beforeLock != nil {
		s.beforeLock()
	}
	s.locked = true
	defer func() { s.locked = false }()
	return fn(ctx)
}
func (s *retargetRepository) Load(string) (model.IssueOpsRecord, error) { return s.record, nil }
func (s *retargetRepository) Save(_ context.Context, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !s.locked {
		panic("save outside lock")
	}
	s.record = record
	s.saves++
	return record, nil
}
func retargetTestStore(record model.IssueOpsRecord, observedTarget string, remotePresent bool) (*retargetRepository, Retargeter) {
	s := &retargetRepository{record: record}
	service := Retargeter{Records: s, Now: func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }, TargetBranch: func(artifact model.IssueOpsRemoteArtifactVerification) (string, error) {
		if s.locked {
			panic("network under lock")
		}
		if artifact.URL != record.RemoteArtifact.URL {
			return "", fmt.Errorf("unexpected artifact %q", artifact.URL)
		}
		return observedTarget, nil
	}, OriginPresent: func(repo, branch string) (bool, error) {
		if s.locked {
			panic("network under lock")
		}
		return remotePresent, nil
	}}
	return s, service
}
func runRetarget(service Retargeter, id string, req model.IssueOpsBranchRetargetRequest) (model.IssueOpsRecord, error) {
	return service.Retarget(context.Background(), id, req, model.IssueOpsActor{})
}

func TestRetargetRecordsProviderObservedBaseChange(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)

	record, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "child MR retargeted to the umbrella branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	prepare := record.BranchPrepare
	if prepare.BaseBranch != "2803-umbrella" {
		t.Fatalf("base_branch must follow the observed retarget: %+v", prepare)
	}
	if prepare.BaseSHA != "abc123" {
		t.Fatalf("sealed base sha is the fork point and must survive a retarget: %+v", prepare)
	}
	if record.RemoteArtifact.TargetBranch != "2803-umbrella" {
		t.Fatalf("remote artifact target must stay in step with the prepared base: %+v", record.RemoteArtifact)
	}
	if len(prepare.Retargets) != 1 {
		t.Fatalf("expected one retarget entry: %+v", prepare.Retargets)
	}
	entry := prepare.Retargets[0]
	if entry.FromBase != "release/stg" || entry.ToBase != "2803-umbrella" || entry.Reason == "" ||
		entry.ArtifactURL != record.RemoteArtifact.URL || entry.ObservedAt == "" {
		t.Fatalf("retarget entry must carry from/to/reason/artifact/observed_at: %+v", entry)
	}
}

func TestRetargetRejectsBaseTheProviderDoesNotShow(t *testing.T) {
	s, store := retargetTestStore(retargetTestRecord(), "release/stg", true)

	_, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "wishful",
	})
	if err == nil || !strings.Contains(err.Error(), "release/stg") {
		t.Fatalf("a base the provider does not show must be rejected with the observed target: %v", err)
	}
	if s.record.BranchPrepare.BaseBranch != "release/stg" || len(s.record.BranchPrepare.Retargets) != 0 {
		t.Fatalf("rejected retarget must not touch the record: %+v", s.record.BranchPrepare)
	}
}

func TestRetargetRejectsBaseAbsentFromRemote(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", false)

	_, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "gone",
	})
	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("a base missing from origin must be rejected: %v", err)
	}
}

func TestRetargetFailsClosedWhenObservationFails(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	store.TargetBranch = func(model.IssueOpsRemoteArtifactVerification) (string, error) {
		return "", fmt.Errorf("network down")
	}

	_, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("observation failure must reject, not pass: %v", err)
	}
}

func TestRetargetRequiresRemoteArtifactAndReason(t *testing.T) {
	record := retargetTestRecord()
	record.RemoteArtifact = nil
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	store.Records.(*retargetRepository).record = record
	if _, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "x"}); err == nil || !strings.Contains(err.Error(), "remote artifact") {
		t.Fatalf("retarget without a verified remote artifact must be rejected: %v", err)
	}

	_, store = retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	if _, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella"}); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("retarget without a reason must be rejected: %v", err)
	}
	if _, err := runRetarget(store, "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "release/stg", Reason: "same"}); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("retarget to the current base must be rejected: %v", err)
	}
}

func TestRetargetRejectsDriftBeforeWrite(t *testing.T) {
	for _, kind := range []string{"artifact", "repo"} {
		t.Run(kind, func(t *testing.T) {
			s, service := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
			s.beforeLock = func() {
				if kind == "artifact" {
					copy := *s.record.RemoteArtifact
					copy.URL += "-changed"
					s.record.RemoteArtifact = &copy
				} else {
					s.record.Repo += "-changed"
				}
			}
			_, err := runRetarget(service, "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "observed"})
			if err == nil || !strings.Contains(err.Error(), "changed while") || s.saves != 0 {
				t.Fatalf("err=%v saves=%d", err, s.saves)
			}
		})
	}
}
func TestRetargetDoesNotMutateObservedSnapshot(t *testing.T) {
	original := retargetTestRecord()
	_, service := retargetTestStore(original, "2803-umbrella", true)
	_, err := runRetarget(service, "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "observed"})
	if err != nil {
		t.Fatal(err)
	}
	if original.BranchPrepare.BaseBranch != "release/stg" || original.RemoteArtifact.TargetBranch != "release/stg" || len(original.BranchPrepare.Retargets) != 0 {
		t.Fatalf("input snapshot mutated: %+v", original.BranchPrepare)
	}
}

func TestRetargetRechecksHolderAfterObservation(t *testing.T) {
	holder := model.NativeActor{Host: "codex", SessionID: "owner", SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: "2026-08-28T00:00:00Z", Executable: "/usr/bin/codex"}}
	active, root := retargetActorRecord(t, model.LeaseStatusActive, &holder)
	record := retargetTestRecord()
	record.Execution = active.Execution
	repo, service := retargetTestStore(record, "2803-umbrella", true)
	service.Authority = cycleapp.NewMutationAuthority(func(a, b string) bool { return a == b }, liveVerifier())
	repo.beforeLock = func() { next := holder; next.SessionID = "new-owner"; repo.record.Execution.Lease.Holder = &next }
	_, err := service.Retarget(context.Background(), record.ID, model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "observed"}, model.IssueOpsActor{Host: "codex", SessionID: "owner", CWD: root, NativeProcessAncestry: []model.NativeProcessReceipt{*holder.SessionProcess}})
	if err == nil || repo.saves != 0 {
		t.Fatalf("stale holder accepted: error=%v saves=%d", err, repo.saves)
	}
}
func TestRetargetRejectsNonHolderBeforeNetwork(t *testing.T) {
	holder := model.NativeActor{Host: "codex", SessionID: "owner", SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: "2026-08-28T00:00:00Z", Executable: "/usr/bin/codex"}}
	active, _ := retargetActorRecord(t, model.LeaseStatusActive, &holder)
	record := retargetTestRecord()
	record.Execution = active.Execution
	repo, service := retargetTestStore(record, "2803-umbrella", true)
	service.TargetBranch = func(model.IssueOpsRemoteArtifactVerification) (string, error) {
		t.Fatal("unauthorized caller reached network")
		return "", nil
	}
	_, err := runRetarget(service, record.ID, model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "observed"})
	if err == nil || repo.saves != 0 {
		t.Fatalf("nonholder accepted: %v", err)
	}
}
