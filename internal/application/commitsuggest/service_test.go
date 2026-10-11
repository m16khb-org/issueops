package commitsuggest

import (
	"errors"
	"strings"
	"testing"

	commitsuggestcontract "issueops/internal/contract/commitsuggest"
)

type fakeEffects struct {
	diff   string
	staged bool
	called int
}

func (f *fakeEffects) NormalizeRoot(root string) (string, error) { return "/repo", nil }
func (f *fakeEffects) Diff(root string, staged bool) (string, error) {
	f.called++
	f.staged = staged
	return f.diff, nil
}

func TestSuggestCommitSkipsPromptForEmptyDiff(t *testing.T) {
	f := &fakeEffects{diff: "  \n"}
	result, err := (Service{Effects: f}).Suggest(commitsuggestcontract.CommitSuggestRequest{Staged: true})
	if err != nil || !result.OK || result.Executed || result.Prompt != "" || !f.staged || f.called != 1 {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
}

func TestSuggestCommitPropagatesDiffError(t *testing.T) {
	f := &errorEffects{}
	result, err := (Service{Effects: f}).Suggest(commitsuggestcontract.CommitSuggestRequest{})
	if err == nil || result.OK || !errors.Is(err, errDiff) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

var errDiff = errors.New("diff failed")

type errorEffects struct{}

func (*errorEffects) NormalizeRoot(string) (string, error) { return "/repo", nil }
func (*errorEffects) Diff(string, bool) (string, error)    { return "", errDiff }

func TestSuggestCommitBuildsPromptContainingDiff(t *testing.T) {
	f := &fakeEffects{diff: "diff --git a/x b/x\n+hello"}
	result, err := (Service{Effects: f}).Suggest(commitsuggestcontract.CommitSuggestRequest{})
	if err != nil || !result.OK || !result.Executed || result.RepoRoot != "/repo" || result.Staged {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, want := range []string{"+hello", "commit_message", "Lore:"} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

var errRoot = errors.New("root failed")

type badRootEffects struct{ fakeEffects }

func (*badRootEffects) NormalizeRoot(string) (string, error) { return "", errRoot }

func TestSuggestCommitPropagatesRootErrorWithoutDiff(t *testing.T) {
	f := &badRootEffects{}
	_, err := (Service{Effects: f}).Suggest(commitsuggestcontract.CommitSuggestRequest{})
	if !errors.Is(err, errRoot) || f.called != 0 {
		t.Fatalf("err=%v diffCalls=%d", err, f.called)
	}
}
