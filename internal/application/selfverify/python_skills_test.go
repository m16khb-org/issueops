package selfverify

import (
	contract "issueops/internal/contract/selfverify"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/verification"
)

func TestPythonSkillSuitesExecuteWithLocalContext(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"scripts", "skills/alpha/tests", "skills/beta/scripts"} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0700); err != nil {
			t.Fatal(err)
		}
	}
	copyPythonSuiteRunner(t, root)
	for _, skill := range []string{"alpha", "beta"} {
		dir := "tests"
		if skill == "beta" {
			dir = "scripts"
		}
		path := filepath.Join(root, "skills", skill, dir, "test_context_test.py")
		body := "import unittest\nfrom pathlib import Path\nimport helper\nclass ContextTest(unittest.TestCase):\n    def test_context(self):\n        self.assertEqual(Path.cwd().name, '" + skill + "')\n        self.assertEqual(helper.VALUE, '" + skill + "')\n        with Path('executed').open('a') as log:\n            log.write('once\\n')\n"
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), "helper.py"), []byte("VALUE = '"+skill+"'\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "fixture_test.py"), []byte("import unittest\nclass RootTest(unittest.TestCase):\n    def test_root(self):\n        pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	deps := SelfVerifyStepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
		return verification.Run(root, label, timeout, stdin, 16384, name, args...)
	}}
	var goTest contract.StepResult
	step := PlannedSteps(root, "unused", 100, &goTest, deps)[2].Run()
	if !step.OK {
		t.Fatalf("%+v", step)
	}
	for _, skill := range []string{"alpha", "beta"} {
		content, err := os.ReadFile(filepath.Join(root, "skills", skill, "executed"))
		if err != nil || strings.TrimSpace(string(content)) != "once" {
			t.Fatalf("skill %s execution count: %q %v; %+v", skill, content, err, step)
		}
	}
}

func copyPythonSuiteRunner(t *testing.T, root string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "python_suite_runner.py"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "python_suite_runner.py"), content, 0600); err != nil {
		t.Fatal(err)
	}
}
