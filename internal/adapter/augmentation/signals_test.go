package augmentation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignalRulesRequireCurrentOwnersInsteadOfTheirOwnSearchLiterals(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "cmd/issueops/selfworkflow/augmentcatalog")
	if err := os.MkdirAll(old, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "self_augment_repo_signals.go"), []byte(`package augmentcatalog
 const searchTerms="risk_qa GoalScores MinimumGoalScore SlowStepRegressions candidate-refill-curriculum release-repro-pack"`), 0644); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/adapter/verification/riskqa/validate.go", "package riskqa\nfunc Validate(){}")
	write("cmd/issueops/selfworkflow/historycompare/self_augment_compare_test.go", "func TestCompareSelfAugmentSummariesDetectsSlowStepRegression(){}")
	write("evidence.md", "slow_step:*")
	repo := Repository{ListDocs: func(string) []string { return []string{filepath.Join(root, "evidence.md")} }}
	got := repo.CollectSignals(root, 0, nil, "")
	if got.HasRiskQATier || got.HasGoalScoreSummary || got.HasPerformanceBaseline || got.HasCandidateRefill {
		t.Fatalf("search literals must not count as implementation: %+v", got)
	}
	for rel, source := range map[string]string{
		"internal/domain/selfverify/contract.go":         "package selfverify\nvar goal=\"risk_qa\"",
		"internal/domain/selfaugment/history_compare.go": "package selfaugment\nfunc Compare(){result.SlowStepRegressions=CompareSlowestStepRegressions()}",
		"internal/domain/selfaugment/summary.go":         "package selfaugment\nfunc FinalizeSummary(){summary.MinimumGoalScore=summary.GoalScores[0].Score}",
		"internal/domain/selfaugment/candidates.go":      "package selfaugment\nvar candidates=[]string{\"candidate-refill-curriculum\",\"release-repro-pack\"}",
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got = repo.CollectSignals(root, 0, nil, "")
	if !got.HasRiskQATier || !got.HasPerformanceBaseline || !got.HasGoalScoreSummary || !got.HasCandidateRefill {
		t.Fatalf("current implementations not observed: %+v", got)
	}
}
