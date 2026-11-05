package issueopsbenchmark

import (
	"errors"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func TestResolveJudgeScoresPreservesDecodeAndCoveragePrecedence(t *testing.T) {
	fixtures := []contract.Fixture{{ID: "first"}, {ID: "second"}}
	bad := errors.New("malformed score")
	calls := 0
	decode := func([]byte) (contract.Score, error) {
		calls++
		return contract.Score{}, bad
	}
	_, err := ResolveJudgeScores(fixtures, map[string][]byte{"first": {}, "extra": {}}, decode)
	if !errors.Is(err, bad) || calls != 1 || !strings.Contains(err.Error(), `fixture "first"`) {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	calls = 0
	_, err = ResolveJudgeScores(fixtures, map[string][]byte{"second": {}, "extra": {}}, decode)
	if err == nil || err.Error() != `judge map missing fixture "first"` || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	decode = func([]byte) (contract.Score, error) {
		return contract.Score{Passed: true}, nil
	}
	_, err = ResolveJudgeScores(fixtures, map[string][]byte{"first": {}, "second": {}, "extra": {}}, decode)
	if err == nil || err.Error() != `judge map has unknown fixture "extra"` {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateFixtureRequiresBusinessInputsInOrder(t *testing.T) {
	f := contract.Fixture{}
	for _, tc := range []struct {
		want string
		fill func()
	}{
		{"id", func() { f.ID = "fixture" }},
		{"title", func() { f.Title = "Fixture" }},
		{"user_prompt", func() { f.UserPrompt = "Improve workflow" }},
		{"repo_context", func() { f.RepoContext = "Go" }},
		{"critical_failures", func() { f.CriticalFailures = []string{"missing issue"} }},
	} {
		if err := ValidateFixture(f); err == nil || err.Error() != tc.want+" is required" {
			t.Fatalf("%s: %v", tc.want, err)
		}
		tc.fill()
	}
	if err := ValidateFixture(f); err != nil {
		t.Fatal(err)
	}
}
