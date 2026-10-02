package issueopscompletion

import (
	"context"
	"strings"
	"testing"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

func capabilityVerifier(identity completioncontract.Actor) ActorVerifier {
	return func(context.Context, completioncontract.Actor, []completioncontract.ProcessReceipt) (completioncontract.Actor, error) {
		return identity, nil
	}
}

func TestCapabilityIdentityCannotCompleteAnotherHoldersGeneration(t *testing.T) {
	trace := []string{}
	other := completioncontract.ProcessReceipt{PID: 999, StartedAt: "2026-08-02T00:00:00Z", Executable: "/bin/codex"}
	repository := &repositoryFake{record: activeCompletionRecord("direct"), trace: &trace}
	environment := &environmentFake{trace: &trace, canonical: true, head: strings.Repeat("a", 40), report: "/worktree/report.json"}
	service := NewService(repository, environment, fixedClock{fixedCompletionTime}, capabilityVerifier(completioncontract.Actor{Host: "codex", SessionID: "other-session", Process: &other}))
	request := validRequest()
	request.Actor, request.Ancestry = completioncontract.Actor{}, nil
	if _, err := service.Complete(context.Background(), request); err == nil || !strings.Contains(err.Error(), "only the current holder") {
		t.Fatalf("foreign capability completed the generation: %v", err)
	}
	if repository.commits != 0 {
		t.Fatalf("commits=%d", repository.commits)
	}
}

func TestCapabilityHolderCompletesWithoutAncestryButNotStaleGeneration(t *testing.T) {
	holder := *activeCompletionRecord("direct").Lease.Holder
	for _, generation := range []uint64{0, 2} {
		trace := []string{}
		repository := &repositoryFake{record: activeCompletionRecord("direct"), trace: &trace}
		environment := &environmentFake{trace: &trace, canonical: true, head: strings.Repeat("a", 40), report: "/worktree/report.json"}
		service := NewService(repository, environment, fixedClock{fixedCompletionTime}, capabilityVerifier(holder))
		request := validRequest()
		request.Actor, request.Ancestry, request.Generation = completioncontract.Actor{}, nil, generation
		if _, err := service.Complete(context.Background(), request); err == nil || repository.commits != 0 {
			t.Fatalf("generation %d bypassed the completion fence: err=%v commits=%d", generation, err, repository.commits)
		}
	}
	trace := []string{}
	repository := &repositoryFake{record: activeCompletionRecord("direct"), trace: &trace}
	environment := &environmentFake{trace: &trace, canonical: true, head: strings.Repeat("a", 40), report: "/worktree/report.json"}
	service := NewService(repository, environment, fixedClock{fixedCompletionTime}, capabilityVerifier(holder))
	request := validRequest()
	request.Actor, request.Ancestry = completioncontract.Actor{}, nil
	if _, err := service.Complete(context.Background(), request); err != nil || repository.commits != 1 {
		t.Fatalf("holder capability refused: err=%v commits=%d", err, repository.commits)
	}
}
