package installutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type commandFixture struct {
	root, target, command string
	deps                  managedCommandPathDeps
}

func newCommandFixture(t *testing.T) commandFixture {
	t.Helper()
	root := t.TempDir()
	deps := validCommandDeps()
	deps.randomSuffix = func() (string, error) { return "fixed", nil }
	return commandFixture{
		root:    root,
		target:  writeCommandFixture(t, root, "target", "new", 0o755),
		command: writeCommandFixture(t, root, "issueops", "old", 0o751),
		deps:    deps,
	}
}

func (f commandFixture) prepare(t *testing.T) *ManagedCommandPathTransaction {
	t.Helper()
	transaction, _, err := prepareManagedCommandPathCandidateWithDeps(f.target, f.target, f.command, true, false, f.deps)
	if err != nil {
		t.Fatal(err)
	}
	return transaction
}

func (f commandFixture) apply(t *testing.T) *ManagedCommandPathTransaction {
	t.Helper()
	transaction := f.prepare(t)
	if _, err := transaction.Apply(); err != nil {
		t.Fatal(err)
	}
	return transaction
}

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("directory permission failures are not enforced for root")
	}
}

func lockDir(t *testing.T, dir string) {
	t.Helper()
	mustChmod(t, dir, 0o500)
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

func TestPrepareManagedCommandPathRejectsUnusableDependenciesAndInputs(t *testing.T) {
	f := newCommandFixture(t)
	missingDeps := f.deps
	missingDeps.exchangePaths = nil
	if _, _, err := prepareManagedCommandPathCandidateWithDeps(f.target, f.target, f.command, true, false, missingDeps); err == nil || !strings.Contains(err.Error(), "dependencies are unavailable") {
		t.Fatalf("missing deps err = %v", err)
	}
	for name, suffix := range map[string]func() (string, error){
		"error": func() (string, error) { return "", errors.New("entropy") },
		"empty": func() (string, error) { return "", nil },
	} {
		deps := f.deps
		deps.randomSuffix = suffix
		if _, _, err := prepareManagedCommandPathCandidateWithDeps(f.target, f.target, f.command, true, false, deps); err == nil || !strings.Contains(err.Error(), "allocate private command backup path") {
			t.Fatalf("%s suffix err = %v", name, err)
		}
	}
	if _, _, err := prepareManagedCommandPathCandidateWithDeps(filepath.Join(f.root, "absent"), filepath.Join(f.root, "absent"), f.command, true, false, f.deps); err == nil || !strings.Contains(err.Error(), "inspect command candidate") {
		t.Fatalf("missing candidate err = %v", err)
	}
	if _, _, err := prepareManagedCommandPathCandidateWithDeps(f.target, f.target, filepath.Join(f.root, "absent"), true, false, f.deps); err == nil || !strings.Contains(err.Error(), "inspect existing command") {
		t.Fatalf("missing existing err = %v", err)
	}
}

func TestInspectManagedCommandRejectsBuildIdentityMismatch(t *testing.T) {
	f := newCommandFixture(t)
	for name, build := range map[string]func(string) (managedBuildInfo, error){
		"read error": func(string) (managedBuildInfo, error) { return managedBuildInfo{}, errors.New("not a go binary") },
		"wrong main": func(string) (managedBuildInfo, error) {
			return managedBuildInfo{MainPath: "other/cmd", ModulePath: managedCommandModulePath}, nil
		},
		"wrong module": func(string) (managedBuildInfo, error) {
			return managedBuildInfo{MainPath: managedCommandMainPath, ModulePath: "other"}, nil
		},
	} {
		if _, err := inspectManagedCommand(f.command, build); err == nil || !strings.Contains(err.Error(), "managed issueops build identity") {
			t.Fatalf("%s err = %v", name, err)
		}
	}
	identity, err := inspectManagedCommand(f.command, f.deps.readBuildInfo)
	if err != nil || identity.size != 3 || identity.links != 1 || identity.sha256 == "" {
		t.Fatalf("identity=%+v err=%v", identity, err)
	}
}

func TestInspectManagedCommandDetectsMutationDuringBuildInspection(t *testing.T) {
	f := newCommandFixture(t)
	_, err := inspectManagedCommand(f.command, func(path string) (managedBuildInfo, error) {
		if err := os.Chmod(path, 0o700); err != nil {
			return managedBuildInfo{}, err
		}
		return managedBuildInfo{MainPath: managedCommandMainPath, ModulePath: managedCommandModulePath}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "changed during build inspection") {
		t.Fatalf("err = %v", err)
	}
}

func TestManagedCommandPathTransactionStateGuards(t *testing.T) {
	var nilTransaction *ManagedCommandPathTransaction
	if _, err := nilTransaction.Apply(); err == nil {
		t.Fatal("nil Apply succeeded")
	}
	if _, err := nilTransaction.Rollback(); err == nil {
		t.Fatal("nil Rollback succeeded")
	}
	if _, err := nilTransaction.Finalize(); err == nil {
		t.Fatal("nil Finalize succeeded")
	}
	f := newCommandFixture(t)
	transaction := f.prepare(t)
	if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "cannot roll back") {
		t.Fatalf("rollback before apply err = %v", err)
	}
	if _, err := transaction.Finalize(); err == nil || !strings.Contains(err.Error(), "cannot finalize") {
		t.Fatalf("finalize before apply err = %v", err)
	}
	if _, err := transaction.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Apply(); err == nil || !strings.Contains(err.Error(), "not applicable") {
		t.Fatalf("second apply err = %v", err)
	}
	if _, err := transaction.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Rollback(); err == nil {
		t.Fatal("rollback after finalize succeeded")
	}
	if _, err := transaction.Finalize(); err == nil {
		t.Fatal("second finalize succeeded")
	}
}

func TestManagedCommandPathApplyFailuresLeaveOriginalCommandAndNoBackup(t *testing.T) {
	t.Run("backup directory sync fails", func(t *testing.T) {
		f := newCommandFixture(t)
		f.deps.syncDir = func(string) error { return errors.New("sync boom") }
		plan, err := f.prepare(t).Apply()
		if err == nil || !strings.Contains(err.Error(), "sync private command backup directory") || plan.BackupRetained || plan.Adopted {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		assertRegularCommand(t, f.command, "old", 0o751)
		if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
			t.Fatalf("backup survived: %v", err)
		}
	})
	t.Run("temporary symlink path occupied", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.prepare(t)
		occupied := f.command + ".activate-.issueops.command-backup-fixed"
		if err := os.WriteFile(occupied, []byte("squatter"), 0o600); err != nil {
			t.Fatal(err)
		}
		plan, err := transaction.Apply()
		if err == nil || !strings.Contains(err.Error(), "create temporary command symlink") || plan.BackupRetained {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		assertRegularCommand(t, f.command, "old", 0o751)
		if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
			t.Fatalf("backup survived: %v", err)
		}
	})
	t.Run("exchange fails", func(t *testing.T) {
		f := newCommandFixture(t)
		f.deps.exchangePaths = func(string, string) error { return errors.New("exchange boom") }
		plan, err := f.prepare(t).Apply()
		if err == nil || !strings.Contains(err.Error(), "atomically exchange managed command path") || plan.BackupRetained || plan.Adopted {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		assertRegularCommand(t, f.command, "old", 0o751)
		for _, leftover := range []string{plan.BackupPath, f.command + ".activate-.issueops.command-backup-fixed"} {
			if _, err := os.Lstat(leftover); !os.IsNotExist(err) {
				t.Fatalf("leftover %s: %v", leftover, err)
			}
		}
	})
}

func TestManagedCommandPathApplyReportsRestoreFailuresAfterConcurrentReplacement(t *testing.T) {
	// The first exchange swaps in a concurrently replaced file; later exchanges are driven per case.
	tests := []struct {
		name  string
		hook  func(f commandFixture, concurrent string, real func(string, string) error) func(string, string) error
		wants []string
	}{
		{
			name: "restore exchange fails",
			hook: func(f commandFixture, concurrent string, real func(string, string) error) func(string, string) error {
				calls := 0
				return func(left, right string) error {
					calls++
					if calls == 1 {
						if err := os.Rename(concurrent, right); err != nil {
							return err
						}
						return real(left, right)
					}
					return errors.New("restore boom")
				}
			},
			wants: []string{"identity changed during replacement", "restore concurrently replaced command", "restore boom"},
		},
		{
			name: "path no longer the managed symlink",
			hook: func(f commandFixture, concurrent string, real func(string, string) error) func(string, string) error {
				return func(left, right string) error {
					if err := os.Rename(concurrent, right); err != nil {
						return err
					}
					if err := real(left, right); err != nil {
						return err
					}
					if err := os.Remove(right); err != nil {
						return err
					}
					return os.WriteFile(right, []byte("third"), 0o755)
				}
			},
			wants: []string{"identity changed during replacement", "recovery retained displaced path"},
		},
		{
			name: "restore swap does not return the symlink",
			hook: func(f commandFixture, concurrent string, real func(string, string) error) func(string, string) error {
				calls := 0
				return func(left, right string) error {
					calls++
					if calls == 1 {
						if err := os.Rename(concurrent, right); err != nil {
							return err
						}
						return real(left, right)
					}
					return nil // pretend success without swapping
				}
			},
			wants: []string{"identity changed during replacement", "changed during restore"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCommandFixture(t)
			concurrent := writeCommandFixture(t, f.root, "concurrent", "concurrent", 0o755)
			f.deps.exchangePaths = tt.hook(f, concurrent, f.deps.exchangePaths)
			_, err := f.prepare(t).Apply()
			if err == nil {
				t.Fatal("apply succeeded")
			}
			for _, want := range tt.wants {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v, want %q", err, want)
				}
			}
		})
	}
}

func TestManagedCommandPathApplyKeepsDisplacedCommandWhenCleanupFailsAndRollbackRemovesIt(t *testing.T) {
	skipIfRoot(t)
	f := newCommandFixture(t)
	real := f.deps.exchangePaths
	calls := 0
	f.deps.exchangePaths = func(left, right string) error {
		calls++
		if err := real(left, right); err != nil {
			return err
		}
		if calls == 1 {
			lockDir(t, f.root) // the displaced file can no longer be unlinked
		}
		return nil
	}
	transaction := f.prepare(t)
	plan, err := transaction.Apply()
	if err == nil || !strings.Contains(err.Error(), "remove displaced managed command") || !plan.Adopted {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	mustChmod(t, f.root, 0o700)
	displaced := f.command + ".activate-.issueops.command-backup-fixed"
	assertRegularCommand(t, displaced, "old", 0o751)

	rolled, err := transaction.Rollback()
	if err != nil || !rolled.RolledBack || rolled.BackupRetained || rolled.Adopted {
		t.Fatalf("rollback plan=%+v err=%v", rolled, err)
	}
	assertRegularCommand(t, f.command, "old", 0o751)
	for _, gone := range []string{displaced, plan.BackupPath} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived rollback: %v", gone, err)
		}
	}
}

func TestManagedCommandPathApplyDetectsSymlinkTamperedRightAfterReplacement(t *testing.T) {
	f := newCommandFixture(t)
	real := f.deps.exchangePaths
	f.deps.exchangePaths = func(left, right string) error {
		if err := real(left, right); err != nil {
			return err
		}
		// Point the managed path at a different target after the swap.
		if err := os.Remove(right); err != nil {
			return err
		}
		return os.Symlink(filepath.Join(f.root, "elsewhere"), right)
	}
	plan, err := f.prepare(t).Apply()
	if err == nil || !strings.Contains(err.Error(), "changed immediately after replacement") || plan.Adopted {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if _, statErr := os.Lstat(plan.BackupPath); statErr != nil {
		t.Fatalf("recovery backup must be retained: %v", statErr)
	}
}

func TestManagedCommandPathRollbackRemovesBackupAndClosesTransaction(t *testing.T) {
	f := newCommandFixture(t)
	transaction := f.apply(t)
	plan, err := transaction.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup survived: %v", err)
	}
	if _, err := transaction.Rollback(); err == nil {
		t.Fatal("closed transaction accepted a second rollback")
	}
}

func TestManagedCommandPathRollbackRefusesAndKeepsStateOnPreconditionFailures(t *testing.T) {
	t.Run("managed symlink replaced", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.Remove(f.command); err != nil {
			t.Fatal(err)
		}
		writeCommandFixture(t, f.root, "issueops", "foreign", 0o755)
		if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "changed before rollback") {
			t.Fatalf("err = %v", err)
		}
		assertRegularCommand(t, f.command, "foreign", 0o755)
	})
	t.Run("temp path allocation fails", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.deps.randomSuffix = func() (string, error) { return "", errors.New("entropy") }
		if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "allocate command rollback path") {
			t.Fatalf("err = %v", err)
		}
		assertSymlinkTarget(t, f.command, f.target)
	})
	t.Run("backup missing", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.Remove(transaction.plan.BackupPath); err != nil {
			t.Fatal(err)
		}
		if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "prepare command rollback") {
			t.Fatalf("err = %v", err)
		}
		assertSymlinkTarget(t, f.command, f.target)
	})
	t.Run("backup bytes tampered", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.WriteFile(transaction.plan.BackupPath, []byte("bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "verify command rollback") {
			t.Fatalf("err = %v", err)
		}
		assertSymlinkTarget(t, f.command, f.target)
		if _, statErr := os.Lstat(filepath.Join(f.root, ".issueops.command-rollback-fixed")); !os.IsNotExist(statErr) {
			t.Fatalf("rollback temporary survived: %v", statErr)
		}
	})
	t.Run("exchange fails", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.deps.exchangePaths = func(string, string) error { return errors.New("exchange boom") }
		if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "exchange original command during rollback") {
			t.Fatalf("err = %v", err)
		}
		assertSymlinkTarget(t, f.command, f.target)
		if _, statErr := os.Lstat(filepath.Join(f.root, ".issueops.command-rollback-fixed")); !os.IsNotExist(statErr) {
			t.Fatalf("rollback temporary survived: %v", statErr)
		}
	})
}

func TestManagedCommandPathRollbackReportsRecoveryFailures(t *testing.T) {
	t.Run("concurrent replacement and restore exchange fails", func(t *testing.T) {
		f := newCommandFixture(t)
		concurrent := writeCommandFixture(t, f.root, "concurrent", "concurrent", 0o755)
		transaction := f.apply(t)
		real := transaction.deps.exchangePaths
		calls := 0
		transaction.deps.exchangePaths = func(left, right string) error {
			calls++
			switch calls {
			case 1:
				if err := os.Rename(concurrent, right); err != nil {
					return err
				}
				return real(left, right)
			default:
				return errors.New("restore boom")
			}
		}
		_, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "changed during rollback") || !strings.Contains(err.Error(), "restore boom") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("restored command mutated after exchange", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		real := transaction.deps.exchangePaths
		transaction.deps.exchangePaths = func(left, right string) error {
			if err := real(left, right); err != nil {
				return err
			}
			return os.WriteFile(right, []byte("bad"), 0o751)
		}
		_, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "verify restored command after rollback") || !strings.Contains(err.Error(), "rollback recovery retained displaced path") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestManagedCommandPathRollbackSurfacesCleanupFailures(t *testing.T) {
	t.Run("displaced symlink cannot be removed", func(t *testing.T) {
		skipIfRoot(t)
		f := newCommandFixture(t)
		transaction := f.apply(t)
		real := transaction.deps.exchangePaths
		transaction.deps.exchangePaths = func(left, right string) error {
			if err := real(left, right); err != nil {
				return err
			}
			lockDir(t, f.root)
			return nil
		}
		if _, err := transaction.Rollback(); err == nil || !strings.Contains(err.Error(), "remove displaced command symlink after rollback") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("restored directory sync fails", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.deps.syncDir = func(string) error { return errors.New("sync boom") }
		plan, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "sync restored command directory") || plan.RolledBack {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
	})
	t.Run("cleanup sync fails after rollback completed", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		calls := 0
		transaction.deps.syncDir = func(string) error {
			calls++
			if calls == 2 {
				return errors.New("sync boom")
			}
			return nil
		}
		plan, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "sync rollback cleanup") || !plan.RolledBack || plan.BackupRetained {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		assertRegularCommand(t, f.command, "old", 0o751)
	})
	t.Run("backup cannot be removed", func(t *testing.T) {
		skipIfRoot(t)
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.deps.syncDir = func(string) error { lockDir(t, f.root); return nil }
		plan, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "remove command backup after rollback") || !plan.RolledBack || !plan.BackupRetained {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
	})
	t.Run("retained displaced command cannot be removed", func(t *testing.T) {
		skipIfRoot(t)
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.displaced = filepath.Join(f.root, "retained")
		writeCommandFixture(t, f.root, "retained", "x", 0o600)
		transaction.deps.syncDir = func(string) error { lockDir(t, f.root); return nil }
		_, err := transaction.Rollback()
		if err == nil || !strings.Contains(err.Error(), "remove retained displaced command after rollback") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestManagedCommandPathFinalizeRejectsChangedPathAndReportsSyncFailure(t *testing.T) {
	t.Run("path replaced by regular file", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.Remove(f.command); err != nil {
			t.Fatal(err)
		}
		writeCommandFixture(t, f.root, "issueops", "foreign", 0o755)
		plan, err := transaction.Finalize()
		if err == nil || !strings.Contains(err.Error(), "changed before finalize") || plan.Committed {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		if _, statErr := os.Lstat(plan.BackupPath); statErr != nil {
			t.Fatalf("backup must remain after refused finalize: %v", statErr)
		}
	})
	t.Run("symlink retargeted", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.Remove(f.command); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(f.root, "other"), f.command); err != nil {
			t.Fatal(err)
		}
		if plan, err := transaction.Finalize(); err == nil || plan.Committed {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
	})
	t.Run("path missing", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		if err := os.Remove(f.command); err != nil {
			t.Fatal(err)
		}
		if _, err := transaction.Finalize(); err == nil {
			t.Fatal("finalize accepted a missing managed path")
		}
	})
	t.Run("sync failure after backup removal", func(t *testing.T) {
		f := newCommandFixture(t)
		transaction := f.apply(t)
		transaction.deps.syncDir = func(string) error { return errors.New("sync boom") }
		plan, err := transaction.Finalize()
		if err == nil || !strings.Contains(err.Error(), "sync committed command cleanup") || !plan.Committed || plan.BackupRetained {
			t.Fatalf("plan=%+v err=%v", plan, err)
		}
		if _, statErr := os.Lstat(plan.BackupPath); !os.IsNotExist(statErr) {
			t.Fatalf("backup survived: %v", statErr)
		}
	})
}

func TestCopyManagedCommandFileRefusalsAndCleanup(t *testing.T) {
	f := newCommandFixture(t)
	identity, err := inspectManagedCommand(f.command, f.deps.readBuildInfo)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(f.root, "copy")
	if err := copyManagedCommandFile(filepath.Join(f.root, "absent"), dest, 0o600, nil); err == nil {
		t.Fatal("missing source copied")
	}
	if err := copyManagedCommandFile(f.command, f.target, 0o600, nil); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	assertRegularCommand(t, f.target, "new", 0o755)

	// A source whose identity no longer matches must be refused without leaving a destination.
	if err := os.WriteFile(f.command, []byte("drifted"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := copyManagedCommandFile(f.command, dest, 0o600, &identity); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("drifted source err = %v", err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatalf("partial copy survived: %v", err)
	}
}

func TestVerifyManagedCommandCopyRejectsMetadataAndByteMismatch(t *testing.T) {
	f := newCommandFixture(t)
	identity, err := inspectManagedCommand(f.command, f.deps.readBuildInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyManagedCommandCopy(f.command, identity); err != nil {
		t.Fatalf("identical file rejected: %v", err)
	}
	sameSizeDifferentBytes := writeCommandFixture(t, f.root, "bytes", "xyz", 0o751)
	if err := verifyManagedCommandCopy(sameSizeDifferentBytes, identity); err == nil || !strings.Contains(err.Error(), "bytes do not match") {
		t.Fatalf("byte mismatch err = %v", err)
	}
	wrongMode := writeCommandFixture(t, f.root, "mode", "old", 0o755)
	mustChmod(t, wrongMode, 0o755)
	if err := verifyManagedCommandCopy(wrongMode, identity); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("mode mismatch err = %v", err)
	}
	wrongSize := writeCommandFixture(t, f.root, "size", "longer", 0o751)
	mustChmod(t, wrongSize, 0o751)
	if err := verifyManagedCommandCopy(wrongSize, identity); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("size mismatch err = %v", err)
	}
	link := filepath.Join(f.root, "link")
	if err := os.Symlink(f.command, link); err != nil {
		t.Fatal(err)
	}
	if err := verifyManagedCommandCopy(link, identity); err == nil {
		t.Fatal("symlink accepted as restored command")
	}
	if err := verifyManagedCommandCopy(filepath.Join(f.root, "absent"), identity); err == nil {
		t.Fatal("missing file accepted")
	}
	unreadable := writeCommandFixture(t, f.root, "unreadable", "old", 0o751)
	if os.Geteuid() != 0 {
		mustChmod(t, unreadable, 0o111)
		if err := verifyManagedCommandCopy(unreadable, identity); err == nil {
			t.Fatal("unreadable file accepted")
		}
	}
}

func TestSyncManagedCommandDirectory(t *testing.T) {
	if err := syncManagedCommandDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := syncManagedCommandDirectory(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("missing directory synced")
	}
}
