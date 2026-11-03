package installcli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
	installcontract "issueops/internal/contract/install"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

func writeInstallFixtureFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if mode&0o077 != 0 {
		testsupport.MakePermissive(t, path, mode)
	} else if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestCollectInstallHostPathsCleansPathsAndRejectsKindConflicts(t *testing.T) {
	sep := string(filepath.Separator)
	home := testsupport.AbsPath("h")
	uncleanB := home + sep + "a" + sep + ".." + sep + "b.json"
	cleanB := filepath.Join(home, "b.json")
	skill := filepath.Join(home, "skills", "x")
	paths, err := collectInstallHostPaths(port.NativeInstallResult{
		Files: []port.InstallFile{{Path: uncleanB}, {Path: ""}, {Path: cleanB}},
		Links: []port.InstallLink{{Path: skill}, {Path: ""}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[cleanB] != installHostFile || paths[skill] != installHostLink {
		t.Fatalf("paths = %v", paths)
	}
	conflict := filepath.Join(home, "x")
	_, err = collectInstallHostPaths(port.NativeInstallResult{
		Files: []port.InstallFile{{Path: conflict}},
		Links: []port.InstallLink{{Path: home + sep + "." + sep + "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "both file and symlink: "+conflict) {
		t.Fatalf("conflict err = %v", err)
	}
	if _, err := prepareInstallHostTransaction(port.NativeInstallResult{
		Files: []port.InstallFile{{Path: conflict}}, Links: []port.InstallLink{{Path: conflict}},
	}); err == nil {
		t.Fatal("prepare must surface a path declared as both file and symlink")
	}
}

func TestCaptureInstallHostSnapshotsRejectUnsafeExistingPaths(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	writeInstallFixtureFile(t, regular, "x", 0o640)
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(root, "gone"), dangling); err != nil {
		t.Fatal(err)
	}

	if snapshot, err := captureInstallHostParent(filepath.Join(root, "missing")); err != nil || snapshot.existed {
		t.Fatalf("missing parent = %+v, %v", snapshot, err)
	}
	if _, err := captureInstallHostParent(regular); err == nil || !strings.Contains(err.Error(), "parent is not a directory") {
		t.Fatalf("file parent err = %v", err)
	}
	if _, err := captureInstallHostPath(regular, installHostLink); err == nil || !strings.Contains(err.Error(), "expected symlink path") {
		t.Fatalf("link kind on regular file err = %v", err)
	}
	if _, err := captureInstallHostPath(dir, installHostFile); err == nil || !strings.Contains(err.Error(), "non-file path") {
		t.Fatalf("file kind on directory err = %v", err)
	}
	if _, err := captureInstallHostPath(dangling, installHostFile); err == nil {
		t.Fatal("a dangling symlink cannot be snapshotted as managed file content")
	}
	snapshot, err := captureInstallHostPath(dangling, installHostLink)
	if err != nil || !snapshot.existed || snapshot.linkTarget != filepath.Join(root, "gone") {
		t.Fatalf("dangling link snapshot = %+v, %v", snapshot, err)
	}
	snapshot, err = captureInstallHostPath(regular, installHostFile)
	wantRegularPerm := os.FileMode(0o640)
	if runtime.GOOS == "windows" {
		wantRegularPerm = 0o666 // osfile.Perm renders a non-private Windows file as owner rw mirrored to group and other
	}
	if err != nil || string(snapshot.content) != "x" || snapshot.mode.Perm() != wantRegularPerm || snapshot.wasSymlink {
		t.Fatalf("regular snapshot = %+v, %v", snapshot, err)
	}
	snapshot, err = captureInstallHostPath(filepath.Join(root, "new"), installHostFile)
	if err != nil || snapshot.existed {
		t.Fatalf("absent snapshot = %+v, %v", snapshot, err)
	}
}

func TestInstallHostRollbackRestoresEverythingTheInstallTouched(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "cfg", "settings.json")
	writeInstallFixtureFile(t, existing, "original", 0o600)
	linkOld := filepath.Join(root, "skills", "s")
	if err := os.MkdirAll(filepath.Dir(linkOld), 0o755); err != nil {
		t.Fatal(err)
	}
	oldTarget := testsupport.AbsPath("old", "target")
	if err := os.Symlink(oldTarget, linkOld); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(root, "fresh", "deep", "new.json")
	keeper := filepath.Join(root, "fresh", "keep.txt")
	plan := port.NativeInstallResult{
		Files: []port.InstallFile{{Path: existing}, {Path: created}},
		Links: []port.InstallLink{{Path: linkOld}},
	}
	transaction, err := prepareInstallHostTransaction(plan)
	if err != nil {
		t.Fatal(err)
	}

	// simulate the install: rewrite, replace a directory over the link, create new files
	writeInstallFixtureFile(t, existing, "rewritten", 0o644)
	if err := os.Remove(linkOld); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(linkOld, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInstallFixtureFile(t, created, "{}", 0o600)
	writeInstallFixtureFile(t, keeper, "user data", 0o600)

	if err := transaction.RollbackHosts(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if body, _ := os.ReadFile(existing); string(body) != "original" {
		t.Fatalf("existing file content = %q", body)
	}
	if info, _ := os.Stat(existing); osfile.Perm(existing, info) != 0o600 {
		t.Fatalf("existing file mode = %v, want 0600", osfile.Perm(existing, info))
	}
	if target, err := os.Readlink(linkOld); err != nil || target != oldTarget {
		t.Fatalf("link target = %q, %v", target, err)
	}
	if _, err := os.Lstat(created); !os.IsNotExist(err) {
		t.Fatalf("created file survived rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "fresh", "deep")); !os.IsNotExist(err) {
		t.Fatalf("empty created parent survived rollback: %v", err)
	}
	if body, _ := os.ReadFile(keeper); string(body) != "user data" {
		t.Fatal("a created parent that gained unrelated content must be left alone")
	}
}

func TestInstallHostRollbackRemovesCreatedParentsAndReplacesNonRegularFileWithSnapshot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "cfg.json")
	writeInstallFixtureFile(t, file, "orig", 0o600)
	created := filepath.Join(root, "a", "b", "c.json")
	transaction, err := prepareInstallHostTransaction(port.NativeInstallResult{Files: []port.InstallFile{{Path: file}, {Path: created}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInstallFixtureFile(t, created, "x", 0o600)

	if err := transaction.rollback(); err != nil {
		t.Fatal(err)
	}

	if body, err := os.ReadFile(file); err != nil || string(body) != "orig" {
		t.Fatalf("directory squatting on a managed file was not replaced: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("created parents must be removed deepest first: %v", err)
	}
}

func TestInstallHostRollbackReportsParentThatBecameAFile(t *testing.T) {
	root := t.TempDir()
	created := filepath.Join(root, "p", "new.json")
	transaction, err := prepareInstallHostTransaction(port.NativeInstallResult{Files: []port.InstallFile{{Path: created}}})
	if err != nil {
		t.Fatal(err)
	}
	writeInstallFixtureFile(t, filepath.Join(root, "p"), "not a dir", 0o600)

	err = transaction.rollback()

	if err == nil || !strings.Contains(err.Error(), "changed to non-directory") {
		t.Fatalf("rollback err = %v", err)
	}
	if (*installHostTransaction)(nil).RollbackHosts() != nil {
		t.Fatal("a nil transaction rolls back to nothing")
	}
}

func TestPathDepthCountsComponents(t *testing.T) {
	sep := string(filepath.Separator)
	for path, want := range map[string]int{sep: 0, ".": 0, sep + "a": 1, sep + filepath.Join("a", "b", "c"): 3, filepath.Join("a", "b"): 2} {
		if got := pathDepth(path); got != want {
			t.Errorf("pathDepth(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestRemoveInstallDirectoryIfEmptyOnlyRemovesEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	full := filepath.Join(root, "full")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInstallFixtureFile(t, filepath.Join(full, "f"), "x", 0o600)
	for _, dir := range []string{empty, full, filepath.Join(root, "absent")} {
		if err := removeInstallDirectoryIfEmpty(dir); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatal("empty directory should be removed")
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatal("non-empty directory must stay")
	}
	if err := removeInstallDirectoryIfEmpty(filepath.Join(full, "f")); err == nil {
		t.Fatal("a file where a directory was expected is an error")
	}
}

type managedPathFixture struct {
	applyPlan installcontract.ManagedCommandPathPlan
	applyErr  error
	calls     []string
}

func (f *managedPathFixture) Apply() (installcontract.ManagedCommandPathPlan, error) {
	f.calls = append(f.calls, "apply")
	return f.applyPlan, f.applyErr
}

func (f *managedPathFixture) Rollback() (installcontract.ManagedCommandPathPlan, error) {
	f.calls = append(f.calls, "rollback")
	return installcontract.ManagedCommandPathPlan{Path: "p", RolledBack: true}, nil
}

func (f *managedPathFixture) Finalize() (installcontract.ManagedCommandPathPlan, error) {
	f.calls = append(f.calls, "finalize")
	return installcontract.ManagedCommandPathPlan{Path: "p", Committed: true}, nil
}

func TestInstallPathTransactionManagedCommandLifecycle(t *testing.T) {
	home := t.TempDir()
	command := filepath.Join(home, ".local", "bin", testsupport.ExecutableName("issueops"))
	writeInstallFixtureFile(t, command, "old binary", 0o755)
	target := filepath.Join(t.TempDir(), "bin", "issueops")
	managed := &managedPathFixture{applyPlan: installcontract.ManagedCommandPathPlan{Path: command, Target: target, Adopted: true}}
	cli := testInstallCommand(t, Deps{})
	cli.PrepareManagedCommandPathCandidate = func(gotTarget, candidate, path string, adopt, dry bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
		if gotTarget != target || path != command || !adopt || dry {
			t.Errorf("unexpected adoption request: target=%q path=%q adopt=%v dry=%v", gotTarget, path, adopt, dry)
		}
		return managed, installcontract.ManagedCommandPathPlan{Path: command, Target: target}, nil
	}
	req := port.NativeInstallRequest{Home: home, BinPath: target, AdoptCommandFile: true}
	var result port.NativeInstallResult

	transaction, err := cli.prepareInstallPathPlanFromCandidate(&result, req, target, "skip")
	if err != nil || !transaction.managed {
		t.Fatalf("prepare = %+v, %v", transaction, err)
	}
	if err := transaction.Apply(&result); err != nil {
		t.Fatal(err)
	}
	if result.CommandPath == nil || !result.CommandPath.Adopted || !transaction.applied {
		t.Fatalf("apply result = %+v applied=%v", result.CommandPath, transaction.applied)
	}
	if err := transaction.Rollback(&result); err != nil {
		t.Fatal(err)
	}
	if !result.CommandPath.RolledBack || transaction.applied {
		t.Fatalf("rollback result = %+v applied=%v", result.CommandPath, transaction.applied)
	}
	if err := transaction.Finalize(&result); err != nil || !result.CommandPath.Committed {
		t.Fatalf("finalize result = %+v, %v", result.CommandPath, err)
	}
	if strings.Join(managed.calls, ",") != "apply,rollback,finalize" {
		t.Fatalf("managed calls = %v", managed.calls)
	}
}

func TestInstallPathTransactionManagedApplyFailureKeepsAdoptionStateForRollback(t *testing.T) {
	home := t.TempDir()
	command := filepath.Join(home, ".local", "bin", testsupport.ExecutableName("issueops"))
	writeInstallFixtureFile(t, command, "old", 0o755)
	boom := errors.New("swap failed")
	managed := &managedPathFixture{applyPlan: installcontract.ManagedCommandPathPlan{Path: command, Adopted: true}, applyErr: boom}
	cli := testInstallCommand(t, Deps{})
	cli.PrepareManagedCommandPathCandidate = func(string, string, string, bool, bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
		return managed, installcontract.ManagedCommandPathPlan{Path: command}, nil
	}
	var result port.NativeInstallResult
	transaction, err := cli.prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: "/t/issueops"}, "/t/issueops", "skip")
	if err != nil {
		t.Fatal(err)
	}

	if err := transaction.Apply(&result); !errors.Is(err, boom) {
		t.Fatalf("apply err = %v, want %v", err, boom)
	}
	if !transaction.applied {
		t.Fatal("a half-applied adoption must stay marked applied so rollback runs")
	}
	for _, link := range result.Links {
		if link.Created {
			t.Fatalf("no link may be reported created after a failed adoption: %+v", result.Links)
		}
	}
	if err := transaction.Rollback(&result); err != nil || strings.Join(managed.calls, ",") != "apply,rollback" {
		t.Fatalf("rollback err=%v calls=%v", err, managed.calls)
	}
}

func TestPrepareInstallPathPlanPropagatesManagedPreflightAndStatFailures(t *testing.T) {
	t.Run("managed preflight refusal still reports the plan", func(t *testing.T) {
		home := t.TempDir()
		writeInstallFixtureFile(t, filepath.Join(home, ".local", "bin", testsupport.ExecutableName("issueops")), "old", 0o755)
		refusal := errors.New("refusing to adopt")
		cli := testInstallCommand(t, Deps{})
		cli.PrepareManagedCommandPathCandidate = func(string, string, string, bool, bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
			return nil, installcontract.ManagedCommandPathPlan{WouldAdopt: false, Path: "p"}, refusal
		}
		var result port.NativeInstallResult
		_, err := cli.prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: "/t/issueops", DryRun: true}, "/t/issueops", "skip")
		if !errors.Is(err, refusal) || result.CommandPath == nil || result.CommandPath.Path != "p" {
			t.Fatalf("err=%v commandPath=%+v", err, result.CommandPath)
		}
	})
	t.Run("a file in place of the .local directory fails the stat", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Lstat under a regular file reports path-not-found (not exist) on Windows, not a stat failure")
		}
		home := t.TempDir()
		writeInstallFixtureFile(t, filepath.Join(home, ".local"), "x", 0o644)
		var result port.NativeInstallResult
		_, err := testInstallCommand(t, Deps{}).prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: "/t/issueops", DryRun: true}, "/t/issueops", "skip")
		if err == nil {
			t.Fatal("expected the ENOTDIR stat failure to be returned")
		}
	})
}

func TestInstallPathTransactionApplyReportsSymlinkFailureAndRollsBackRestoredLink(t *testing.T) {
	home := t.TempDir()
	userBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(userBin, testsupport.ExecutableName("issueops"))
	if err := os.Symlink(previousCommandTarget, command); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "issueops")
	cli := testInstallCommand(t, Deps{})
	var result port.NativeInstallResult
	transaction, err := cli.prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: target}, target, "skip")
	if err != nil {
		t.Fatal(err)
	}
	// a stray regular file named io makes the short shim step refuse after the command link moved
	writeInstallFixtureFile(t, filepath.Join(userBin, testsupport.ExecutableName("io")), "not ours", 0o755)

	if err := transaction.Apply(&result); err == nil || !strings.Contains(err.Error(), "refusing to replace existing io command") {
		t.Fatalf("apply err = %v", err)
	}
	if got, _ := os.Readlink(command); got != target {
		t.Fatalf("command link = %q, want the new target after apply", got)
	}
	if err := transaction.Rollback(&result); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(command); got != previousCommandTarget {
		t.Fatalf("rollback must restore the previous link target, got %q", got)
	}
	if body, _ := os.ReadFile(filepath.Join(userBin, testsupport.ExecutableName("io"))); string(body) != "not ours" {
		t.Fatal("rollback must not touch a file it never created")
	}
}

func TestInstallPathTransactionFinalizeWithoutManagedCommandIsNoop(t *testing.T) {
	var result port.NativeInstallResult
	if err := (&installPathTransaction{}).Finalize(&result); err != nil || result.CommandPath != nil {
		t.Fatalf("finalize = %v, commandPath=%+v", err, result.CommandPath)
	}
}

func TestPrepareInstallPathPlanPlansRelinkOfExistingCommandSymlink(t *testing.T) {
	home := t.TempDir()
	command := filepath.Join(home, ".local", "bin", testsupport.ExecutableName("issueops"))
	if err := os.MkdirAll(filepath.Dir(command), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(previousCommandTarget, command); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "issueops")
	var result port.NativeInstallResult
	transaction, err := testInstallCommand(t, Deps{}).prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: target, DryRun: true}, target, "skip")
	if err != nil || transaction.managed {
		t.Fatalf("prepare = %+v, %v", transaction, err)
	}
	planned := false
	for _, link := range result.Links {
		if link.Path == command && link.Target == target && link.WouldCreate {
			planned = true
		}
	}
	if !planned {
		t.Fatalf("dry-run must plan the command symlink to the new target: %+v", result.Links)
	}
}

func TestInstallPathTransactionRollbackReportsFailureToRestorePreviousLink(t *testing.T) {
	home := t.TempDir()
	userBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(userBin, testsupport.ExecutableName("issueops"))
	if err := os.Symlink(previousCommandTarget, command); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "issueops")
	var result port.NativeInstallResult
	transaction, err := testInstallCommand(t, Deps{}).prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: target}, target, "skip")
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Apply(&result); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(userBin); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Rollback(&result); err == nil {
		t.Fatal("rollback must report that the previous symlink could not be restored")
	}
}

func TestInstallPathTransactionRollbackReportsCreatedDirectoryReplacedByFile(t *testing.T) {
	home := t.TempDir()
	boom := errors.New("symlink denied")
	cli := testInstallCommand(t, Deps{})
	cli.EnsureSymlinkPlan = func(target, path string, dryRun bool) (port.InstallLink, error) {
		link := port.InstallLink{Path: path, Target: target}
		if dryRun {
			return link, nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		return link, boom
	}
	var result port.NativeInstallResult
	transaction, err := cli.prepareInstallPathPlanFromCandidate(&result, port.NativeInstallRequest{Home: home, BinPath: "/t/issueops"}, "/t/issueops", "skip")
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Apply(&result); !errors.Is(err, boom) {
		t.Fatalf("apply err = %v, want %v", err, boom)
	}
	bin := filepath.Join(home, ".local", "bin")
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	writeInstallFixtureFile(t, bin, "not a directory", 0o644)
	err = transaction.Rollback(&result)
	if err == nil || !strings.Contains(err.Error(), "changed to non-directory") {
		t.Fatalf("rollback err = %v", err)
	}
}
