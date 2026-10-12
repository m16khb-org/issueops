package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	install "issueops/internal/adapter/install"
	"issueops/internal/port"
)

func edgeRequest(t *testing.T) (port.NativeInstallRequest, string) {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	return req, filepath.Join(home, ".codex")
}

func stdioBlock(req port.NativeInstallRequest) string {
	return "[mcp_servers.issueops]\ncommand = " + `"` + req.BinPath + `"` + "\nargs = [\"mcp\"]\nstartup_timeout_sec = 30\n\n[mcp_servers.issueops.env]\nISSUEOPS_ROOT = " + `"` + req.Root + `"` + "\n"
}

func fileFor(t *testing.T, result port.HostInstallResult, kind string) port.InstallFile {
	t.Helper()
	for _, f := range result.Files {
		if f.Kind == kind {
			return f
		}
	}
	t.Fatalf("no %s file in result: %+v", kind, result.Files)
	return port.InstallFile{}
}

func skipIfPermissionsIgnored(t *testing.T) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(probe, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		t.Skip("file permissions are not enforced for this user")
	}
}

func TestCodexInstallFreshConfigHasNoLeadingBlankLine(t *testing.T) {
	req, codexHome := edgeRequest(t)
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != stdioBlock(req) {
		t.Fatalf("config =\n%q\nwant\n%q", got, stdioBlock(req))
	}
}

func TestCodexInstallSeparatesUnterminatedConfigWithBlankLine(t *testing.T) {
	req, codexHome := edgeRequest(t)
	writeFile(t, filepath.Join(codexHome, "config.toml"), `model = "x"`)
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "model = \"x\"\n\n" + stdioBlock(req); string(got) != want {
		t.Fatalf("config =\n%q\nwant\n%q", got, want)
	}
}

func TestCodexInstallReplacesIssueopsSectionsAtEveryPosition(t *testing.T) {
	cases := map[string]struct{ existing, want string }{
		"first in file": {
			existing: "[mcp_servers.issueops]\ncommand = \"/old\"\n\n[mcp_servers.other]\ncommand = \"other\"\n",
			want:     "[mcp_servers.other]\ncommand = \"other\"\n\n",
		},
		"empty section followed by another": {
			existing: "[mcp_servers.issueops]\n[mcp_servers.other]\ncommand = \"other\"\n",
			want:     "[mcp_servers.other]\ncommand = \"other\"\n\n",
		},
		"between other sections": {
			existing: "[a]\nx = 1\n\n[mcp_servers.issueops]\ncommand = \"/old\"\n\n[b]\ny = 2\n",
			want:     "[a]\nx = 1\n\n[b]\ny = 2\n\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req, codexHome := edgeRequest(t)
			writeFile(t, filepath.Join(codexHome, "config.toml"), tc.existing)
			if _, err := testInstaller().Install(req); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if want := tc.want + stdioBlock(req); string(got) != want {
				t.Fatalf("config =\n%q\nwant\n%q", got, want)
			}
		})
	}
}

func TestCodexInstallFailsWhenConfigIsNotWritable(t *testing.T) {
	skipIfPermissionsIgnored(t)
	req, codexHome := edgeRequest(t)
	path := filepath.Join(codexHome, "config.toml")
	writeFile(t, path, "model = \"x\"\n")
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	result, err := testInstaller().Install(req)
	if err == nil || result.OK {
		t.Fatalf("read-only config must fail the install: ok=%v err=%v", result.OK, err)
	}
	if fileFor(t, result, "codex_user_mcp_config").Written {
		t.Fatalf("failed config write reported as written: %+v", result.Files)
	}
}

func TestCodexInstallFailsWhenHooksFileIsNotWritable(t *testing.T) {
	skipIfPermissionsIgnored(t)
	req, codexHome := edgeRequest(t)
	path := filepath.Join(codexHome, "hooks.json")
	writeFile(t, path, `{"hooks":{}}`)
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	result, err := testInstaller().Install(req)
	if err == nil || result.OK {
		t.Fatalf("read-only hooks file must fail the install: ok=%v err=%v", result.OK, err)
	}
	if fileFor(t, result, "codex_user_hooks_config").Written {
		t.Fatalf("failed hooks write reported as written: %+v", result.Files)
	}
}

func TestCodexInstallRefusesToOverwriteUnreadableHooksFile(t *testing.T) {
	skipIfPermissionsIgnored(t)
	req, codexHome := edgeRequest(t)
	path := filepath.Join(codexHome, "hooks.json")
	writeFile(t, path, `{"hooks":{}}`)
	if err := os.Chmod(path, 0o200); err != nil {
		t.Fatal(err)
	}
	result, err := testInstaller().Install(req)
	if err == nil || result.OK {
		t.Fatalf("unreadable hooks file must fail the install: ok=%v err=%v", result.OK, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != `{"hooks":{}}` {
		t.Fatalf("unreadable hooks file was overwritten: %q", got)
	}
}

func TestCodexInstallTreatsBlankHooksFileAsEmpty(t *testing.T) {
	req, codexHome := edgeRequest(t)
	path := filepath.Join(codexHome, "hooks.json")
	writeFile(t, path, " \n\t\n")
	result, err := testInstaller().Install(req)
	if err != nil || !result.OK {
		t.Fatalf("blank hooks file must install: ok=%v err=%v", result.OK, err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(got), "hook session-start --host codex") {
		t.Fatalf("hooks not written over blank file:\n%s", got)
	}
}

func TestCodexInstallReportsUnchangedFilesAsNotWritten(t *testing.T) {
	req, codexHome := edgeRequest(t)
	writeFile(t, filepath.Join(codexHome, "config.toml"), "model = \"x\"\n")
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	for _, dryRun := range []bool{false, true} {
		req.DryRun = dryRun
		result, err := testInstaller().Install(req)
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"codex_user_mcp_config", "codex_user_hooks_config"} {
			if f := fileFor(t, result, kind); f.Written || f.WouldWrite {
				t.Fatalf("dryRun=%v: unchanged %s reported as pending write: %+v", dryRun, kind, f)
			}
		}
	}
}

func TestCodexInstallReportsHookBuildGenerationMismatch(t *testing.T) {
	setup := func(t *testing.T) port.NativeInstallRequest {
		req, codexHome := edgeRequest(t)
		writeFile(t, filepath.Join(codexHome, "hooks.json"),
			`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"'`+req.BinPath+`' hook session-start --host codex"}]}]}}`)
		return req
	}
	t.Run("reported when every dependency is wired", func(t *testing.T) {
		req := setup(t)
		deps := testDependencies()
		deps.RunningBuildGenerationString = func() string { return "gen-cli" }
		deps.FileBuildGenerationString = func(string) string { return "gen-hook" }
		result, err := NewInstaller(deps).Install(req)
		if err != nil {
			t.Fatal(err)
		}
		want := "codex native hook target is a different build than this CLI: hook=" + req.BinPath + " (gen-hook) cli=gen-cli;"
		found := 0
		for _, m := range result.Messages {
			if strings.HasPrefix(m, want) {
				found++
			}
		}
		if found != 1 {
			t.Fatalf("messages = %#v, want exactly one starting with %q", result.Messages, want)
		}
	})
	t.Run("skipped without a generation reader or running generation", func(t *testing.T) {
		for name, mutate := range map[string]func(*Dependencies){
			"no message source": func(d *Dependencies) { d.HookTargetGenerationMessages = nil },
			"no running source": func(d *Dependencies) { d.RunningBuildGenerationString = nil },
			"no file source":    func(d *Dependencies) { d.FileBuildGenerationString = nil },
		} {
			t.Run(name, func(t *testing.T) {
				req := setup(t)
				deps := testDependencies()
				deps.RunningBuildGenerationString = func() string { return "gen-cli" }
				deps.FileBuildGenerationString = func(string) string { return "gen-hook" }
				mutate(&deps)
				result, err := NewInstaller(deps).Install(req)
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range result.Messages {
					if strings.Contains(m, "different build") {
						t.Fatalf("generation message emitted with missing dependency: %q", m)
					}
				}
			})
		}
	})
}
