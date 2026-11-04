package selfverify

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/verification"
	contract "issueops/internal/contract/selfverify"
	"issueops/internal/testsupport"
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
	deps := StepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
		return verification.Run(root, label, timeout, stdin, 16384, name, args...)
	}}
	var goTest contract.StepResult
	requirePython3(t)
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

// requirePython3 returns the interpreter behind `python3` and guarantees that
// `python3` resolves by name for the step under test. Windows installs Python
// as python.exe (python3.exe is a Microsoft Store stub), so a `python3`
// command that forwards to the real interpreter is installed on PATH there;
// the test skips when no usable Python 3.10+ exists.
func requirePython3(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Fatal(err)
		}
		python, err = filepath.Abs(python)
		if err != nil {
			t.Fatal(err)
		}
		return python
	}
	python, err := exec.LookPath("python")
	if err != nil || strings.Contains(strings.ToLower(python), "windowsapps") {
		t.Skip("no python.exe on PATH: Windows names the interpreter python, and python3.exe is a Store stub")
	}
	if err := exec.Command(python, "-c", "import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)").Run(); err != nil {
		t.Skipf("python on PATH is not a working Python 3.10+: %v", err)
	}
	tools := t.TempDir()
	testsupport.WriteFakeCommand(t, tools, "python3", "exec '"+python+"' \"$@\"\n")
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	return python
}

// writeUnsupportedPython3 installs a `python3` in dir that runs the -c source
// it receives under a faked Python 3.9 version.
func writeUnsupportedPython3(t *testing.T, dir, python string) {
	t.Helper()
	const body = "import sys\nsys.version_info=(3,9,0)\nsys.version='3.9.0 fixture'\nexec(sys.argv[2])\n"
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(filepath.Join(dir, "python3"), []byte("#!"+python+"\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
		return
	}
	shim := filepath.Join(dir, "unsupported_python3.py")
	if err := os.WriteFile(shim, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFakeCommand(t, dir, "python3", "exec '"+python+"' '"+shim+"' \"$@\"\n")
}
