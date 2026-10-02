package selfverify

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/verification"
)

func evidenceRunner(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
	return verification.Run(root, label, timeout, stdin, 16384, name, args...)
}

func writeEvidenceFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGoldenFallbackRequiresActualPassingTests(t *testing.T) {
	for _, mode := range []string{"pass", "no-match", "skip", "fail"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeEvidenceFixture(t, filepath.Join(root, "go.mod"), "module issueops\n\ngo 1.26\n")
			writeEvidenceFixture(t, filepath.Join(root, "cmd/issueops/main_test.go"), "package main\nimport \"testing\"\nfunc TestUnrelated(t *testing.T) {}\n")
			names := map[string][]string{"contractgolden": {"TestCLIUsageGolden", "TestMCPToolsGolden", "TestMCPResourcesGolden"}, "issueopsapp": {"TestResponseContractsGolden"}}
			for pkg, tests := range names {
				body := "package " + pkg + "\nimport \"testing\"\n"
				for _, name := range tests {
					action := ""
					if name == "TestResponseContractsGolden" {
						switch mode {
						case "no-match":
							name = "TestUnrelated"
						case "skip":
							action = "t.Skip(\"fixture skip\")"
						case "fail":
							action = "t.Fatal(\"fixture failure\")"
						}
					}
					body += "func " + name + "(t *testing.T) {" + action + "}\n"
				}
				writeEvidenceFixture(t, filepath.Join(root, "cmd/issueops", pkg, "golden_test.go"), body)
			}
			result := CachedContractGoldenStep(StepResult{OK: false}, SelfVerifyStepDeps{IssueOpsRoot: func() string { return root }, RunCommandStep: evidenceRunner})
			if result.OK != (mode == "pass") {
				t.Fatalf("%s: %+v", mode, result)
			}
			if mode == "pass" && !strings.Contains(result.Stdout, "TestResponseContractsGolden") {
				t.Fatalf("named tests not executed: %+v", result)
			}
			if !result.OK && result.Error == "" {
				t.Fatalf("missing failure diagnosis: %+v", result)
			}
		})
	}
}

func TestBinaryDriftUsesRealDoctorObservation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "issueops")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/issueops")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build doctor: %s %v", out, err)
	}
	for _, mode := range []string{"stale", "fresh", "missing"} {
		t.Run(mode, func(t *testing.T) {
			fixture := t.TempDir()
			source := filepath.Join(fixture, "internal/source.go")
			writeEvidenceFixture(t, source, "package fixture\n")
			now := time.Now().Add(-time.Hour)
			if err := os.Chtimes(source, now, now); err != nil {
				t.Fatal(err)
			}
			if mode != "missing" {
				target := filepath.Join(fixture, "bin/issueops")
				writeEvidenceFixture(t, target, "fixture binary")
				stamp := now.Add(time.Hour)
				if mode == "stale" {
					stamp = now.Add(-time.Hour)
				}
				if err := os.Chtimes(target, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			var raw StepResult
			deps := SelfVerifyStepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
				raw = evidenceRunner(root, label, timeout, stdin, name, args...)
				return raw
			}}
			var full StepResult
			result := PlannedSteps(fixture, bin, 100, &full, deps)[8].Run()
			if !raw.OK {
				t.Fatalf("doctor exit contract changed: %+v", raw)
			}
			var doctor struct {
				Checks []struct {
					Name    string
					Healthy bool
				}
			}
			if err := json.Unmarshal([]byte(raw.Stdout), &doctor); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, check := range doctor.Checks {
				if check.Name == "binary_drift" {
					found = true
					if check.Healthy != (mode != "stale") {
						t.Fatalf("fixture observation: %+v", check)
					}
				}
			}
			if !found || result.OK != (mode != "stale") {
				t.Fatalf("%s: %+v", mode, result)
			}
		})
	}
}

func TestBinaryDriftRejectsInvalidEvidenceAndPreservesCommandFailure(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"fresh", `{"checks":[{"name":"binary_drift","healthy":true}]}`, true},
		{"unrelated warning", `{"healthy":false,"checks":[{"name":"other","healthy":false},{"name":"binary_drift","healthy":true}]}`, true},
		{"stale", `{"checks":[{"name":"binary_drift","healthy":false}]}`, false},
		{"missing", `{"checks":[]}`, false},
		{"duplicate", `{"checks":[{"name":"binary_drift","healthy":true},{"name":"binary_drift","healthy":true}]}`, false},
		{"missing bool", `{"checks":[{"name":"binary_drift"}]}`, false},
		{"wrong bool", `{"checks":[{"name":"binary_drift","healthy":"true"}]}`, false},
		{"null bool", `{"checks":[{"name":"binary_drift","healthy":null}]}`, false},
		{"malformed", `{"checks":`, false},
		{"trailing", `{"checks":[{"name":"binary_drift","healthy":true}]} {}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := SelfVerifyStepDeps{RunCommandStep: func(string, string, time.Duration, string, string, ...string) StepResult {
				return StepResult{Label: "binary drift", OK: true, Stdout: tc.body}
			}}
			var full StepResult
			result := PlannedSteps("/repo", "/bin", 100, &full, deps)[8].Run()
			if result.OK != tc.ok || (!tc.ok && result.Error == "") {
				t.Fatalf("%s: %+v", tc.name, result)
			}
		})
	}
	original := StepResult{Label: "binary drift", OK: false, Error: "timeout after 10s", Stdout: "partial", DurationMS: 10000}
	deps := SelfVerifyStepDeps{RunCommandStep: func(string, string, time.Duration, string, string, ...string) StepResult { return original }}
	var full StepResult
	if got := PlannedSteps("/repo", "/bin", 100, &full, deps)[8].Run(); !reflect.DeepEqual(got, original) {
		t.Fatalf("command failure changed: %+v", got)
	}
}

func TestGoldenFallbackRejectsIncompleteJSONEvidence(t *testing.T) {
	valid := ""
	for _, tc := range []struct{ pkg, test string }{
		{"issueops/cmd/issueops/contractgolden", "TestCLIUsageGolden"},
		{"issueops/cmd/issueops/contractgolden", "TestMCPToolsGolden"},
		{"issueops/cmd/issueops/contractgolden", "TestMCPResourcesGolden"},
		{"issueops/cmd/issueops/issueopsapp", "TestResponseContractsGolden"},
	} {
		for _, action := range []string{"run", "pass"} {
			event, err := json.Marshal(map[string]string{"Package": tc.pkg, "Test": tc.test, "Action": action})
			if err != nil {
				t.Fatal(err)
			}
			valid += string(event) + "\n"
		}
	}
	valid += `{"Action":"pass","Package":"issueops/cmd/issueops/contractgolden"}` + "\n"
	valid += `{"Action":"pass","Package":"issueops/cmd/issueops/issueopsapp"}` + "\n"
	for _, tc := range []struct {
		name, body    string
		truncated, ok bool
	}{
		{"complete", valid, false, true},
		{"no run", strings.ReplaceAll(valid, `"Action":"run"`, `"Action":"output"`), false, false},
		{"no package pass", strings.ReplaceAll(valid, `{"Action":"pass","Package":"issueops/cmd/issueops/issueopsapp"}`, ""), false, false},
		{"malformed", valid + "{", false, false},
		{"budget truncated", valid, true, false},
		{"empty", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := SelfVerifyStepDeps{IssueOpsRoot: func() string { return "/repo" }, RunCommandStep: func(string, string, time.Duration, string, string, ...string) StepResult {
				return StepResult{OK: true, Stdout: tc.body, StdoutTruncated: tc.truncated}
			}}
			got := CachedContractGoldenStep(StepResult{OK: false}, deps)
			if got.OK != tc.ok || (!tc.ok && got.Error == "") {
				t.Fatalf("%s: %+v", tc.name, got)
			}
		})
	}
}
