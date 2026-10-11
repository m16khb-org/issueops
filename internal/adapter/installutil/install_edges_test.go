package installutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellCommandWordParsesQuotingAndEscapes(t *testing.T) {
	tests := []struct {
		in       string
		word     string
		rest     string
		wantOkay bool
	}{
		{"issueops hook x", "issueops", " hook x", true},
		{"'/a b/issueops' hook", "/a b/issueops", " hook", true},
		{`"/a b/issueops"	hook`, "/a b/issueops", "\thook", true},
		{`/a\ b/issueops hook`, "/a b/issueops", " hook", true},
		{`pre'fix'"post" rest`, "prefixpost", " rest", true},
		{"issueops", "issueops", "", true},
		{"", "", "", false},
		{" leading", "", "", false},
		{`trailing\`, "", "", false},
		{"'unterminated hook", "", "", false},
		{`"unterminated`, "", "", false},
		{"''", "", "", false},
	}
	for _, tt := range tests {
		word, rest, ok := shellCommandWord(tt.in)
		if word != tt.word || rest != tt.rest || ok != tt.wantOkay {
			t.Fatalf("shellCommandWord(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.in, word, rest, ok, tt.word, tt.rest, tt.wantOkay)
		}
	}
}

func TestHookCommandTargetRequiresHookSubcommand(t *testing.T) {
	if target, ok := hookCommandTarget("  '/x/issueops'   hook session-start"); !ok || target != "/x/issueops" {
		t.Fatalf("target=%q ok=%v", target, ok)
	}
	for _, command := range []string{"/x/issueops", "/x/issueops run", "/x/issueops 'unterminated", "", "/x/issueops hooks"} {
		if target, ok := hookCommandTarget(command); ok {
			t.Fatalf("hookCommandTarget(%q) = %q, want no target", command, target)
		}
	}
}

func TestPruneStaleSkillLinksHandlesRelativeTargetsAndDryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "alive"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "hosts", "codex")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// Relative links resolve against destRoot: one points at a live harness skill, one at a removed one.
	if err := os.Symlink("../../skills/alive", filepath.Join(dest, "alive")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../skills/gone", filepath.Join(dest, "gone")); err != nil {
		t.Fatal(err)
	}
	links, errs := PruneStaleSkillLinks(root, dest, true)
	if len(errs) != 0 || len(links) != 1 || links[0].Path != filepath.Join(dest, "gone") || !links[0].WouldRemove || links[0].Removed {
		t.Fatalf("dry run links=%+v errs=%v", links, errs)
	}
	if _, err := os.Lstat(filepath.Join(dest, "gone")); err != nil {
		t.Fatalf("dry run removed the link: %v", err)
	}
	links, errs = PruneStaleSkillLinks(root, dest, false)
	if len(errs) != 0 || len(links) != 1 || !links[0].Removed || links[0].WouldRemove {
		t.Fatalf("links=%+v errs=%v", links, errs)
	}
	if _, err := os.Lstat(filepath.Join(dest, "gone")); !os.IsNotExist(err) {
		t.Fatalf("stale link survived: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "alive")); err != nil {
		t.Fatalf("live link was removed: %v", err)
	}
}

func TestPruneStaleSkillLinksReportsUnreadableDestRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	links, errs := PruneStaleSkillLinks(t.TempDir(), file, false)
	if len(links) != 0 || len(errs) != 1 {
		t.Fatalf("links=%+v errs=%v", links, errs)
	}
}

func TestPruneStaleSkillLinksReportsRemovalFailure(t *testing.T) {
	skipIfRoot(t)
	root := t.TempDir()
	dest := filepath.Join(root, "dest")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "skills", "gone"), filepath.Join(dest, "gone")); err != nil {
		t.Fatal(err)
	}
	lockDir(t, dest)
	links, errs := PruneStaleSkillLinks(root, dest, false)
	if len(links) != 0 || len(errs) != 1 {
		t.Fatalf("links=%+v errs=%v", links, errs)
	}
}

func TestEnsureSymlinkPlanDryRunReportsRetargetWithoutTouchingLink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "link")
	if err := os.Symlink("/old", path); err != nil {
		t.Fatal(err)
	}
	link, err := EnsureSymlinkPlan("/new", path, true)
	if err != nil || !link.WouldCreate || link.Created {
		t.Fatalf("link=%+v err=%v", link, err)
	}
	if actual, _ := os.Readlink(path); actual != "/old" {
		t.Fatalf("dry run changed link to %q", actual)
	}
}

func TestEnsureSymlinkPlanReportsStatFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// Lstat under a regular file fails with ENOTDIR, which is not "not exist".
	if _, err := EnsureSymlinkPlan("/t", filepath.Join(blocker, "link"), false); err == nil {
		t.Fatal("ENOTDIR lstat error was swallowed")
	}
}

func TestEnsureSymlinkPlanReportsDirectoryCreationFailure(t *testing.T) {
	skipIfRoot(t)
	parent := t.TempDir()
	lockDir(t, parent)
	link, err := EnsureSymlinkPlan("/t", filepath.Join(parent, "sub", "link"), false)
	if err == nil || link.Created {
		t.Fatalf("link=%+v err=%v", link, err)
	}
}

func TestWriteTextPlanReportsReadFailureUnlessDryRun(t *testing.T) {
	dir := t.TempDir() // reading a directory as a file fails with a non-ENOENT error
	if _, err := WriteTextPlan(dir, "config", "x", 0o600, false); err == nil {
		t.Fatal("read failure was swallowed")
	}
	file, err := WriteTextPlan(dir, "config", "x", 0o600, true)
	if err != nil || !file.WouldWrite || file.Written {
		t.Fatalf("dry run file=%+v err=%v", file, err)
	}
}

func TestWriteTextPlanReportsDirectoryCreationFailure(t *testing.T) {
	skipIfRoot(t)
	parent := t.TempDir()
	lockDir(t, parent)
	file, err := WriteTextPlan(filepath.Join(parent, "sub", "config"), "config", "x", 0o600, false)
	if err == nil || file.Written {
		t.Fatalf("file=%+v err=%v", file, err)
	}
}

func TestPlanSkillLinksCollectsPerSkillErrorsAndContinues(t *testing.T) {
	root, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "blocked"), []byte("regular"), 0o600); err != nil {
		t.Fatal(err)
	}
	links, errs := PlanSkillLinks(root, dest, []string{"blocked", "fresh"}, false)
	if len(links) != 2 || len(errs) != 1 || !strings.Contains(errs[0].Error(), "refusing to replace non-symlink path") {
		t.Fatalf("links=%+v errs=%v", links, errs)
	}
	if links[0].Created || !links[1].Created {
		t.Fatalf("links=%+v", links)
	}
}

func TestTOMLStringQuotesAndEscapes(t *testing.T) {
	if got := TOMLString("a\"b\\c\n"); got != `"a\"b\\c\n"` {
		t.Fatalf("TOMLString = %s", got)
	}
}

func TestCaptureNativeActivationEvidenceResolvesRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "h.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	evidence, err := CaptureNativeActivationEvidence("codex", "hooks", "./h.json", "")
	if err != nil || !filepath.IsAbs(evidence.Path) || filepath.Base(evidence.Path) != "h.json" {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
}
