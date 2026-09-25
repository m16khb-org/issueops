package guard

import (
	"testing"

	guardcontract "issueops/internal/contract/guard"
)

func TestAnalyzeFindingsFromObservedSourceAndPaths(t *testing.T) {
	testSource := "package sample\nfunc TestWorks(t *testing.T) { time.Sleep(time.Second) }\n"
	got := Analyze([]FileObservation{
		{Path: "pkg/foo_test.go", Content: testSource, Read: true},
		{Path: ".env", Read: false},
	}, nil)
	if got.OK || got.Summary.Block != 2 || !hasRule(got.Findings, "sleep-in-test") || !hasRule(got.Findings, "secret-like-path") {
		t.Fatalf("unexpected analysis: %+v", got)
	}
}

func TestAnalyzeWarnsOnProductionWithoutChangedTest(t *testing.T) {
	got := Analyze([]FileObservation{{Path: "cmd/app/main.go", Content: "package main", Read: true}}, nil)
	if !got.OK || got.Summary.Warn != 1 || !hasRule(got.Findings, "prod-change-without-test") {
		t.Fatalf("unexpected analysis: %+v", got)
	}
}

func TestRelevantPathAndSourceClassification(t *testing.T) {
	if !RelevantPath(".env") || !RelevantPath("pkg/main.go") || RelevantPath("image.png") {
		t.Fatal("relevant path classification changed")
	}
	if !SourcePath("pkg/main.go") || SourcePath("pkg/main_test.go") || SourcePath("docs/guide.md") {
		t.Fatal("source path classification changed")
	}
}

func TestModePreservesStagedPriority(t *testing.T) {
	if got := Mode(guardcontract.GuardCheckRequest{Files: []string{"a.go"}, Staged: true}); got != "staged" {
		t.Fatalf("mode = %q", got)
	}
	if got := Mode(guardcontract.GuardCheckRequest{All: true, Files: []string{"a.go"}}); got != "all" {
		t.Fatalf("all mode = %q", got)
	}
}

func hasRule(findings []guardcontract.GuardFinding, rule string) bool {
	for _, finding := range findings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}
