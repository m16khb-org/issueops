package codex

import (
	"issueops/internal/port"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallerInstancesKeepSkillPlanner(t *testing.T) {
	var calls []string
	makeInstaller := func(owner string) Installer {
		deps := testDependencies()
		deps.PlanHostSkillLinks = func(root, dest string, names []string, host string, dry bool) ([]string, []port.InstallLink, []string, []error) {
			calls = append(calls, owner)
			return nil, nil, nil, nil
		}
		return NewInstaller(deps)
	}
	a, b := makeInstaller("first"), makeInstaller("second")
	root, home := t.TempDir(), t.TempDir()
	req := port.NativeInstallRequest{Root: root, Home: home, CodexHome: filepath.Join(home, ".codex"), BinPath: filepath.Join(root, "bin", "issueops"), DryRun: true}
	for _, installer := range []Installer{a, b, a} {
		if _, err := installer.Install(req); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"first", "second", "first"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("planners=%v want=%v", calls, want)
	}
}
