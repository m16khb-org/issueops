package parallelisolation

import (
	selfverify "issueops/internal/contract/selfverify"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
)

func TestValidateParallelTempIsolationWithDepsCoversSuccessErrorsAndCollision(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationValidationDeps{
		runProbe: func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
			base := filepath.Join(root, fmt.Sprintf("worker-%d-%d", seed, worker))
			return parallelIsolationProbe{Worker: worker, TempRoot: base, StateDir: base + "/state", ArtifactPath: base + "/build/harness", Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Commands: []string{fmt.Sprintf("write-%d", worker)}}
		},
	}
	step := validateParallelTempIsolationWithDeps("issueops", root, 7, deps)
	if !step.OK || step.Label != "parallel isolation" || !strings.Contains(step.Command, "write-0") || !strings.Contains(step.Stdout, `"workers": 3`) {
		t.Fatalf("expected parallel success, got %+v", step)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, "same"), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker)}
	}
	collision := validateParallelTempIsolationWithDeps("issueops", root, 7, deps)
	if collision.OK || !strings.Contains(collision.Error, "path collision:") {
		t.Fatalf("expected path collision, got %+v", collision)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, fmt.Sprint(worker)), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Error: "boom"}
	}
	failed := validateParallelTempIsolationWithDeps("issueops", root, 7, deps)
	if failed.OK || !strings.Contains(failed.Error, "worker 0: boom") {
		t.Fatalf("expected worker error aggregation, got %+v", failed)
	}
}

func TestRunParallelIsolationProbeWithDepsCoversCommandAndContractFailures(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationProbeDeps{
		mkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern+"dir"), nil },
		removeAll: func(string) error { return nil },
		mkdirAll:  func(string, os.FileMode) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
		runCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) selfverify.StepResult {
			switch {
			case strings.Contains(label, "write"):
				return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true}
			case strings.Contains(label, "read"):
				body, _ := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
				return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			default:
				body, _ := json.Marshal(statecontract.StateListResult{OK: true, Keys: []string{"parallel-9-2"}})
				return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			}
		},
	}
	probe := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if probe.Error != "" || probe.Key != "parallel-9-2" || len(probe.Commands) != 3 {
		t.Fatalf("expected successful probe, got %+v", probe)
	}

	deps.runCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) selfverify.StepResult {
		return selfverify.StepResult{OK: false, Error: "write denied"}
	}
	writeFail := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if !strings.Contains(writeFail.Error, "state write failed: write denied") {
		t.Fatalf("expected write failure, got %+v", writeFail)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) selfverify.StepResult {
		if strings.Contains(label, "read") {
			return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
		}
		return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	badReadJSON := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if !strings.Contains(badReadJSON.Error, "state read parse failed") {
		t.Fatalf("expected read parse failure, got %+v", badReadJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) selfverify.StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "other", Content: "wrong"}})
			return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	wrongContent := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if wrongContent.Error != "state read returned another worker's content" {
		t.Fatalf("expected content failure, got %+v", wrongContent)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) selfverify.StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
	}
	badListJSON := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if !strings.Contains(badListJSON.Error, "state list parse failed") {
		t.Fatalf("expected list parse failure, got %+v", badListJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) selfverify.StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		body, _ := json.Marshal(statecontract.StateListResult{OK: true, Keys: []string{"parallel-9-2", "leak"}})
		return selfverify.StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
	}
	leaked := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if !strings.Contains(leaked.Error, "state list leaked keys across workers") {
		t.Fatalf("expected list leak failure, got %+v", leaked)
	}
}

func TestRunParallelIsolationProbeWithDepsRoutesEveryStepThroughInjectedDeps(t *testing.T) {
	root := t.TempDir()
	tempRoot := filepath.Join(root, "virtual-temp")
	var calls []string
	deps := parallelIsolationProbeDeps{
		mkdirTemp: func(dir, _ string) (string, error) {
			calls = append(calls, "mkdirTemp:"+dir)
			return tempRoot, nil
		},
		removeAll: func(path string) error {
			calls = append(calls, "removeAll:"+path)
			return nil
		},
		mkdirAll: func(path string, mode os.FileMode) error {
			calls = append(calls, fmt.Sprintf("mkdirAll:%s:%o", path, mode))
			return nil
		},
		writeFile: func(path string, data []byte, mode os.FileMode) error {
			calls = append(calls, fmt.Sprintf("writeFile:%s:%o", path, mode))
			return nil
		},
		runCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, env []string, _ string, args ...string) selfverify.StepResult {
			calls = append(calls, "run:"+label)
			if len(env) != 1 || env[0] != "ISSUEOPS_STATE_DIR="+filepath.Join(tempRoot, "state") {
				t.Fatalf("unexpected env: %v", env)
			}
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(label, "read"):
				body, _ := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
				return selfverify.StepResult{OK: true, Command: cmd, Stdout: string(body)}
			case strings.Contains(label, "list"):
				body, _ := json.Marshal(statecontract.StateListResult{OK: true, Keys: []string{"parallel-9-2"}})
				return selfverify.StepResult{OK: true, Command: cmd, Stdout: string(body)}
			}
			return selfverify.StepResult{OK: true, Command: cmd}
		},
	}
	probe := runParallelIsolationProbeWithDeps("issueops", root, 9, 2, deps)
	if probe.Error != "" || probe.TempRoot != tempRoot {
		t.Fatalf("unexpected probe: %+v", probe)
	}
	want := []string{
		"mkdirTemp:",
		fmt.Sprintf("mkdirAll:%s:700", filepath.Join(tempRoot, "build")),
		fmt.Sprintf("writeFile:%s:600", filepath.Join(tempRoot, "build", "issueops")),
		"run:parallel state write 2",
		"run:parallel state read 2",
		"run:parallel state list 2",
		"removeAll:" + tempRoot,
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("injected deps calls = %q, want %q", calls, want)
	}
}

func TestParallelIsolationResultOrdersProbesByWorker(t *testing.T) {
	probes := []parallelIsolationProbe{
		{Worker: 2, TempRoot: "t2", StateDir: "s2", ArtifactPath: "a2", Commands: []string{"cmd-2"}},
		{Worker: 0, TempRoot: "t0", StateDir: "s0", ArtifactPath: "a0", Commands: []string{"cmd-0"}},
		{Worker: 1, TempRoot: "t1", StateDir: "s1", ArtifactPath: "a1", Commands: []string{"cmd-1"}},
	}
	step := parallelIsolationResult(time.Now(), 3, probes)
	if !step.OK || step.Command != "cmd-0 && cmd-1 && cmd-2" {
		t.Fatalf("expected commands in worker order, got %+v", step)
	}
	if i0, i1, i2 := strings.Index(step.Stdout, `"worker": 0`), strings.Index(step.Stdout, `"worker": 1`), strings.Index(step.Stdout, `"worker": 2`); i0 < 0 || i0 >= i1 || i1 >= i2 {
		t.Fatalf("expected stdout probes in worker order, got %s", step.Stdout)
	}
}
