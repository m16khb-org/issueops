package issueopsapp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/apidoc"
	contract "issueops/internal/contract/apidoc"
	"issueops/internal/testsupport"
)

func TestAPIDocCommandsCaptureCWDWithoutHostEnvironmentOverride(t *testing.T) {
	makeRepo := func(name string) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name+".openapi.yaml"), []byte("openapi: 3.0.0\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init"}, {"add", "."}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
		}
		return dir
	}
	aRepo, bRepo := makeRepo("first"), makeRepo("second")
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	t.Chdir(aRepo)
	a := newAPIDocCommand()
	t.Chdir(bRepo)
	b := newAPIDocCommand()
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		name    string
		command apidoc.Command
	}{{"first", a}, {"second", b}, {"first", a}} {
		output, err := testsupport.CaptureStdoutAndError(t, func() error { return tc.command.Run([]string{"static-check", "--json"}) })
		if err != nil {
			t.Fatal(err)
		}
		var result contract.StaticResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Skipped || len(result.Files) != 1 || result.Files[0] != tc.name+".openapi.yaml" {
			t.Fatalf("%s result=%+v", tc.name, result)
		}
		if got := tc.command.ResolveTarget("../relative"); got != "../relative" {
			t.Fatalf("explicit target changed: %s", got)
		}
	}
}
