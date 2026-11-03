//go:build unix

package installcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestPreferredShellRCAndAppendShellPathLinePlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/bash")
	if got := preferredShellRC(home); got != filepath.Join(home, ".bashrc") {
		t.Fatalf("bash shell rc = %q", got)
	}

	t.Setenv("SHELL", "/bin/unknown")
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := preferredShellRC(home); got != zshrc {
		t.Fatalf("existing zsh rc = %q", got)
	}

	dryRunFile, err := appendShellPathLinePlan(filepath.Join(home, ".profile"), true)
	if err != nil {
		t.Fatalf("dry-run append path failed: %v", err)
	}
	if !dryRunFile.WouldWrite || dryRunFile.Written || dryRunFile.Kind != "shell_path_rc" {
		t.Fatalf("unexpected dry-run shell path file: %#v", dryRunFile)
	}

	rcPath := filepath.Join(home, "nested", ".profile")
	writtenFile, err := appendShellPathLinePlan(rcPath, false)
	if err != nil {
		t.Fatalf("append path failed: %v", err)
	}
	if !writtenFile.Written || writtenFile.WouldWrite {
		t.Fatalf("unexpected written shell path file: %#v", writtenFile)
	}
	body, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), ShellPathRCMarker) || !shellRCAlreadyAddsLocalBin(rcPath, home) {
		t.Fatalf("shell rc did not contain local bin marker:\n%s", string(body))
	}
}

func TestPlanShellPathHonorsModeExistingPathAndExistingRCLine(t *testing.T) {
	newReq := func(t *testing.T) port.NativeInstallRequest {
		home := t.TempDir()
		t.Setenv("SHELL", "/bin/zsh")
		t.Setenv("PATH", "/usr/bin:/bin")
		return port.NativeInstallRequest{Home: home, DryRun: true}
	}
	t.Run("manual and skip only add guidance", func(t *testing.T) {
		for mode, want := range map[string]string{"manual": "path-mode=manual", "skip": "path-mode=skip"} {
			req := newReq(t)
			var result port.NativeInstallResult
			if err := planShellPath(&result, req, mode); err != nil {
				t.Fatal(err)
			}
			if len(result.Files) != 0 || len(result.Messages) != 1 || !strings.HasPrefix(result.Messages[0], want) {
				t.Fatalf("%s: %+v", mode, result)
			}
		}
	})
	t.Run("local bin already on PATH changes nothing", func(t *testing.T) {
		req := newReq(t)
		t.Setenv("PATH", "/usr/bin:"+filepath.Join(req.Home, ".local", "bin", "."))
		var result port.NativeInstallResult
		if err := planShellPath(&result, req, "auto"); err != nil || len(result.Files)+len(result.Messages) != 0 {
			t.Fatalf("result = %+v, %v", result, err)
		}
	})
	t.Run("rc that already exports local bin changes nothing", func(t *testing.T) {
		req := newReq(t)
		writeInstallFixtureFile(t, filepath.Join(req.Home, ".zshrc"), "export PATH=\"$HOME/.local/bin:$PATH\"\n", 0o644)
		var result port.NativeInstallResult
		if err := planShellPath(&result, req, "auto"); err != nil || len(result.Files)+len(result.Messages) != 0 {
			t.Fatalf("result = %+v, %v", result, err)
		}
	})
	t.Run("dry run plans an rc edit without writing", func(t *testing.T) {
		req := newReq(t)
		var result port.NativeInstallResult
		if err := planShellPath(&result, req, "auto"); err != nil {
			t.Fatal(err)
		}
		rc := filepath.Join(req.Home, ".zshrc")
		if len(result.Files) != 1 || result.Files[0].Path != rc || !result.Files[0].WouldWrite || !strings.HasPrefix(result.Messages[0], "dry-run: would add") {
			t.Fatalf("result = %+v", result)
		}
		if _, err := os.Stat(rc); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the rc file")
		}
	})
	t.Run("apply appends the marker and export once", func(t *testing.T) {
		req := newReq(t)
		req.DryRun = false
		var result port.NativeInstallResult
		if err := planShellPath(&result, req, "auto"); err != nil {
			t.Fatal(err)
		}
		body, _ := os.ReadFile(filepath.Join(req.Home, ".zshrc"))
		if !strings.Contains(string(body), shellPathRCMarker) || len(result.Files) != 1 || !result.Files[0].Written || !strings.HasPrefix(result.Messages[0], "added ~/.local/bin") {
			t.Fatalf("body=%q result=%+v", body, result)
		}
		var again port.NativeInstallResult
		if err := planShellPath(&again, req, "auto"); err != nil || len(again.Files) != 0 {
			t.Fatalf("a second run must find the marker and write nothing: %+v, %v", again, err)
		}
	})
	t.Run("unwritable rc location is an error and records no file", func(t *testing.T) {
		req := newReq(t)
		req.DryRun = false
		writeInstallFixtureFile(t, filepath.Join(req.Home, ".zshrc"), "", 0o644)
		if err := os.Remove(filepath.Join(req.Home, ".zshrc")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(req.Home, ".zshrc"), 0o755); err != nil {
			t.Fatal(err)
		}
		var result port.NativeInstallResult
		if err := planShellPath(&result, req, "auto"); err == nil || len(result.Files) != 0 {
			t.Fatalf("err = %v, files = %+v", err, result.Files)
		}
	})
}

func TestAppendShellPathLinePlanReportsUnwritableTargets(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	writeInstallFixtureFile(t, blocker, "x", 0o644)
	if file, err := appendShellPathLinePlan(filepath.Join(blocker, "rc"), false); err == nil || file.Written {
		t.Fatalf("mkdir through a file must fail: %+v, %v", file, err)
	}
	if file, err := appendShellPathLinePlan(root, false); err == nil || file.Written {
		t.Fatalf("opening a directory for append must fail: %+v, %v", file, err)
	}
}

func TestShellRCDetectionRecognizesEveryExportFormAndFallsBackToProfile(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, "rc")
	cases := map[string]bool{
		shellPathRCMarker + "\n":                          true,
		"export PATH=\"$HOME/.local/bin:$PATH\"\n":        true,
		"export PATH=\"" + home + "/.local/bin:$PATH\"\n": true,
		"export PATH=\"/other/bin:$PATH\"\n":              false,
		"":                                                false,
	}
	for body, want := range cases {
		writeInstallFixtureFile(t, rc, body, 0o644)
		if got := shellRCAlreadyAddsLocalBin(rc, home); got != want {
			t.Errorf("shellRCAlreadyAddsLocalBin(%q) = %v, want %v", body, got, want)
		}
	}
	if shellRCAlreadyAddsLocalBin(filepath.Join(home, "absent"), home) {
		t.Error("a missing rc cannot already contain the line")
	}

	t.Setenv("SHELL", "/usr/bin/zsh")
	if got := preferredShellRC(home); got != filepath.Join(home, ".zshrc") {
		t.Errorf("zsh rc = %q", got)
	}
	t.Setenv("SHELL", "/usr/bin/fish")
	if got := preferredShellRC(home); got != filepath.Join(home, ".profile") {
		t.Errorf("fish with no rc files falls back to .profile, got %q", got)
	}
}
