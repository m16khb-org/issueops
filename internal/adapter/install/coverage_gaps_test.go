package install

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
}

func TestBuildGenerationFromSettingsReadsRevisionAndDirtyFlag(t *testing.T) {
	got := buildGenerationFromSettings([]debug.BuildSetting{
		{Key: "-compiler", Value: "gc"},
		{Key: "vcs.revision", Value: "  abc123  "},
		{Key: "vcs.modified", Value: " true "},
	})
	if got != (NativeBuildGeneration{Revision: "abc123", Modified: true}) {
		t.Fatalf("generation = %+v", got)
	}
	clean := buildGenerationFromSettings([]debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "false"}})
	if clean.Modified || clean.Revision != "abc" {
		t.Fatalf("clean = %+v", clean)
	}
	if none := buildGenerationFromSettings(nil); none.Observed() {
		t.Fatalf("no settings must be unobserved: %+v", none)
	}
}

func TestGenerationStringsStayEmptyUnlessObserved(t *testing.T) {
	if got := observedGenerationString(NativeBuildGeneration{}); got != "" {
		t.Fatalf("unobserved = %q, want empty", got)
	}
	if got := observedGenerationString(NativeBuildGeneration{Revision: "0123456789abcdef", Modified: true}); got != "0123456789ab+dirty" {
		t.Fatalf("observed = %q", got)
	}
	if got := FileBuildGenerationString(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Fatalf("unreadable file = %q, want empty", got)
	}
	if got, want := RunningBuildGenerationString(), observedGenerationString(RunningBuildGeneration()); got != want {
		t.Fatalf("running string = %q, want %q", got, want)
	}
}

func TestResolveStableNativeRootRejectsUnusableRoots(t *testing.T) {
	base := t.TempDir()
	if _, err := ResolveStableNativeRoot(""); err == nil || !strings.Contains(err.Error(), "root is required") {
		t.Fatalf("empty root error = %v", err)
	}
	if got, err := ResolveStableNativeRoot(filepath.Join(base, "nogit")); err != nil || got != filepath.Join(base, "nogit") {
		t.Fatalf("missing .git must pass through: %q %v", got, err)
	}

	fileRoot := filepath.Join(base, "file")
	mustWrite(t, fileRoot, "x")
	if _, err := ResolveStableNativeRoot(fileRoot); err == nil || !strings.Contains(err.Error(), "resolve native install root") {
		t.Fatalf("file root error = %v", err)
	}

	linkRoot := filepath.Join(base, "linkroot")
	mustMkdir(t, linkRoot)
	realGit := filepath.Join(base, "realgit")
	mustMkdir(t, realGit)
	if err := os.Symlink(realGit, filepath.Join(linkRoot, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveStableNativeRoot(linkRoot); err == nil || !strings.Contains(err.Error(), "directory or regular gitdir file") {
		t.Fatalf("symlinked .git error = %v", err)
	}
}

func TestResolveStableNativeRootRejectsBrokenWorktreeMetadata(t *testing.T) {
	type fixture struct {
		gitFile    string // content of <worktree>/.git
		gitdirKind string // "dir", "file", "symlink", "missing"
		commondir  *string
		commonKind string // "git", "other-name", "file", "symlink", "missing"
	}
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name    string
		fixture fixture
		want    string
	}{
		{"multi-line gitdir file", fixture{gitFile: "gitdir: a\ngitdir: b\n"}, "exactly one value"},
		{"empty gitdir value", fixture{gitFile: "gitdir:   \n"}, "value is empty"},
		{"missing gitdir directory", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "missing"}, "resolve worktree gitdir"},
		{"gitdir is a file", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "file"}, "must be a physical directory"},
		{"gitdir is a symlink", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "symlink"}, "must be a physical directory"},
		{"commondir file absent", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir"}, "read git metadata"},
		{"commondir file empty", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("\n")}, "value is empty"},
		{"commondir multi-line", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("a\nb")}, "exactly one value"},
		{"commondir target missing", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("COMMON"), commonKind: "missing"}, "resolve worktree commondir"},
		{"commondir not named .git", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("COMMON"), commonKind: "other-name"}, "physical .git directory"},
		{"commondir is a file", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("COMMON"), commonKind: "file"}, "physical .git directory"},
		{"commondir is a symlink", fixture{gitFile: "gitdir: GITDIR\n", gitdirKind: "dir", commondir: str("COMMON"), commonKind: "symlink"}, "physical .git directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			worktree := filepath.Join(base, "wt")
			gitdir := filepath.Join(base, "gitdir")
			common := filepath.Join(base, "common", ".git")
			mustMkdir(t, worktree)
			switch tc.fixture.gitdirKind {
			case "dir", "":
				mustMkdir(t, gitdir)
			case "file":
				mustWrite(t, gitdir, "x")
			case "symlink":
				target := filepath.Join(base, "gitdir-target")
				mustMkdir(t, target)
				if err := os.Symlink(target, gitdir); err != nil {
					t.Fatal(err)
				}
			}
			if tc.fixture.gitdirKind == "dir" && tc.fixture.commondir != nil {
				value := strings.ReplaceAll(*tc.fixture.commondir, "COMMON", common)
				switch tc.fixture.commonKind {
				case "other-name":
					common = filepath.Join(base, "common", "notgit")
					value = common
					mustMkdir(t, common)
				case "file":
					mustMkdir(t, filepath.Dir(common))
					mustWrite(t, common, "x")
				case "symlink":
					target := filepath.Join(base, "common-target")
					mustMkdir(t, target)
					mustMkdir(t, filepath.Dir(common))
					if err := os.Symlink(target, common); err != nil {
						t.Fatal(err)
					}
				}
				mustWrite(t, filepath.Join(gitdir, "commondir"), value)
			}
			mustWrite(t, filepath.Join(worktree, ".git"), strings.ReplaceAll(tc.fixture.gitFile, "GITDIR", gitdir))

			got, err := ResolveStableNativeRoot(worktree)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got=%q err=%v, want error containing %q", got, err, tc.want)
			}
		})
	}
}

func TestValidateStableNativeRuntimeBoundaries(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "source")
	mustMkdir(t, filepath.Join(root, ".git"))

	for name, bin := range map[string]string{
		"sibling sharing the root name prefix": filepath.Join(base, "source-evil", "bin", "issueops"),
		"the parent of the root":               base,
		"empty path":                           "",
	} {
		if err := ValidateStableNativeRuntime(root, bin); err == nil || !strings.Contains(err.Error(), "stay inside the stable source root") {
			t.Errorf("%s: error = %v", name, err)
		}
	}
	for name, bin := range map[string]string{
		"the root itself":      root,
		"a nested custom path": filepath.Join(root, "tools", "nested", "issueops"),
	} {
		if err := ValidateStableNativeRuntime(root, bin); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}

	badRoot := filepath.Join(base, "bad")
	mustMkdir(t, badRoot)
	mustWrite(t, filepath.Join(badRoot, ".git"), "garbage\n")
	if err := ValidateStableNativeRuntime(badRoot, filepath.Join(badRoot, "bin", "issueops")); err == nil || !strings.Contains(err.Error(), "gitdir metadata is malformed") {
		t.Fatalf("unresolvable root must surface its resolution error: %v", err)
	}
}

func TestDefaultNativeInstallRequestDefaults(t *testing.T) {
	t.Run("explicit custom binary is kept even in a linked worktree", func(t *testing.T) {
		base := t.TempDir()
		source := filepath.Join(base, "source")
		worktree := filepath.Join(base, "wt")
		gitdir := filepath.Join(source, ".git", "worktrees", "wt")
		mustMkdir(t, gitdir)
		mustMkdir(t, worktree)
		mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
		mustWrite(t, filepath.Join(gitdir, "commondir"), "../..\n")
		custom := filepath.Join(base, "elsewhere", "issueops")

		req := DefaultNativeInstallRequest(worktree, base, "", custom)
		if req.Root != source || req.BinPath != custom {
			t.Fatalf("root=%q bin=%q", req.Root, req.BinPath)
		}
		if req.CodexHome != filepath.Join(base, ".codex") {
			t.Fatalf("codex home = %q", req.CodexHome)
		}
	})

	t.Run("unresolvable worktree falls back to the invoking root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "wt")
		mustMkdir(t, root)
		mustWrite(t, filepath.Join(root, ".git"), "garbage\n")
		req := DefaultNativeInstallRequest(root, t.TempDir(), "", "")
		if req.Root != root || req.BinPath != filepath.Join(root, "bin", "issueops") {
			t.Fatalf("request = %+v", req)
		}
	})

	t.Run("empty home uses the user home directory and explicit codex home wins", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		root := filepath.Join(t.TempDir(), "src")
		mustMkdir(t, root)
		req := DefaultNativeInstallRequest(root, "", "", "")
		if req.Home != home || req.CodexHome != filepath.Join(home, ".codex") {
			t.Fatalf("home=%q codex=%q", req.Home, req.CodexHome)
		}
		codex := filepath.Join(t.TempDir(), "codex")
		if got := DefaultNativeInstallRequest(root, home, codex, "").CodexHome; got != codex {
			t.Fatalf("explicit codex home = %q", got)
		}
	})

	t.Run("empty root yields no default binary", func(t *testing.T) {
		req := DefaultNativeInstallRequest("", t.TempDir(), "", "")
		if req.Root != "" || req.BinPath != "" {
			t.Fatalf("request = %+v", req)
		}
	})
}

func TestListSkillNamesRequiresSkillMarkerAndSortsNames(t *testing.T) {
	root := t.TempDir()
	writeInstallTestSkill(t, root, "zeta")
	writeInstallTestSkill(t, root, "alpha")
	mustMkdir(t, filepath.Join(root, "skills", "no-marker"))
	mustWrite(t, filepath.Join(root, "skills", "stray-file.md"), "x")
	got, err := ListSkillNames(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("skills = %v", got)
	}
	if _, err := ListSkillNames(t.TempDir()); !os.IsNotExist(err) {
		t.Fatalf("missing skills dir error = %v", err)
	}
}

func TestEnvironmentDelegatesToNativeRootRules(t *testing.T) {
	root := filepath.Join(t.TempDir(), "src")
	mustMkdir(t, filepath.Join(root, ".git"))
	env := Environment{}
	if got, err := env.ResolveStableRoot(root); err != nil || got != root {
		t.Fatalf("stable root = %q %v", got, err)
	}
	if err := env.ValidateRuntime(root, filepath.Join(root, "bin", "issueops")); err != nil {
		t.Fatal(err)
	}
	if err := env.ValidateRuntime(root, filepath.Join(filepath.Dir(root), "other")); err == nil {
		t.Fatal("outside runtime accepted")
	}
}

func TestRemoveRetiredStateReportsUnexpectedEntryTypes(t *testing.T) {
	state := t.TempDir()
	mustWrite(t, filepath.Join(state, "daemon"), "i am a file")
	mustMkdir(t, filepath.Join(state, "hook-failures.jsonl"))
	mustWrite(t, filepath.Join(state, "hook-failures.jsonl", "inner"), "x")

	messages := strings.Join(RemoveRetiredState(state, false), "\n")
	if !strings.Contains(messages, filepath.Join(state, "daemon")+" is not a directory") {
		t.Fatalf("file-as-daemon not reported: %s", messages)
	}
	if !strings.Contains(messages, filepath.Join(state, "hook-failures.jsonl")+" is not a regular file") {
		t.Fatalf("dir-as-file not reported: %s", messages)
	}
	for _, name := range []string{"daemon", "hook-failures.jsonl/inner"} {
		if !exists(filepath.Join(state, name)) {
			t.Fatalf("%s must be left untouched", name)
		}
	}
}

func TestRemoveRetiredStateDryRunStillKeepsLiveDaemon(t *testing.T) {
	state := t.TempDir()
	daemon := filepath.Join(state, "daemon")
	writeRetiredFixture(t, filepath.Join(daemon, "issueops.pid"), `{"pid":`+strconv.Itoa(os.Getpid())+`}`)
	messages := strings.Join(RemoveRetiredState(state, true), "\n")
	if !strings.Contains(messages, "legacy daemon is still running") || strings.Contains(messages, "would remove") {
		t.Fatalf("messages = %s", messages)
	}
}

func TestRemoveRetiredStateIgnoresUnusableDaemonPidRecords(t *testing.T) {
	for name, record := range map[string]string{
		"not json":      "garbage",
		"zero pid":      `{"pid":0}`,
		"negative pid":  `{"pid":-4}`,
		"missing field": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			daemon := filepath.Join(state, "daemon")
			writeRetiredFixture(t, filepath.Join(daemon, "issueops.pid"), record)
			messages := strings.Join(RemoveRetiredState(state, false), "\n")
			if exists(daemon) || !strings.Contains(messages, "removed retired state path "+daemon) {
				t.Fatalf("daemon with unusable pid record must be removed: %s", messages)
			}
		})
	}
}

func TestRemoveRetiredStateReportsFilesystemFailuresAsMessages(t *testing.T) {
	skipIfRoot(t)
	t.Run("state dir not searchable", func(t *testing.T) {
		state := t.TempDir()
		writeRetiredFixture(t, filepath.Join(state, "hook-metrics.jsonl"), "x")
		if err := os.Chmod(state, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(state, 0o700) })
		messages := RemoveRetiredState(state, false)
		joined := strings.Join(messages, "\n")
		if !strings.Contains(joined, "could not be inspected; remove it manually") {
			t.Fatalf("messages = %v", messages)
		}
	})
	t.Run("state dir not writable", func(t *testing.T) {
		state := t.TempDir()
		target := filepath.Join(state, "hook-metrics.jsonl")
		writeRetiredFixture(t, target, "x")
		if err := os.Chmod(state, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(state, 0o700) })
		joined := strings.Join(RemoveRetiredState(state, false), "\n")
		if !strings.Contains(joined, target+" could not be removed; remove it manually") || !exists(target) {
			t.Fatalf("messages = %s", joined)
		}
	})
}
