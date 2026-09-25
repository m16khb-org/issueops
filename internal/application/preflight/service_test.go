package preflight

import (
	"reflect"
	"testing"

	preflightcontract "issueops/internal/contract/preflight"
)

type observerStub struct{ observation Observation }

func (stub observerStub) Observe(string, string) Observation { return stub.observation }

func TestServiceAssemblesObservedGitFacts(t *testing.T) {
	upstream := "origin/main"
	service := Service{Observer: observerStub{Observation{
		GitOK: true, RepoRoot: "/repo", Branch: "main", Head: "abc", Upstream: &upstream,
		StatusLines: []string{" M changed.txt", "?? .env"},
		Remotes:     []preflightcontract.RemoteInfo{{Name: "origin", URL: "redacted"}},
	}}}
	got := service.Check("/repo", "/issueops")
	if !got.OK || got.IsClean || !reflect.DeepEqual(got.UnstagedFiles, []string{"changed.txt"}) ||
		!reflect.DeepEqual(got.SecretLikePaths, []string{".env"}) ||
		!reflect.DeepEqual(got.Warnings, []string{"secret_like_paths_present"}) {
		t.Fatalf("unexpected preflight result: %+v", got)
	}
}

func TestServicePreservesGitFailure(t *testing.T) {
	got := (Service{Observer: observerStub{Observation{ErrorDetail: "fatal"}}}).Check("/missing", "/issueops")
	if got.OK || got.Error != "not_git_repo" || got.Path != "/missing" || got.Detail != "fatal" || got.Upstream != nil || got.Ahead != nil {
		t.Fatalf("unexpected failure result: %+v", got)
	}
}
