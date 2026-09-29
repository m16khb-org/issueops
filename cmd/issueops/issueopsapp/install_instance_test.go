package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/installcli"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRootsAndAllHostReadbacksStayIsolated(t *testing.T) {
	makeDeps := func() (installcli.Deps, string, string, string) {
		root, home, state := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "state")
		skill := filepath.Join(root, "skills", "fixture", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skill), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, []byte("---\nname: fixture\ndescription: fixture\n---\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISSUEOPS_ROOT", root)
		t.Setenv("ISSUEOPS_STATE_DIR", state)
		return installDependencies(), root, home, state
	}
	a, ar, ah, as := makeDeps()
	b, br, bh, bs := makeDeps()
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, entry := range []struct {
		deps              installcli.Deps
		root, home, state string
	}{{a, ar, ah, as}, {b, br, bh, bs}, {a, ar, ah, as}} {
		if entry.deps.IssueOpsRoot() != entry.root || filepath.Dir(entry.deps.StateRoot) != entry.state {
			t.Fatalf("root or state drifted: %+v", entry.deps)
		}
		req := entry.deps.NativeInstallRequest(entry.deps.IssueOpsRoot(), entry.home, filepath.Join(entry.home, ".codex"), filepath.Join(entry.root, "bin", "issueops"))
		result, err := entry.deps.InstallNative(req)
		if err != nil || !result.OK || len(result.Hosts) != 4 {
			t.Fatalf("install=%+v err=%v", result, err)
		}
		evidence, err := entry.deps.ActivationReadback(req).Verify(context.Background(), req.Root, req.BinPath)
		if err != nil || len(evidence.Evidence) != 7 {
			t.Fatalf("readback=%+v err=%v", evidence, err)
		}
		for _, item := range evidence.Evidence {
			if !strings.HasPrefix(item.Path, entry.home+string(filepath.Separator)) {
				t.Fatalf("crossed home: %s", item.Path)
			}
		}
		if _, err := entry.deps.ActivationReadback(req).Verify(context.Background(), t.TempDir(), req.BinPath); err == nil {
			t.Fatal("changed activation target accepted")
		}
		if _, err := os.Stat(entry.state); !os.IsNotExist(err) {
			t.Fatalf("host install unexpectedly wrote state: %v", err)
		}
	}
}
