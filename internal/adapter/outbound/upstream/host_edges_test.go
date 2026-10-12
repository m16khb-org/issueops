package upstream

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	upstreamcontract "issueops/internal/contract/upstream"
	"issueops/internal/testsupport"
)

// fakeClaude writes an executable sh script and returns its path.
func fakeClaude(t *testing.T, script string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"+script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binary, 0o700); err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestClaudePluginHostBinaryDefaultsToClaudeWhenBlank(t *testing.T) {
	if got := (ClaudePluginHost{Binary: "  "}).binary(); got != "claude" {
		t.Fatalf("blank binary = %q, want claude", got)
	}
	if got := (ClaudePluginHost{Binary: "/opt/x"}).binary(); got != "/opt/x" {
		t.Fatalf("explicit binary = %q", got)
	}
}

func TestClaudePluginHostAvailableReflectsBinaryPresence(t *testing.T) {
	present := ClaudePluginHost{Binary: fakeClaude(t, "exit 0\n")}
	if !present.Available() {
		t.Fatalf("executable binary must be available")
	}
	absent := ClaudePluginHost{Binary: filepath.Join(t.TempDir(), "missing")}
	if absent.Available() {
		t.Fatalf("missing binary must not be available")
	}
}

func TestClaudePluginHostInstalledPluginsParsesListOutput(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, `
[ "$*" = "plugin list --json" ] || { echo "bad args: $*" >&2; exit 64; }
echo '[{"id":"a@m"},{"id":" "},{"id":"b@m"}]'
`)}
	ids, err := host.InstalledPlugins(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a@m" || ids[1] != "b@m" {
		t.Fatalf("ids = %#v", ids)
	}
}

func TestClaudePluginHostMarketplacesParsesListOutput(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, `
[ "$*" = "plugin marketplace list --json" ] || { echo "bad args: $*" >&2; exit 64; }
echo '[{"name":"official"},{"name":"community"}]'
`)}
	names, err := host.Marketplaces(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "official" || names[1] != "community" {
		t.Fatalf("names = %#v", names)
	}
}

func TestClaudePluginHostAddMarketplacePassesSource(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, `
[ "$*" = "plugin marketplace add owner/repo" ] || { echo "bad args: $*" >&2; exit 64; }
`)}
	if err := host.AddMarketplace(t.Context(), "owner/repo"); err != nil {
		t.Fatal(err)
	}
}

func TestClaudePluginHostListsRejectMalformedJSON(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, "echo 'not json'\n")}
	if _, err := host.InstalledPlugins(t.Context()); err == nil || !strings.Contains(err.Error(), "parse installed plugin list") {
		t.Fatalf("InstalledPlugins error = %v", err)
	}
	if _, err := host.Marketplaces(t.Context()); err == nil || !strings.Contains(err.Error(), "parse marketplace list") {
		t.Fatalf("Marketplaces error = %v", err)
	}
}

func TestClaudePluginHostRunSurfacesStderrOfFailingCommand(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, "echo '  boom detail  ' >&2\nexit 3\n")}
	for name, err := range map[string]error{
		"list":        func() error { _, e := host.InstalledPlugins(t.Context()); return e }(),
		"marketplace": func() error { _, e := host.Marketplaces(t.Context()); return e }(),
		"add":         host.AddMarketplace(t.Context(), "src"),
		"install":     host.InstallPlugin(t.Context(), "x@y"),
	} {
		if err == nil {
			t.Fatalf("%s: want error", name)
		}
		if !strings.Contains(err.Error(), ": boom detail") || !strings.Contains(err.Error(), "exit status 3") {
			t.Fatalf("%s: error must carry trimmed stderr and exit status: %v", name, err)
		}
	}
}

func TestClaudePluginHostRunReportsCommandWhenBinaryCannotStart(t *testing.T) {
	host := ClaudePluginHost{Binary: filepath.Join(t.TempDir(), "missing")}
	err := host.InstallPlugin(t.Context(), "x@y")
	if err == nil {
		t.Fatal("want error for missing binary")
	}
	if _, isExit := errors.AsType[*exec.ExitError](err); isExit {
		t.Fatalf("a binary that never started must not look like an exit error: %v", err)
	}
	if !strings.Contains(err.Error(), "plugin install x@y --scope user") {
		t.Fatalf("error must name the attempted command: %v", err)
	}
}

func TestClaudePluginHostRunOmitsStderrSuffixWhenCommandIsSilent(t *testing.T) {
	host := ClaudePluginHost{Binary: fakeClaude(t, "exit 2\n")}
	err := host.AddMarketplace(t.Context(), "src")
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.HasSuffix(err.Error(), "exit status 2") {
		t.Fatalf("silent failure must end at the exit status: %v", err)
	}
}

func TestReadConfigRejectsMalformedDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstream.json")
	writeFile(t, path, "{not json")
	_, err := ReadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "parse upstream declaration "+path) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadConfigPropagatesNonMissingReadErrors(t *testing.T) {
	// A directory at the declaration path fails the read without being ErrNotExist.
	_, err := ReadConfig(t.TempDir())
	if err == nil {
		t.Fatal("unreadable declaration must be an error, not an empty config")
	}
	if strings.Contains(err.Error(), "parse upstream declaration") {
		t.Fatalf("read failure must not be reported as a parse failure: %v", err)
	}
}

func TestInstalledSkillsPropagatesNonMissingReadErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "skills")
	writeFile(t, file, "not a directory")
	if _, err := (GitSkillStore{SkillsDir: file, CacheDir: t.TempDir()}).InstalledSkills(); err == nil {
		t.Fatal("a skills path that is a file must be an error")
	}
}

func TestInstallSkillRequiresConfiguredDirectories(t *testing.T) {
	entry := upstreamcontract.SkillEntry{Name: "demo", Repo: "unused"}
	for name, store := range map[string]GitSkillStore{
		"no skills dir": {CacheDir: t.TempDir()},
		"no cache dir":  {SkillsDir: t.TempDir()},
		"blank both":    {SkillsDir: " ", CacheDir: "\t"},
	} {
		err := store.InstallSkill(t.Context(), entry)
		if err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Fatalf("%s: error = %v", name, err)
		}
	}
}

func TestInstallSkillRefusesToReplaceARealDirectory(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "demo", "---\nname: demo\n---\n")
	skillsDir := t.TempDir()
	mine := filepath.Join(skillsDir, "demo")
	writeFile(t, filepath.Join(mine, "keep.txt"), "user data")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: filepath.Join(t.TempDir(), "cache")}

	err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: repo, Path: "demo"})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace non-symlink path") {
		t.Fatalf("error = %v", err)
	}
	if body, readErr := os.ReadFile(filepath.Join(mine, "keep.txt")); readErr != nil || string(body) != "user data" {
		t.Fatalf("existing real directory must be untouched: %v", readErr)
	}
}

func TestInstallSkillReplacesAnExistingSymlinkAndRefreshesCache(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "demo", "---\nname: demo\n---\nfresh\n")
	skillsDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	stale := t.TempDir()
	symlink(t, stale, filepath.Join(skillsDir, "demo"))
	writeFile(t, filepath.Join(cacheDir, "demo", "stale.txt"), "old cache")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: cacheDir}

	if err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: repo, Path: "demo"}); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(skillsDir, "demo"))
	if err != nil || target != filepath.Join(cacheDir, "demo") {
		t.Fatalf("link target = %q (%v), want the cache copy", target, err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "demo", "stale.txt")); err == nil {
		t.Fatal("previous cache copy must be replaced, not merged")
	}
	if body, err := os.ReadFile(filepath.Join(cacheDir, "demo", "SKILL.md")); err != nil || !strings.Contains(string(body), "fresh") {
		t.Fatalf("cache must hold the fetched skill: %v", err)
	}
}

func TestInstallSkillWithoutPathInstallsRepoRootWithoutGitMetadata(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, ".", "---\nname: root\n---\n")
	skillsDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: cacheDir}

	if err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "root", Repo: repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "root", "SKILL.md")); err != nil {
		t.Fatalf("root skill must be reachable through the link: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "root", ".git")); err == nil {
		t.Fatal(".git must be stripped from the cache copy")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, ".fetch-root")); err == nil {
		t.Fatal("scratch fetch directory must be removed")
	}
}

func TestInstallSkillHonorsRefAndRejectsUnknownRef(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "demo", "---\nname: demo\n---\n")
	gitIn(t, repo, "checkout", "--quiet", "-b", "release")
	writeFile(t, filepath.Join(repo, "demo", "SKILL.md"), "---\nname: demo\n---\nrelease-branch\n")
	gitIn(t, repo, "commit", "--quiet", "-am", "release change")
	gitIn(t, repo, "checkout", "--quiet", "main")

	skillsDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: cacheDir}

	if err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: repo, Path: "demo", Ref: "release"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(skillsDir, "demo", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "release-branch") {
		t.Fatalf("Ref must select the release branch content: %v %q", err, body)
	}

	err = store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "other", Repo: repo, Path: "demo", Ref: "no-such-ref"})
	if err == nil || !strings.Contains(err.Error(), "--branch no-such-ref") {
		t.Fatalf("unknown ref error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(skillsDir, "other")); statErr == nil {
		t.Fatal("failed fetch must not leave a host link")
	}
}

func TestInstallSkillReportsGitFailureForUnreachableRepo(t *testing.T) {
	requireGit(t)
	testsupport.IsolateGitConfig(t)
	missing := filepath.Join(t.TempDir(), "no-repo")
	store := GitSkillStore{SkillsDir: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache")}

	err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: missing})
	if err == nil || !strings.HasPrefix(err.Error(), "git clone ") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallSkillStopsWhenContextIsCancelled(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "demo", "---\nname: demo\n---\n")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	skillsDir := t.TempDir()
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: filepath.Join(t.TempDir(), "cache")}

	if err := store.InstallSkill(ctx, upstreamcontract.SkillEntry{Name: "demo", Repo: repo, Path: "demo"}); err == nil {
		t.Fatal("cancelled context must fail the fetch")
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "demo")); err == nil {
		t.Fatal("cancelled install must not link")
	}
}

func TestInstallSkillFailsWhenCacheDirIsAFile(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	writeFile(t, cache, "file")
	store := GitSkillStore{SkillsDir: t.TempDir(), CacheDir: cache}
	if err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: "unused"}); err == nil {
		t.Fatal("cache directory that is a file must fail before any fetch")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestInstallSkillClonesWholeRepoWhenPathIsEmpty(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, ".", "---\nname: demo\n---\n")
	writeFile(t, filepath.Join(repo, "references", "guide.md"), "guide")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "--quiet", "-m", "references"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cacheDir := filepath.Join(t.TempDir(), "cache")
	store := GitSkillStore{SkillsDir: t.TempDir(), CacheDir: cacheDir}

	if err := store.InstallSkill(t.Context(), upstreamcontract.SkillEntry{Name: "demo", Repo: repo}); err != nil {
		t.Fatalf("InstallSkill with empty Path: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(cacheDir, "demo", "references", "guide.md")); err != nil || string(body) != "guide" {
		t.Fatalf("whole-repo checkout must include nested files: %q, %v", body, err)
	}
}
