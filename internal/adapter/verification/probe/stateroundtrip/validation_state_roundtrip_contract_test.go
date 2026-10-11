package stateroundtrip

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
	statecontract "issueops/internal/contract/state"
)

const contractSeed = 321

// depsWithOverride answers every CLI step with its valid payload except one
// label, whose step is replaced by override.
func depsWithOverride(t *testing.T, label string, override func(command []string) selfverify.StepResult) stateRoundtripValidationDeps {
	t.Helper()
	deps := stateRoundtripTestDeps(t, contractSeed)
	deps.run = func(_ string, got string, _ time.Duration, _ string, _ []string, command ...string) selfverify.StepResult {
		if got == label {
			return override(command)
		}
		return stateRoundtripStep(t, got, command, validStateRoundtripPayload(t, got, contractSeed))
	}
	return deps
}

var roundtripLabels = []string{
	"state write", "state read", "state list", "state old write", "state prune dry-run",
	"state prune confirm", "state list after prune", "self verify compare ok",
	"self verify compare regression", "self verify promote dry-run", "self verify promote confirm",
	"self verify compare promoted", "self verify history", "self verify history retention dry-run",
	"self verify history retention confirm", "self verify history after retention", "state doctor",
}

func TestStateRoundtripEveryCLIStepFailureIsAttributedToThatStep(t *testing.T) {
	for _, label := range roundtripLabels {
		t.Run(label, func(t *testing.T) {
			deps := depsWithOverride(t, label, func(command []string) selfverify.StepResult {
				return selfverify.StepResult{Label: label, Command: strings.Join(command, " "), OK: false, Error: "exit 7"}
			})
			step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
			if step.OK || step.Label != "state roundtrip" || step.Error != label+": exit 7" {
				t.Fatalf("failure not attributed to %q: %+v", label, step)
			}
			if !strings.Contains(step.Command, "bin ") {
				t.Fatalf("failed step must keep the executed command history: %q", step.Command)
			}
		})
	}
}

func TestStateRoundtripEveryCLIStepRejectsNonJSONOutput(t *testing.T) {
	for _, label := range roundtripLabels {
		t.Run(label, func(t *testing.T) {
			deps := depsWithOverride(t, label, func(command []string) selfverify.StepResult {
				return selfverify.StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: "not json"}
			})
			step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
			if step.OK || !strings.Contains(step.Error, "invalid character") {
				t.Fatalf("non-JSON stdout of %q must fail the roundtrip: %+v", label, step)
			}
		})
	}
}

func TestStateRoundtripRejectsContractViolatingResponses(t *testing.T) {
	key := "self-verify-321"
	record := func(k, content string) statecontract.RecordEnvelope {
		return statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: k, Content: content, Bytes: len(content)}
	}
	goodContent := "seed=321\nLore: state roundtrip\n"
	for _, tc := range []struct {
		name    string
		label   string
		payload any
		want    string
	}{
		{"write wrong key", "state write", statecontract.StateResult{OK: true, Record: record("other", goodContent)}, "write result did not match expected record"},
		{"write wrong content", "state write", statecontract.StateResult{OK: true, Record: record(key, "tampered")}, "write result did not match expected record"},
		{"write wrong byte count", "state write", statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: key, Content: goodContent, Bytes: 1}}, "write result did not match expected record"},
		{"write not ok", "state write", statecontract.StateResult{OK: false, Record: record(key, goodContent)}, "write result did not match expected record"},
		{"read wrong content", "state read", statecontract.StateResult{OK: true, Record: record(key, "tampered")}, "read result did not match expected record"},
		{"list missing key", "state list", statecontract.StateListResult{OK: true, Keys: []string{"other"}}, "state list did not include roundtrip key"},
		{"list not ok", "state list", statecontract.StateListResult{OK: false, Keys: []string{key}}, "state list did not include roundtrip key"},
		{"prune dry-run not dry", "state prune dry-run", statecontract.StatePruneResult{OK: true, DryRun: false, DeletedKeys: []string{key + "-old"}, KeptKeys: []string{key}}, "state prune dry-run did not classify old/fresh keys"},
		{"prune dry-run keeps old", "state prune dry-run", statecontract.StatePruneResult{OK: true, DryRun: true, KeptKeys: []string{key, key + "-old"}}, "state prune dry-run did not classify old/fresh keys"},
		{"prune confirm still dry", "state prune confirm", statecontract.StatePruneResult{OK: true, DryRun: true, Confirm: true, DeletedKeys: []string{key + "-old"}}, "state prune confirm did not delete old key"},
		{"prune confirm without confirm flag", "state prune confirm", statecontract.StatePruneResult{OK: true, DeletedKeys: []string{key + "-old"}}, "state prune confirm did not delete old key"},
		{"list after prune lost fresh key", "state list after prune", statecontract.StateListResult{OK: true, Keys: []string{}}, "state prune did not preserve fresh key and remove old key"},
		{"compare ok regressed", "self verify compare ok", selfaugment.SelfAugmentCompareResult{OK: true, Regressed: true, ElapsedDeltaMS: 100}, "self-verify compare reported unexpected non-regression result"},
		{"compare ok wrong elapsed delta", "self verify compare ok", selfaugment.SelfAugmentCompareResult{OK: true, ElapsedDeltaMS: 99}, "self-verify compare reported unexpected non-regression result"},
		{"compare ok not ok", "self verify compare ok", selfaugment.SelfAugmentCompareResult{OK: false, ElapsedDeltaMS: 100}, "self-verify compare reported unexpected non-regression result"},
		{"compare threshold not regressed", "self verify compare regression", selfaugment.SelfAugmentCompareResult{OK: true}, "self-verify compare did not report expected elapsed regression"},
		{"compare threshold without reasons", "self verify compare regression", selfaugment.SelfAugmentCompareResult{OK: true, Regressed: true}, "self-verify compare did not report expected elapsed regression"},
		{"promote dry-run promoted", "self verify promote dry-run", selfaugment.SelfAugmentPromoteResult{OK: true, DryRun: true, Promoted: true}, "self-verify promote dry-run mutated state or did not report dry-run"},
		{"promote dry-run not flagged", "self verify promote dry-run", selfaugment.SelfAugmentPromoteResult{OK: true}, "self-verify promote dry-run mutated state or did not report dry-run"},
		{"promote confirm not promoted", "self verify promote confirm", selfaugment.SelfAugmentPromoteResult{OK: true}, "self-verify promote confirm did not write baseline"},
		{"promote confirm still dry-run", "self verify promote confirm", selfaugment.SelfAugmentPromoteResult{OK: true, DryRun: true, Promoted: true}, "self-verify promote confirm did not write baseline"},
		{"promoted compare drift", "self verify compare promoted", selfaugment.SelfAugmentCompareResult{OK: true, ElapsedDeltaMS: 5}, "promoted baseline did not compare cleanly with candidate"},
		{"promoted compare regressed", "self verify compare promoted", selfaugment.SelfAugmentCompareResult{OK: true, Regressed: true}, "promoted baseline did not compare cleanly with candidate"},
		{"history too few matches", "self verify history", selfaugment.SelfAugmentHistoryResult{OK: true, TotalMatches: 2, Entries: []selfaugment.SelfAugmentHistoryEntry{{Key: key + "-compare-base"}, {Key: key + "-compare-candidate"}, {Key: key + "-promoted-baseline"}}}, "self-verify history did not list saved baseline/candidate/promoted summaries"},
		{"history missing promoted entry", "self verify history", selfaugment.SelfAugmentHistoryResult{OK: true, TotalMatches: 3, Entries: []selfaugment.SelfAugmentHistoryEntry{{Key: key + "-compare-base"}, {Key: key + "-compare-candidate"}}}, "self-verify history did not list saved baseline/candidate/promoted summaries"},
		{"retention dry-run without retention", "self verify history retention dry-run", selfaugment.SelfAugmentHistoryResult{OK: true}, "retention dry-run did not classify prune candidates safely"},
		{"retention dry-run already deleted", "self verify history retention dry-run", selfaugment.SelfAugmentHistoryResult{OK: true, Retention: &selfaugment.SelfAugmentHistoryRetention{DryRun: true, Limit: 1, CandidateKeys: []string{"a"}, DeletedKeys: []string{"a"}}}, "retention dry-run did not classify prune candidates safely"},
		{"retention dry-run wrong limit", "self verify history retention dry-run", selfaugment.SelfAugmentHistoryResult{OK: true, Retention: &selfaugment.SelfAugmentHistoryRetention{DryRun: true, Limit: 2, CandidateKeys: []string{"a"}}}, "retention dry-run did not classify prune candidates safely"},
		{"retention dry-run no candidates", "self verify history retention dry-run", selfaugment.SelfAugmentHistoryResult{OK: true, Retention: &selfaugment.SelfAugmentHistoryRetention{DryRun: true, Limit: 1}}, "retention dry-run did not classify prune candidates safely"},
		{"retention confirm without retention", "self verify history retention confirm", selfaugment.SelfAugmentHistoryResult{OK: true}, "retention confirm did not delete prune candidates"},
		{"retention confirm deleted nothing", "self verify history retention confirm", selfaugment.SelfAugmentHistoryResult{OK: true, Retention: &selfaugment.SelfAugmentHistoryRetention{Confirm: true}}, "retention confirm did not delete prune candidates"},
		{"retention confirm still dry-run", "self verify history retention confirm", selfaugment.SelfAugmentHistoryResult{OK: true, Retention: &selfaugment.SelfAugmentHistoryRetention{DryRun: true, Confirm: true, DeletedKeys: []string{"a"}}}, "retention confirm did not delete prune candidates"},
		{"history after retention too many", "self verify history after retention", selfaugment.SelfAugmentHistoryResult{OK: true, TotalMatches: 2}, "retention confirm left too many matching summaries"},
		{"doctor healthy despite corruption", "state doctor", statecontract.StateDoctorResult{OK: true, Healthy: true, ValidKeys: []string{key}, Issues: []statecontract.StateDoctorIssue{{Code: "invalid_state"}}}, "state doctor did not report corrupt fixture and preserve valid key"},
		{"doctor lost valid key", "state doctor", statecontract.StateDoctorResult{OK: true, ValidKeys: []string{"other"}, Issues: []statecontract.StateDoctorIssue{{Code: "invalid_state"}}}, "state doctor did not report corrupt fixture and preserve valid key"},
		{"doctor wrong issue code", "state doctor", statecontract.StateDoctorResult{OK: true, ValidKeys: []string{key}, Issues: []statecontract.StateDoctorIssue{{Code: "other"}}}, "state doctor did not report corrupt fixture and preserve valid key"},
		{"doctor no issues", "state doctor", statecontract.StateDoctorResult{OK: true, ValidKeys: []string{key}}, "state doctor did not report corrupt fixture and preserve valid key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := depsWithOverride(t, tc.label, func(command []string) selfverify.StepResult {
				return stateRoundtripStep(t, tc.label, command, tc.payload)
			})
			step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
			if step.OK || step.Label != "state roundtrip" || !strings.Contains(step.Error, tc.want) {
				t.Fatalf("want rejection containing %q, got %+v", tc.want, step)
			}
		})
	}
}

func TestStateRoundtripStopsAtFirstFailingStep(t *testing.T) {
	var labels []string
	deps := stateRoundtripTestDeps(t, contractSeed)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) selfverify.StepResult {
		labels = append(labels, label)
		if label == "self verify promote dry-run" {
			return selfverify.StepResult{Label: label, OK: false, Error: "stop"}
		}
		return stateRoundtripStep(t, label, command, validStateRoundtripPayload(t, label, contractSeed))
	}
	step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
	if step.OK {
		t.Fatalf("expected failure: %+v", step)
	}
	if last := labels[len(labels)-1]; last != "self verify promote dry-run" {
		t.Fatalf("steps after the failure still ran: %v", labels)
	}
}

func TestStateRoundtripSeedsCompareSnapshotsWithKnownElapsedTimes(t *testing.T) {
	type written struct {
		key       string
		elapsedMS int64
		seed      int64
	}
	var got []written
	deps := stateRoundtripTestDeps(t, contractSeed)
	deps.writeSnapshot = func(_ string, key string, snap selfaugment.SelfAugmentStateSnapshot) error {
		got = append(got, written{key, snap.ElapsedMS, snap.BaseSeed})
		return nil
	}
	step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
	if !step.OK {
		t.Fatalf("roundtrip failed: %+v", step)
	}
	want := []written{
		{"self-verify-321-compare-base", 1000, contractSeed},
		{"self-verify-321-compare-candidate", 1100, contractSeed},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("snapshots = %+v, want %+v", got, want)
	}
}

func TestStateRoundtripSnapshotWriteFailureAbortsBeforeAnyCompare(t *testing.T) {
	var labels []string
	deps := stateRoundtripTestDeps(t, contractSeed)
	deps.writeSnapshot = func(_ string, key string, _ selfaugment.SelfAugmentStateSnapshot) error {
		if strings.HasSuffix(key, "-compare-candidate") {
			return errors.New("snapshot disk full")
		}
		return nil
	}
	inner := deps.run
	deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) selfverify.StepResult {
		labels = append(labels, label)
		return inner(root, label, timeout, input, env, command...)
	}
	step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
	if step.OK || !strings.Contains(step.Error, "snapshot disk full") {
		t.Fatalf("expected snapshot failure, got %+v", step)
	}
	if slices.Contains(labels, "self verify compare ok") {
		t.Fatalf("compare ran despite failed snapshot seeding: %v", labels)
	}
}

type failingDatabase struct{ err error }

func (d failingDatabase) Put(string, string, []byte) error { return d.err }

func TestStateRoundtripDoctorCorruptionSeedingFailures(t *testing.T) {
	t.Run("open database fails", func(t *testing.T) {
		deps := stateRoundtripTestDeps(t, contractSeed)
		deps.openDatabase = func(string) (StateDatabase, error) { return nil, errors.New("cannot open store") }
		step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
		if step.OK || !strings.Contains(step.Error, "cannot open store") {
			t.Fatalf("got %+v", step)
		}
	})
	t.Run("corrupt put fails", func(t *testing.T) {
		deps := stateRoundtripTestDeps(t, contractSeed)
		deps.openDatabase = func(string) (StateDatabase, error) { return failingDatabase{errors.New("put refused")}, nil }
		step := validateStateRoundtripWithTestDeps("bin", t.TempDir(), contractSeed, deps)
		if step.OK || !strings.Contains(step.Error, "put refused") {
			t.Fatalf("got %+v", step)
		}
	})
}

func TestStateRoundtripDefaultRunnerRejectsEmptyCommand(t *testing.T) {
	deps := (stateRoundtripValidationDeps{}).withDefaults()
	step := deps.run(t.TempDir(), "label", time.Second, "", nil)
	if step.OK || step.Label != "label" || !strings.Contains(step.Error, "missing command") {
		t.Fatalf("got %+v", step)
	}
}
