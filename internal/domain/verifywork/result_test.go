package verifywork

import (
	projectdoc "issueops/internal/domain/projectdoc"
	"testing"
)

func TestEvaluateRequiresEveryEvidenceSource(t *testing.T) {
	for _, missing := range []string{"none", "git", "preflight", "guard", "command"} {
		t.Run(missing, func(t *testing.T) {
			facts := Observation{PreflightOK: true, GuardOK: true, GuardMode: "staged", CommandPresent: true, CommandOK: true}
			switch missing {
			case "git":
				facts.GitFailed = true
				facts.GitError = ""
			case "preflight":
				facts.PreflightOK = false
			case "guard":
				facts.GuardOK = false
			case "command":
				facts.CommandOK = false
			}
			result := Evaluate(facts)
			if result.OK != (missing == "none") {
				t.Fatalf("missing %s: %+v", missing, result)
			}
		})
	}
}

func TestSuggestionsPreserveSignalOrderAndSparseNames(t *testing.T) {
	facts := Observation{Signals: projectdoc.ProjectSignals{
		TestCommands:  []projectdoc.EvidenceCommand{{Command: "  "}, {Command: "go test ./...", Evidence: []string{"go.mod"}, Confidence: "high"}},
		BuildCommands: []projectdoc.EvidenceCommand{{Command: "go build ./..."}},
		LintCommands:  []projectdoc.EvidenceCommand{{Command: "go vet ./..."}},
	}}
	result := Evaluate(facts)
	if len(result.SuggestedCommands) != 3 {
		t.Fatalf("commands: %+v", result.SuggestedCommands)
	}
	for i, want := range []string{"test_2", "build_1", "lint_1"} {
		if result.SuggestedCommands[i].Name != want {
			t.Fatalf("commands: %+v", result.SuggestedCommands)
		}
	}
	if result.SuggestedCommands[0].Reason != "test command inferred from go.mod (confidence=high)" {
		t.Fatal(result.SuggestedCommands[0].Reason)
	}
}
