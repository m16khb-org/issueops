package issueopsreview

import (
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
)

type planSourceStub struct {
	linkedCalls, stagedCalls int
	linkedErr, stagedErr     error
	staged                   map[string]string
}

func (s *planSourceStub) LinkedDigest(model.IssueOpsRecord) (string, error) {
	s.linkedCalls++
	return "linked digest", s.linkedErr
}
func (s *planSourceStub) StagedPlans(string, string) (map[string]string, error) {
	s.stagedCalls++
	return s.staged, s.stagedErr
}

func TestReviewPlanBindingDoesNotFallBackFromBrokenLinkedPlan(t *testing.T) {
	source := &planSourceStub{staged: map[string]string{"plan": "abc"}}
	resolver := NewPlanDigestResolver(source)
	record := model.IssueOpsRecord{ID: "io-plan", PlanPath: " plan.md "}
	got, err := resolver.Digest("state", record)
	if err != nil || got != "linked digest" || source.linkedCalls != 1 || source.stagedCalls != 0 {
		t.Fatalf("digest=%s err=%v source=%+v", got, err, source)
	}
	source.linkedErr = errors.New("linked plan unreadable")
	if _, err := resolver.Digest("state", record); !errors.Is(err, source.linkedErr) || source.stagedCalls != 0 {
		t.Fatalf("linked error lost or staged fallback read: %v", err)
	}
	record.PlanPath = " "
	got, err = resolver.Digest("state", record)
	if err != nil || got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || source.stagedCalls != 1 {
		t.Fatalf("staged digest=%s err=%v", got, err)
	}
	source.stagedErr = errors.New("storage error")
	_, err = resolver.Digest("state", record)
	if err == nil || err.Error() != "link the plan (issueops link-plan) or stage it (issueops artifact stage --name plan) before recording the devil's-advocate review" {
		t.Fatalf("staged read error contract=%v", err)
	}
}
