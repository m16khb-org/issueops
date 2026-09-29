package gates

import (
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	domain "issueops/internal/domain/gates"
)

type mutationStore struct {
	data     []byte
	exists   bool
	calls    []string
	writeErr error
}

func (s *mutationStore) Discover(string) ([]string, error) { return []string{"gates.md"}, nil }
func (s *mutationStore) Read(string) ([]byte, error) {
	s.calls = append(s.calls, "read")
	return s.data, nil
}
func (s *mutationStore) WritePreservingMode(_ string, data []byte) error {
	s.calls = append(s.calls, "write")
	if s.writeErr != nil {
		return s.writeErr
	}
	s.data = data
	return nil
}
func (s *mutationStore) ExistsFile(string) bool { s.calls = append(s.calls, "exists"); return s.exists }
func (s *mutationStore) Create(_ string, data []byte) error {
	s.calls = append(s.calls, "create")
	s.data = data
	return nil
}

func TestInitExistingFilePrecedesMalformedSpecWithoutWriting(t *testing.T) {
	store := &mutationStore{exists: true, data: []byte("unchanged")}
	result, err := (Service{Store: store}).Init(model.InitRequest{File: "gates.md", Scope: "scope", Gates: []string{"| INVALID: bad"}})
	if err == nil || err.Error() != "gate file already exists: gates.md" || result.OK || string(store.data) != "unchanged" || strings.Join(store.calls, ",") != "exists" {
		t.Fatalf("result=%+v calls=%v err=%v", result, store.calls, err)
	}
}
func TestAbandonRejectsUnknownAndAlreadyAbandonedBeforeWrite(t *testing.T) {
	for _, id := range []string{"G2", "G1"} {
		store := &mutationStore{data: []byte("- [ ] G1: proof\nABANDON: G1 deferred\n")}
		result, err := (Service{Store: store}).Abandon(model.AbandonRequest{File: "gates.md", GateID: id, Reason: "reason"})
		if err == nil || result.Recorded || strings.Join(store.calls, ",") != "read" {
			t.Fatalf("result=%+v calls=%v err=%v", result, store.calls, err)
		}
	}
}
func TestAbandonWriteFailureDoesNotClaimRecorded(t *testing.T) {
	want := errors.New("write failed")
	store := &mutationStore{data: []byte("- [ ] G1: proof\n"), writeErr: want}
	result, err := (Service{Store: store}).Abandon(model.AbandonRequest{File: "gates.md", GateID: "G1", Reason: "later"})
	if !errors.Is(err, want) || result.OK || result.Recorded {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestCommandRunnerRefusesDeniedPolicyBeforeExecution(t *testing.T) {
	calls := 0
	runner := CommandRunner{Evaluate: func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation {
		return policy.CommandPolicyEvaluation{Allowed: false, AuditLogID: "denied", DenyReasons: []string{"rule"}}
	}, Execute: func(policy.CommandPolicyRequest) policy.CommandRunResult { calls++; return policy.CommandRunResult{} }}
	result := runner.Run("/repo", "/repo", model.CheckRequest{TimeoutSeconds: 1}, domain.Gate{CheckCmd: "printf ok"})
	if calls != 0 || !result.PolicyDenied || result.AuditLogID != "denied" || result.CheckError != "check denied by policy: rule" {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
}
