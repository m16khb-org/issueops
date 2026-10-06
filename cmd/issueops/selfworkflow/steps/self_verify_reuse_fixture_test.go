package steps

import (
	"encoding/json"
	contract "issueops/internal/contract/selfverify"
	"os"
	"path/filepath"
	"testing"
	"time"

	application "issueops/internal/application/selfverify"
	augment "issueops/internal/contract/selfaugment"
	augmentdomain "issueops/internal/domain/selfaugment"
)

func TestSelfVerifyReuseFixtureEntryPoint(t *testing.T) {
	// Given: only external probe execution is substituted, not orchestration,
	// evidence reuse, scoring, summary aggregation, or serialization.
	root := t.TempDir()
	tempDir := filepath.Join(root, "verification")
	stepDeps := fakeSelfVerifyStepDeps(t)
	stepDeps.IssueOpsRoot = func() string { return root }
	stepDeps.ValidateRiskQATier = func(string) application.RiskQAEvidence {
		return application.RiskQAEvidence{
			Step:             contract.StepResult{Label: "risk QA tier", Command: "fixture full-suite race", OK: true, DurationMS: 137},
			CoversFullGoTest: true,
		}
	}
	command := stepDeps.RunCommandStep
	stepDeps.RunCommandStep = func(cwd, label string, timeout time.Duration, stdin, executable string, args ...string) contract.StepResult {
		if label == "go test" || label == "contract golden tests" {
			t.Fatalf("covered command ran again: %s", label)
		}
		step := command(cwd, label, timeout, stdin, executable, args...)
		step.DurationMS = 11
		return step
	}

	// When: drive the same loop and summary entry points used by self-verify.
	result, err := application.ExecuteLoop(application.LoopRequest{BaseSeed: 100, TargetScore: 95}, application.LoopDeps{
		IssueOpsRoot: func() string { return root },
		StepDeps:     stepDeps,
		MkdirTemp: func() (string, error) {
			return tempDir, os.Mkdir(tempDir, 0700)
		},
		RemoveAll:      os.RemoveAll,
		TempBinaryPath: func(dir string) string { return filepath.Join(dir, "issueops") },
		Now:            func() time.Time { return time.Unix(100, 0) },
		Summarize:      application.SummarizeSelfVerification,
	})

	// Then
	if err != nil || !result.OK || !result.TerminationEligible || result.Summary.TotalSteps != 27 ||
		result.Summary.PassedSteps != 27 || len(result.Summary.CoverageGaps) != 0 || result.Summary.Contract.Version != 8 {
		t.Fatalf("fixture execution: result=%+v err=%v", result, err)
	}
	stats := augmentdomain.StepDurationStatByLabel(result.Summary.StepDurationStats)
	for _, label := range []string{"go test", "contract golden tests"} {
		if stats[label].Count != 0 || stats[label].ReusedCount != 1 || stats[label].P95DurationMS != 0 {
			t.Fatalf("reused stat: %+v", stats[label])
		}
	}
	if stats["risk QA tier"].Count != 1 || stats["risk QA tier"].ReusedCount != 0 || stats["risk QA tier"].P95DurationMS != 137 {
		t.Fatalf("measured stat: %+v", stats["risk QA tier"])
	}
	snapshot := augmentdomain.NewSelfVerificationSummarySnapshot(result, time.Unix(100, 0))
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded augment.SelfAugmentStateSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := augmentdomain.ValidateSummarySnapshot("fixture", decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.Summary.Contract.Version != 8 {
		t.Fatalf("serialized contract changed: %+v", decoded)
	}
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("fixture workspace was not removed: %v", err)
	}
	output, err := json.Marshal(struct {
		ContractVersion int                                   `json:"contract_version"`
		SnapshotSchema  int                                   `json:"snapshot_schema"`
		PassedSteps     int                                   `json:"passed_steps"`
		Stats           []augment.SelfAugmentStepDurationStat `json:"step_duration_stats"`
	}{
		result.Summary.Contract.Version, decoded.SchemaVersion, result.Summary.PassedSteps,
		[]augment.SelfAugmentStepDurationStat{stats["risk QA tier"], stats["go test"], stats["contract golden tests"]},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fixture usage: %s", output)
}
