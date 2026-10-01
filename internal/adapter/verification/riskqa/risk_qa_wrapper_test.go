package riskqa

import (
	riskqacontractx "issueops/internal/application/riskqa"
	riskqacontract "issueops/internal/contract/riskqa"
	selfverify "issueops/internal/contract/selfverify"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	riskqadomain "issueops/internal/domain/riskqa"
)

func TestValidateRiskQATierWrapperRunsElevatedDefaultCommands(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	writeFileForWrapperTest(t, filepath.Join(root, "cmd", "issueops", "risk_qa.go"), "package main\n")
	runRiskQATestCommand(t, root, "git", "add", "cmd/issueops/risk_qa.go")
	fakeBin := t.TempDir()
	writeFileForWrapperTest(t, filepath.Join(fakeBin, "go"), "#!/bin/sh\ncase \"$*\" in 'test -race ./... -count=1'|'vet ./...') printf 'fake go %s\\n' \"$*\";; *) exit 64;; esac\n")
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	step := Validate(root)
	if !step.OK || step.Label != "risk QA tier" {
		t.Fatalf("expected risk QA wrapper success, got %#v", step)
	}
	for _, want := range []string{"go test -race ./... -count=1", "go vet ./...", `"tier":"elevated"`, "fake go test -race ./... -count=1"} {
		if !strings.Contains(step.Command+"\n"+step.Stdout, want) {
			t.Fatalf("expected %q in command/stdout, got command=%q stdout=%q", want, step.Command, step.Stdout)
		}
	}
}

func TestCommittedOnlyGoScopeSelectsRaceAndVet(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "base")
	writeFileForWrapperTest(t, filepath.Join(root, "internal", "adapter", "scope.go"), "package adapter\n")
	runRiskQATestCommand(t, root, "git", "add", ".")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "Go change")
	plan := PlanWithScope(root, "HEAD~1")
	if plan.Tier != "elevated" || !sameStringSlice(plan.Commands, []string{"go test -race ./... -count=1", "go vet ./..."}) {
		t.Fatalf("committed Go change must select race and vet: %+v", plan)
	}
}

func TestPlanRiskQATierFromPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		tier     string
		commands []string
	}{
		{
			name:     "no changes",
			paths:    nil,
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "docs only",
			paths:    []string{".issueops/TESTING.md"},
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "go but not sensitive",
			paths:    []string{"examples/demo.go"},
			tier:     "static",
			commands: []string{"go vet ./..."},
		},
		{
			name:     "sensitive go",
			paths:    []string{"internal/core/policy.go", "cmd/issueops/main.go"},
			tier:     "elevated",
			commands: []string{"go test -race ./... -count=1", "go vet ./..."},
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			plan := riskqadomain.PlanFromPaths(tc.paths)
			if plan.Tier != tc.tier {
				t.Fatalf("tier=%q want %q: %+v", plan.Tier, tc.tier, plan)
			}
			if !sameStringSlice(plan.Commands, tc.commands) {
				t.Fatalf("commands=%+v want %+v", plan.Commands, tc.commands)
			}
		})
	}
}

func TestRiskQATierHelpersCoverGitWarningsAndJSON(t *testing.T) {
	nonGitRoot := t.TempDir()
	nonGitPlan := Plan(nonGitRoot)
	if nonGitPlan.Tier != "standard" || len(nonGitPlan.Commands) != 0 || nonGitPlan.Scope == nil || nonGitPlan.Scope.Error == "" {
		t.Fatalf("unexpected non-git risk plan: %+v", nonGitPlan)
	}
	if !strings.Contains(nonGitPlan.Scope.Error, "git status unavailable") {
		t.Fatalf("non-git plan missing warnings: %+v", nonGitPlan.Reasons)
	}
	step := Validate(nonGitRoot)
	if step.OK || step.Label != "risk QA tier" || !strings.Contains(step.Stdout, `"tier":"standard"`) {
		t.Fatalf("unexpected no-command risk QA step: %+v", step)
	}

	gitRoot := t.TempDir()
	runRiskQATestCommand(t, gitRoot, "git", "init")
	if plan := Plan(gitRoot); plan.Tier != "standard" || !containsString(plan.Reasons, "working tree has no local changes") {
		t.Fatalf("unexpected clean git risk plan: %+v", plan)
	}

	jsonText := PlanJSON(riskqacontract.RiskQATierPlan{Tier: "static", ChangedPaths: []string{"b.go", "a.go"}, Commands: []string{"go vet ./..."}})
	if !strings.Contains(jsonText, `"tier":"static"`) || !strings.Contains(jsonText, `"changed_paths":["b.go","a.go"]`) {
		t.Fatalf("unexpected risk QA JSON: %s", jsonText)
	}
}

func TestValidateRiskQATierWithDepsCoversCommandSuccessAndFailure(t *testing.T) {
	root := t.TempDir()
	plan := riskqacontract.RiskQATierPlan{
		Tier:         "elevated",
		ChangedPaths: []string{"cmd/issueops/risk_qa.go"},
		Reasons:      []string{"go changes detected"},
		Commands:     []string{"go test -race ./... -count=1", "go vet ./..."},
	}
	calls := []string{}
	success := ValidateWithDeps(root, riskqacontractx.ExecuteDeps{
		Plan: func(gotRoot string) riskqacontract.RiskQATierPlan {
			if gotRoot != root {
				t.Fatalf("plan root=%q want %q", gotRoot, root)
			}
			return plan
		},
		Run: func(gotRoot string, command string) selfverify.StepResult {
			if gotRoot != root || (command != "go test -race ./... -count=1" && command != "go vet ./...") {
				t.Fatalf("unexpected risk QA command in %q: %q", gotRoot, command)
			}
			calls = append(calls, command)
			return selfverify.StepResult{Label: command, Command: "stub " + command, OK: true, Stdout: "ok " + command}
		},
	})
	if !success.OK || success.Command != "stub go test -race ./... -count=1 && stub go vet ./..." {
		t.Fatalf("unexpected successful risk QA result: %+v", success)
	}
	if !sameStringSlice(calls, plan.Commands) || !strings.Contains(success.Stdout, `"tier":"elevated"`) || !strings.Contains(success.Stdout, "ok go vet ./...") {
		t.Fatalf("risk QA success did not preserve command order/stdout: calls=%v result=%+v", calls, success)
	}

	failure := ValidateWithDeps(root, riskqacontractx.ExecuteDeps{
		Plan: func(string) riskqacontract.RiskQATierPlan { return plan },
		Run: func(_ string, command string) selfverify.StepResult {
			if command == "go test -race ./... -count=1" {
				return selfverify.StepResult{Label: "risk QA race test", Command: "stub race", OK: false, Error: "race failed", Stdout: "race out"}
			}
			t.Fatalf("failure path should stop after first failing command, got %q", command)
			return selfverify.StepResult{}
		},
	})
	if failure.OK || failure.Label != "risk QA tier" || !strings.Contains(failure.Command, "stub race") || !strings.Contains(failure.Error, "race failed") {
		t.Fatalf("unexpected failing risk QA result: %+v", failure)
	}
}

func TestRiskQARaceTimeoutCoversFullRepoRaceGate(t *testing.T) {
	if riskQARaceTimeout < 10*time.Minute {
		t.Fatalf("risk QA race timeout = %s, want at least 10m for full repo race gate", riskQARaceTimeout)
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func runRiskQATestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func writeFileForWrapperTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScopedPlanUnionPreservesNULPathsAndRenameSources(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	for _, name := range []string{"internal/old.go", "staged.txt", "unstaged.txt"} {
		writeFileForWrapperTest(t, filepath.Join(root, name), "base\n")
	}
	runRiskQATestCommand(t, root, "git", "add", ".")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "base")
	runRiskQATestCommand(t, root, "git", "mv", "internal/old.go", "internal/new.txt")
	committed := []string{"plain.txt", " plain.txt", "plain.txt ", "\nplain.txt\n", "quote\".go", "space name.go"}
	for _, name := range committed {
		writeFileForWrapperTest(t, filepath.Join(root, name), "committed\n")
	}
	runRiskQATestCommand(t, root, "git", "add", ".")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "paths")
	writeFileForWrapperTest(t, filepath.Join(root, "staged.txt"), "staged\n")
	runRiskQATestCommand(t, root, "git", "add", "staged.txt")
	writeFileForWrapperTest(t, filepath.Join(root, "unstaged.txt"), "unstaged\n")
	writeFileForWrapperTest(t, filepath.Join(root, "untracked\n.go"), "untracked\n")
	runRiskQATestCommand(t, root, "git", "mv", "internal/new.txt", "internal/renamed.txt")
	plan := PlanWithScope(root, "HEAD~1")
	for _, name := range append(committed, "internal/old.go", "internal/new.txt", "internal/renamed.txt", "staged.txt", "unstaged.txt", "untracked\n.go") {
		if !containsString(plan.ChangedPaths, name) {
			t.Errorf("lost exact path %q: %#v", name, plan.ChangedPaths)
		}
	}
	if plan.Tier != "elevated" || plan.Scope == nil || len(plan.Scope.BaseSHA) != 40 || len(plan.Scope.HeadSHA) != 40 || plan.Scope.Error != "" {
		t.Fatalf("unexpected scope: %+v", plan)
	}
	// Omitted scope keeps committed-only paths out of the plan.
	if containsString(Plan(root).ChangedPaths, "internal/old.go") {
		t.Fatal("unscoped plan included committed path")
	}
}

func TestScopedPlanEmptyAndInvalidRefs(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	if plan := PlanWithScope(root, "HEAD"); plan.Scope == nil || plan.Scope.Error == "" {
		t.Fatalf("unborn scope accepted: %+v", plan)
	}
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "base")
	plan := PlanWithScope(root, "HEAD")
	if plan.Scope == nil || plan.Scope.Error != "" || len(plan.Commands) != 0 || len(plan.ChangedPaths) != 0 || plan.Scope.BaseSHA != plan.Scope.HeadSHA {
		t.Fatalf("valid empty scope: %+v", plan)
	}
	for _, ref := range []string{"absent-ref", "--all", "HEAD^{tree}", " "} {
		t.Run(ref, func(t *testing.T) {
			deps := defaultDeps()
			deps.Plan = func(string) riskqacontract.RiskQATierPlan { return PlanWithScope(root, ref) }
			deps.Run = func(string, string) selfverify.StepResult {
				t.Fatal("invalid scope ran a command")
				return selfverify.StepResult{}
			}
			step := ValidateWithDeps(root, deps)
			if step.OK || step.Error == "" || !strings.Contains(step.Stdout, `"error"`) {
				t.Fatalf("invalid ref accepted: %+v", step)
			}
			if riskqadomain.CoversFullGoTest(deps.Plan(root)) {
				t.Fatal("invalid scope claims coverage")
			}
		})
	}
	step, coverage := ValidateForSelfVerifyWithScope(t.TempDir(), "HEAD")
	if step.OK || coverage || step.Error == "" {
		t.Fatalf("non-repo accepted: %+v coverage=%v", step, coverage)
	}
}

func TestScopedPlanUsesTreeRangeAndPreservesObservationFailures(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "common")
	writeFileForWrapperTest(t, filepath.Join(root, "internal", "base-only.go"), "package internal\n")
	runRiskQATestCommand(t, root, "git", "add", ".")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "base side")
	runRiskQATestCommand(t, root, "git", "tag", "base-side")
	runRiskQATestCommand(t, root, "git", "checkout", "-q", "--detach", "HEAD~1")
	runRiskQATestCommand(t, root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "head side")
	plan := PlanWithScope(root, "base-side")
	if !containsString(plan.ChangedPaths, "internal/base-only.go") || plan.Tier != "elevated" {
		t.Fatalf("tree range replaced by merge-base: %+v", plan)
	}
	for _, failure := range []string{"diff", "status"} {
		t.Run(failure, func(t *testing.T) {
			fake := t.TempDir()
			script := "#!/bin/sh\ncase \"$3\" in rev-parse) printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\\n';; " + failure + ") exit 7;; diff|status) :;; *) exit 8;; esac\n"
			if err := os.WriteFile(filepath.Join(fake, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			plan := PlanWithScope(root, "base-side")
			if plan.Scope == nil || plan.Scope.Error == "" || len(plan.Scope.BaseSHA) != 40 || len(plan.Scope.HeadSHA) != 40 || riskqadomain.CoversFullGoTest(plan) {
				t.Fatalf("observation failure accepted: %+v", plan)
			}
		})
	}
}
