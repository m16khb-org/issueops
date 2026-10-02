package issueopslease

import (
	"context"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type capabilityVerifier struct{ identity model.NativeActor }

func (v capabilityVerifier) Verify(context.Context, model.NativeActor) (model.VerifiedActor, error) {
	return model.VerifiedActor{Identity: v.identity, Method: model.VerifiedByCapability}, nil
}

type capabilityRepository struct {
	record Record
	saved  bool
}

func (r *capabilityRepository) Update(_ context.Context, _ string, validate RecordValidator, transition RecordTransition) (RepositoryResult, error) {
	if err := validate(r.record); err != nil {
		return RepositoryResult{}, err
	}
	next, err := transition(r.record)
	if err != nil {
		return RepositoryResult{}, err
	}
	r.record, r.saved = next, true
	return RepositoryResult{Record: next}, nil
}

type samePathMatcher struct{}

func (samePathMatcher) Matches(left, right string) bool { return left == right }

func capabilityLeaseRecord() Record {
	holder := leasecontract.Actor{Host: "codex", SessionID: "holder", SessionProcess: &leasecontract.ProcessReceipt{PID: 11, StartedAt: "start", Executable: "/bin/codex"}}
	return Record{ID: "io-cap", CanonicalRoot: "/canonical", Lease: leasecontract.Lease{Generation: 4, Status: "active", Holder: &holder}}
}

func TestCapabilityIdentityNeverReleasesAnotherHoldersLease(t *testing.T) {
	other := model.NativeActor{Host: "codex", SessionID: "other", SessionProcess: &model.NativeProcessReceipt{PID: 22, StartedAt: "start", Executable: "/bin/codex"}}
	repository := &capabilityRepository{record: capabilityLeaseRecord()}
	service := NewReleaseService(repository, fixedClock{now: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}, capabilityVerifier{identity: other}, samePathMatcher{})
	if _, err := service.Release(context.Background(), ReleaseRequest{ID: "io-cap", Generation: 4, CWD: "/canonical"}); err == nil || !strings.Contains(err.Error(), "active generation holder") {
		t.Fatalf("foreign capability released the lease: %v", err)
	}
	if repository.saved {
		t.Fatal("refused release persisted")
	}
}

func TestCapabilityHolderStillObeysGenerationAndCanonicalCWD(t *testing.T) {
	holder := model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &model.NativeProcessReceipt{PID: 11, StartedAt: "start", Executable: "/bin/codex"}}
	for name, request := range map[string]ReleaseRequest{
		"stale generation": {ID: "io-cap", Generation: 3, CWD: "/canonical"},
		"foreign cwd":      {ID: "io-cap", Generation: 4, CWD: "/elsewhere"},
	} {
		repository := &capabilityRepository{record: capabilityLeaseRecord()}
		service := NewReleaseService(repository, fixedClock{now: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}, capabilityVerifier{identity: holder}, samePathMatcher{})
		if _, err := service.Release(context.Background(), request); err == nil || repository.saved {
			t.Fatalf("%s: capability bypassed the core lease check: err=%v saved=%v", name, err, repository.saved)
		}
	}
	repository := &capabilityRepository{record: capabilityLeaseRecord()}
	service := NewReleaseService(repository, fixedClock{now: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}, capabilityVerifier{identity: holder}, samePathMatcher{})
	if _, err := service.Release(context.Background(), ReleaseRequest{ID: "io-cap", Generation: 4, CWD: "/canonical"}); err != nil || repository.record.Lease.Holder != nil {
		t.Fatalf("holder capability without ancestry refused: err=%v lease=%+v", err, repository.record.Lease)
	}
}

func TestResolveActorUsesVerifiedIdentityNotRequestFields(t *testing.T) {
	granted := model.NativeActor{Host: "claude", SessionID: "granted", SessionProcess: &model.NativeProcessReceipt{PID: 7, StartedAt: "s", Executable: "/bin/claude"}}
	actor, err := resolveActor(context.Background(), leasedomain.Actor{}, nil, capabilityVerifier{identity: granted})
	if err != nil || actor.Host != "claude" || actor.SessionID != "granted" || actor.Process == nil || actor.Process.PID != 7 {
		t.Fatalf("actor=%+v err=%v", actor, err)
	}
}
