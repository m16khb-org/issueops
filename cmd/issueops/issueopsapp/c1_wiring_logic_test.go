package issueopsapp

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"issueops/cmd/issueops/channelcli"
	"issueops/cmd/issueops/gatescli"
	"issueops/cmd/issueops/modelcli"
	issueopsadapter "issueops/internal/adapter/issueops"
	guard "issueops/internal/contract/guard"
	issueopscontract "issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	reviewcontract "issueops/internal/contract/issueopsreview"
	policycontract "issueops/internal/contract/policy"
	"issueops/internal/port"
)

func TestRootSubcommandErrorExitCodeMapsTypedErrorsPerSubcommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		err  error
		want int
	}{
		{"policy denied", "policy", policycontract.PolicyDeniedError{Reasons: []string{"x"}}, 3},
		{"guard blocked", "guard", guard.GuardBlockedError{}, 3},
		{"gates usage", "gates", gatescli.UsageError{Message: "bad"}, 2},
		{"wrapped gates usage", "gates", fmt.Errorf("wrap: %w", gatescli.UsageError{Message: "bad"}), 2},
		{"channel timeout", "channel", channelcli.TimedOutError{}, 1},
		{"model usage", "model", modelcli.UsageError{Message: "bad"}, 2},
		{"policy denial under other subcommand", "gates", policycontract.PolicyDeniedError{}, 1},
		{"model usage under gates", "gates", modelcli.UsageError{Message: "bad"}, 1},
		{"plain policy error", "policy", errors.New("boom"), 1},
		{"plain guard error", "guard", errors.New("boom"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rootSubcommandErrorExitCode(tc.cmd, tc.err); got != tc.want {
				t.Fatalf("exit code(%s, %v) = %d, want %d", tc.cmd, tc.err, got, tc.want)
			}
		})
	}
}

func TestPublicationReconcileRequestHashesBodyAndDetachesSlices(t *testing.T) {
	create := publicationcontract.ProviderCreateRequest{
		Repo: "r", ProjectKey: "g/p", Title: "t", Body: "hello",
		HeadBranch: "h", BaseBranch: "b", ExpectedHeadSHA: "sha",
		Labels: []string{"a"}, Assignees: []string{"u"}, Draft: true,
	}
	got := publicationReconcileRequest(create)
	const helloSHA256 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got.BodySHA256 != helloSHA256 {
		t.Fatalf("BodySHA256 = %s", got.BodySHA256)
	}
	if got.Repo != "r" || got.ProjectKey != "g/p" || got.HeadBranch != "h" || got.BaseBranch != "b" ||
		got.ExpectedHeadSHA != "sha" || got.Title != "t" || !got.Draft {
		t.Fatalf("request fields not carried: %+v", got)
	}
	got.Labels[0] = "mutated"
	if create.Labels[0] != "a" {
		t.Fatal("reconcile request shares label storage with the create request")
	}
	if empty := publicationReconcileRequest(publicationcontract.ProviderCreateRequest{}); empty.BodySHA256 !=
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" || empty.Labels != nil {
		t.Fatalf("empty request = %+v", empty)
	}
}

func TestPublicationCandidateCopiesFieldsAndClonesSlices(t *testing.T) {
	source := port.IssueProviderReconcilePullRequestCandidate{
		URL: "u", ProjectKey: "p", SourceProjectKey: "sp", HeadBranch: "h", BaseBranch: "b",
		HeadSHA: "s", Title: "t", BodySHA256: "d", Labels: []string{"l"}, Assignees: []string{"a"},
		Draft: true, State: "opened",
	}
	got := publicationCandidate(source)
	want := publicationcontract.Candidate{
		URL: "u", ProjectKey: "p", SourceProjectKey: "sp", HeadBranch: "h", BaseBranch: "b",
		HeadSHA: "s", Title: "t", BodySHA256: "d", Labels: []string{"l"}, Assignees: []string{"a"},
		Draft: true, State: "opened",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate = %+v, want %+v", got, want)
	}
	got.Labels[0] = "x"
	got.Assignees[0] = "x"
	if source.Labels[0] != "l" || source.Assignees[0] != "a" {
		t.Fatal("candidate aliases provider slices")
	}
}

func TestClonePublicationStringsKeepsNilDistinctFromEmpty(t *testing.T) {
	if clonePublicationStrings(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if got := clonePublicationStrings([]string{}); got == nil || len(got) != 0 {
		t.Fatalf("empty slice must stay non-nil empty, got %#v", got)
	}
}

func TestNextLocalReadinessObservationDoesNotReuseAcrossExecutionRootOrBaseSHA(t *testing.T) {
	base := issueopscontract.IssueOpsRecord{
		ID: "io", Repo: "/r", WorktreePath: "/w", Branch: "b",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "s1"},
		Execution:     &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/root"}},
	}
	otherRoot := base
	otherRoot.Execution = &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/other"}}
	otherSHA := base
	otherSHA.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "s2"}
	otherBase := base
	otherBase.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{BaseBranch: "dev", BaseSHA: "s1"}
	noExec := base
	noExec.Execution = nil

	observer := nextLocalReadinessObservation{
		observe: func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsReadiness, reviewcontract.LocalChangeObservation) {
			return issueopscontract.IssueOpsReadiness{OK: true}, reviewcontract.LocalChangeObservation{Paths: []string{"observed.go"}, Verified: true}
		},
		fallback: func(issueopscontract.IssueOpsRecord) ([]string, bool) { return []string{"fallback.go"}, true },
	}
	observer.localReadiness(base)
	if paths, _ := observer.changedPaths(base); !reflect.DeepEqual(paths, []string{"observed.go"}) {
		t.Fatalf("same record must reuse the observation, got %v", paths)
	}
	for name, record := range map[string]issueopscontract.IssueOpsRecord{
		"execution root": otherRoot, "base sha": otherSHA, "base branch": otherBase, "no execution": noExec,
	} {
		if paths, _ := observer.changedPaths(record); !reflect.DeepEqual(paths, []string{"fallback.go"}) {
			t.Fatalf("%s change must not reuse the observation, got %v", name, paths)
		}
	}
}

func TestNextLocalReadinessObservationChangedPathsWithoutFallbackIsUnverified(t *testing.T) {
	observer := nextLocalReadinessObservation{}
	paths, verified := observer.changedPaths(issueopscontract.IssueOpsRecord{ID: "io"})
	if paths != nil || verified {
		t.Fatalf("no observation and no fallback must be unverified, got %v %v", paths, verified)
	}
}

func TestObserveIssueOpsHolderLivenessClassifiesProcessReceipts(t *testing.T) {
	self, err := issueopsadapter.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if got := observeIssueOpsHolderLiveness(self); got == nil || !*got {
		t.Fatalf("matching live receipt must be live, got %v", got)
	}
	mismatch := self
	mismatch.Executable += ".other"
	if got := observeIssueOpsHolderLiveness(mismatch); got == nil || *got {
		t.Fatalf("reused pid must be reported not live, got %v", got)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	dead := issueopscontract.NativeProcessReceipt{PID: pid, StartedAt: self.StartedAt, Executable: self.Executable}
	if got := observeIssueOpsHolderLiveness(dead); got == nil || *got {
		t.Fatalf("exited process must be reported not live, got %v", got)
	}
}
