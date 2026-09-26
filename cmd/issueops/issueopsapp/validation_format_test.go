package issueopsapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGoFormatUsesProductionVerificationDriver(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "messy.go")
	if err := os.WriteFile(file, []byte("package sample\n\nvar   Value   =   1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "messy.go"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	step := validateGoFormat(root)
	if step.OK || step.Label != "gofmt" || !strings.Contains(step.Error, "messy.go") {
		t.Fatalf("unformatted result=%+v", step)
	}
	if out, err := exec.Command("gofmt", "-w", file).CombinedOutput(); err != nil {
		t.Fatalf("gofmt: %v\n%s", err, out)
	}
	step = validateGoFormat(root)
	if !step.OK || step.Error != "" || !strings.Contains(step.Stdout, "checked 1 tracked .go file(s)") {
		t.Fatalf("formatted result=%+v", step)
	}
}
