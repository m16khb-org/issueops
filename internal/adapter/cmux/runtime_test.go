package cmux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"
	port "issueops/internal/port/cmux"
	"issueops/internal/testsupport"
)

type foreignDirectory struct{}

func (foreignDirectory) Path() string                    { return "/" }
func (foreignDirectory) SamePath(string) bool            { return true }
func (foreignDirectory) Pin() (port.DirectoryPin, error) { return nil, errors.New("unused") }

func TestObserveDirectoryRejectsRelativeUncleanMissingAndFilePaths(t *testing.T) {
	root := canonicalTempDir(t)
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"relative": "relative/dir",
		"unclean":  root + "/../" + filepath.Base(root),
		"missing":  filepath.Join(root, "missing"),
		"file":     file,
	} {
		t.Run(name, func(t *testing.T) {
			if dir, err := ObserveDirectory(path); err == nil {
				t.Fatalf("accepted %q as directory %v", path, dir)
			}
		})
	}
}

func TestObservedDirectoryResolvesSymlinksAndDetectsReplacement(t *testing.T) {
	root := canonicalTempDir(t)
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(work, link); err != nil {
		t.Fatal(err)
	}
	observed, err := ObserveDirectory(link)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Path() != work {
		t.Fatalf("path=%q want resolved %q", observed.Path(), work)
	}
	if !observed.SamePath(work) || !observed.SamePath(link) {
		t.Fatal("same directory through direct path or symlink was not recognized")
	}
	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if observed.SamePath(other) || observed.SamePath("relative") || observed.SamePath(filepath.Join(root, "missing")) {
		t.Fatal("different, relative, or missing path matched the observed directory")
	}

	pin, err := observed.Pin()
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	again, err := ObserveDirectory(work)
	if err != nil {
		t.Fatal(err)
	}
	if !pin.Matches(again) {
		t.Fatal("pin did not match the unchanged directory")
	}
	if pin.Matches(foreignDirectory{}) {
		t.Fatal("pin matched a directory implementation it did not observe")
	}

	if err := os.Rename(work, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	replaced, err := ObserveDirectory(work)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Matches(replaced) {
		t.Fatal("pin matched a replacement directory at the same path")
	}
	if observed.SamePath(work) {
		t.Fatal("replacement directory treated as the originally observed directory")
	}
}

func TestPinFailsWhenDirectoryDisappeared(t *testing.T) {
	root := canonicalTempDir(t)
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	observed, err := ObserveDirectory(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(work); err != nil {
		t.Fatal(err)
	}
	if pin, err := observed.Pin(); err == nil {
		pin.Close()
		t.Fatal("pinned a removed directory")
	}
}

func TestValidateHostExecutableRequiresCleanAbsoluteExecutableRegularFile(t *testing.T) {
	root := canonicalTempDir(t)
	exe := filepath.Join(root, "host")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(root, "plain")
	if err := os.WriteFile(plain, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHostExecutable(exe); err != nil {
		t.Fatalf("executable rejected: %v", err)
	}
	for name, path := range map[string]string{
		"relative":       "host",
		"unclean":        root + "/./host",
		"missing":        filepath.Join(root, "missing"),
		"not executable": plain,
		"directory":      root,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateHostExecutable(path); err == nil {
				t.Fatalf("accepted %q", path)
			}
		})
	}
}

func TestGitTopReturnsRepositoryRootFromSubdirectoryAndErrorsOutsideRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	testsupport.IsolateGitConfig(t)
	root := canonicalTempDir(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	plain := filepath.Join(root, "plain")
	repo := filepath.Join(root, "repo")
	sub := filepath.Join(repo, "a", "b")
	for _, dir := range []string{plain, sub} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	top, err := GitTop(sub)
	if err != nil {
		t.Fatal(err)
	}
	if !SamePath(top, repo) {
		t.Fatalf("top=%q want %q", top, repo)
	}
	if top, err := GitTop(plain); err == nil {
		t.Fatalf("non-repository reported top %q", top)
	}
}

func TestSamePathComparesResolvedTargetsAndRejectsMissingPaths(t *testing.T) {
	root := canonicalTempDir(t)
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if !SamePath(dir, link) || !SamePath(dir+"/", dir) {
		t.Fatal("symlink or trailing-slash alias not treated as same path")
	}
	if SamePath(dir, root) {
		t.Fatal("distinct directories treated as same path")
	}
	missing := filepath.Join(root, "missing")
	if SamePath(missing, missing) {
		t.Fatal("two identical missing paths treated as same")
	}
}

func TestCmuxShellProcessRecognizesInteractiveShellsOnly(t *testing.T) {
	for path, want := range map[string]bool{
		"/bin/sh": true, "/bin/BASH": true, "/usr/bin/zsh": true, "/bin/dash": true, `C:\x\sh.exe`: false, "/bin/sh.exe": true,
		"/opt/native/codex": false, "/usr/bin/fish": false, "/opt/shell": false,
	} {
		if got := cmuxShellProcess(path); got != want {
			t.Errorf("cmuxShellProcess(%q)=%v want %v", path, got, want)
		}
	}
}

func receiptBytes(r BootstrapReceipt, pid string) []byte {
	fields := []string{"1", r.Status, pid, r.CWD, r.WindowID, r.WorkspaceID, r.SurfaceID, r.SocketPath, r.HostExecutable, r.PromptSHA256, r.MaterialSHA256, r.HostArgvSHA256}
	return []byte(strings.Join(fields, "\x00") + "\x00")
}

func bootstrapFixture(t *testing.T) (cmuxcontract.PreparedLauncher, cmuxcontract.BootstrapExpectation, BootstrapReceipt) {
	t.Helper()
	expected := cmuxcontract.BootstrapExpectation{
		CWD: "/repo/worktree", WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface, SocketPath: socketPath,
		HostExecutable: "/opt/native/codex", HostArgvSHA256: strings.Repeat("c", 64), PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64),
	}
	receipt := BootstrapReceipt{Status: "ok", CWD: expected.CWD, WindowID: expected.WindowID, WorkspaceID: expected.WorkspaceID, SurfaceID: expected.SurfaceID,
		SocketPath: expected.SocketPath, HostExecutable: expected.HostExecutable, PromptSHA256: expected.PromptSHA256, MaterialSHA256: expected.MaterialSHA256, HostArgvSHA256: expected.HostArgvSHA256}
	return cmuxcontract.PreparedLauncher{ReceiptPath: filepath.Join(canonicalTempDir(t), "bootstrap.receipt")}, expected, receipt
}

func TestAwaitBootstrapReceiptReturnsObservedNativeProcess(t *testing.T) {
	prepared, expected, receipt := bootstrapFixture(t)
	if err := os.WriteFile(prepared.ReceiptPath, receiptBytes(receipt, "4242"), 0o600); err != nil {
		t.Fatal(err)
	}
	process := issueopscontract.NativeProcessReceipt{PID: 4242, StartedAt: "2026-09-20T10:00:00Z", Executable: expected.HostExecutable}
	got, err := AwaitBootstrapReceipt(t.Context(), prepared, expected, func(pid int) (issueopscontract.NativeProcessReceipt, error) {
		if pid != 4242 {
			t.Fatalf("observed pid=%d", pid)
		}
		return process, nil
	})
	if err != nil || got != process {
		t.Fatalf("process=%+v err=%v", got, err)
	}
}

func TestAwaitBootstrapReceiptFailsFastOnReceiptReportingMismatchOrMalformedFile(t *testing.T) {
	observe := func(int) (issueopscontract.NativeProcessReceipt, error) {
		t.Fatal("process observed for rejected receipt")
		return issueopscontract.NativeProcessReceipt{}, nil
	}
	t.Run("launcher reported identity mismatch", func(t *testing.T) {
		prepared, expected, receipt := bootstrapFixture(t)
		receipt.Status = "identity_mismatch"
		if err := os.WriteFile(prepared.ReceiptPath, receiptBytes(receipt, "4242"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := AwaitBootstrapReceipt(t.Context(), prepared, expected, observe)
		if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("malformed receipt", func(t *testing.T) {
		prepared, expected, _ := bootstrapFixture(t)
		if err := os.WriteFile(prepared.ReceiptPath, []byte("1\x00ok\x00"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := AwaitBootstrapReceipt(t.Context(), prepared, expected, observe)
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestAwaitBootstrapReceiptKeepsWaitingUntilContextEndsWhileReceiverIsShellOrUnobservable(t *testing.T) {
	t.Run("receiver still a shell", func(t *testing.T) {
		prepared, expected, receipt := bootstrapFixture(t)
		expected.HostExecutable = "/bin/sh"
		receipt.HostExecutable = "/bin/sh"
		if err := os.WriteFile(prepared.ReceiptPath, receiptBytes(receipt, "4242"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		calls := 0
		_, err := AwaitBootstrapReceipt(ctx, prepared, expected, func(int) (issueopscontract.NativeProcessReceipt, error) {
			calls++
			cancel()
			return issueopscontract.NativeProcessReceipt{PID: 4242, StartedAt: "t", Executable: "/bin/sh"}, nil
		})
		if !errors.Is(err, context.Canceled) || calls == 0 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
	t.Run("observer error with ok receipt is retried", func(t *testing.T) {
		prepared, expected, receipt := bootstrapFixture(t)
		if err := os.WriteFile(prepared.ReceiptPath, receiptBytes(receipt, "4242"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		_, err := AwaitBootstrapReceipt(ctx, prepared, expected, func(int) (issueopscontract.NativeProcessReceipt, error) {
			cancel()
			return issueopscontract.NativeProcessReceipt{}, errors.New("process not yet visible")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("receipt never appears", func(t *testing.T) {
		prepared, expected, _ := bootstrapFixture(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := AwaitBootstrapReceipt(ctx, prepared, expected, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})
}
