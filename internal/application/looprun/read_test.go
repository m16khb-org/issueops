package looprun

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	loopcontract "issueops/internal/contract/looprun"
	loopdomain "issueops/internal/domain/looprun"
)

type listStore struct {
	loops []loopdomain.LoopObservation
	err   error
	calls int
}

func (store *listStore) ReadAllExisting() ([]loopdomain.LoopObservation, error) {
	store.calls++
	return store.loops, store.err
}

type joiningIdentity struct{ fixedIdentity }

func (joiningIdentity) NormalizeRepo(repo string) (string, error) {
	return "/" + strings.TrimSpace(repo), nil
}
func (joiningIdentity) NewID(repo, name string) string { return repo + "/" + name }

type failingRepoIdentity struct{ fixedIdentity }

func (failingRepoIdentity) NormalizeRepo(string) (string, error) { return "", errors.New("cwd lost") }

func TestReaderRepoGateReportsIncompleteLoopsForNormalizedRepoOnly(t *testing.T) {
	store := &listStore{loops: []loopdomain.LoopObservation{
		{ID: "a", Loop: loopcontract.LoopRun{ID: "a", Repo: "/repo", Status: "active"}},
		{ID: "b", Loop: loopcontract.LoopRun{ID: "b", Repo: "/other", Status: "active"}},
		{ID: "c", Loop: loopcontract.LoopRun{ID: "c", Repo: "/repo", Status: "exhausted"}},
		{ID: "d", Error: errors.New("corrupt")},
	}}
	reader := Reader{Store: store, Identity: fixedIdentity{}}
	missing, warnings := reader.RepoGateMissing("relative")
	if containsString(missing, "loop_incomplete:b") || !containsString(missing, "loop_incomplete:a") || !containsString(missing, "loop_incomplete:d") {
		t.Fatalf("missing=%v", missing)
	}
	if !reflect.DeepEqual(warnings, []string{"failed to inspect loop run d: corrupt"}) {
		t.Fatalf("warnings=%v", warnings)
	}
	summary, _ := reader.RepoGateSummaryFor("relative")
	if summary.Active != 1 || summary.Exhausted != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestReaderRepoGateFailsClosedOnIdentityAndScanErrors(t *testing.T) {
	store := &listStore{err: errors.New("denied")}
	missing, warnings := Reader{Store: store, Identity: fixedIdentity{}}.RepoGateMissing("/repo")
	if !reflect.DeepEqual(missing, []string{"loops_complete"}) || !reflect.DeepEqual(warnings, []string{"failed to scan loop runs: denied"}) {
		t.Fatalf("scan: %v %v", missing, warnings)
	}
	store = &listStore{}
	missing, warnings = Reader{Store: store, Identity: failingRepoIdentity{}}.RepoGateMissing("/repo")
	if !reflect.DeepEqual(missing, []string{"loops_complete"}) || !reflect.DeepEqual(warnings, []string{"failed to resolve loop repo: cwd lost"}) || store.calls != 0 {
		t.Fatalf("resolve: %v %v calls=%d", missing, warnings, store.calls)
	}
}

func TestReaderRepoGateSkipsStoreForBlankRepo(t *testing.T) {
	store := &listStore{}
	missing, warnings := Reader{Store: store, Identity: fixedIdentity{}}.RepoGateMissing("  ")
	if missing != nil || warnings != nil || store.calls != 0 {
		t.Fatalf("blank repo: %v %v calls=%d", missing, warnings, store.calls)
	}
}

func TestServiceStatusReportsLastVerdictAndPreservesReadError(t *testing.T) {
	store := &memoryStore{exists: true, loop: loopcontract.LoopRun{ID: "loop-id", Attempts: []loopcontract.LoopAttempt{{Verdict: "fail"}, {Verdict: "pass"}}}}
	status, err := Service{Store: store, Identity: fixedIdentity{}}.Status("loop-id")
	if err != nil || !status.OK || status.AttemptCount != 2 || status.LastVerdict != "pass" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	status, err = Service{Store: &errorStore{err: errors.New("gone")}, Identity: fixedIdentity{}}.Status("x")
	if err == nil || err.Error() != "gone" || status.OK {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestServiceResolveIDValidatesNameAndRepo(t *testing.T) {
	service := Service{Identity: joiningIdentity{}}
	if id, err := service.ResolveID(" repo ", " name "); err != nil || id != "/repo/name" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if _, err := service.ResolveID("/repo", "  "); err == nil || err.Error() != "name is required" {
		t.Fatalf("blank name err=%v", err)
	}
	if _, err := (Service{Identity: failingRepoIdentity{}}).ResolveID("/repo", "n"); err == nil || err.Error() != "cwd lost" {
		t.Fatalf("repo err=%v", err)
	}
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}
