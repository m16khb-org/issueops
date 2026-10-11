package cmux

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"

	"golang.org/x/sys/unix"
)

func launcherRequestFixture(t *testing.T) cmuxcontract.ArtifactRequest {
	t.Helper()
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "codex")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("prompt")
	return cmuxcontract.ArtifactRequest{
		Root: filepath.Join(root, "artifacts"), CWD: worktree, WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface,
		SocketPath: socketPath, Host: "codex", HostExecutable: host, Model: "model", Prompt: prompt,
		PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
	}
}

func TestPrepareLauncherRejectsInvalidRequestsAndLeavesNoArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*cmuxcontract.ArtifactRequest)
		want   string
	}{
		{name: "relative root", mutate: func(r *cmuxcontract.ArtifactRequest) { r.Root = "artifacts" }, want: "scope is invalid"},
		{name: "relative cwd", mutate: func(r *cmuxcontract.ArtifactRequest) { r.CWD = "work" }, want: "scope is invalid"},
		{name: "relative socket", mutate: func(r *cmuxcontract.ArtifactRequest) { r.SocketPath = "s.sock" }, want: "scope is invalid"},
		{name: "bad window id", mutate: func(r *cmuxcontract.ArtifactRequest) { r.WindowID = "w" }, want: "scope is invalid"},
		{name: "bad workspace id", mutate: func(r *cmuxcontract.ArtifactRequest) { r.WorkspaceID = "w" }, want: "scope is invalid"},
		{name: "bad surface id", mutate: func(r *cmuxcontract.ArtifactRequest) { r.SurfaceID = "" }, want: "scope is invalid"},
		{name: "prompt digest mismatch", mutate: func(r *cmuxcontract.ArtifactRequest) { r.Prompt = []byte("tampered") }, want: "digest is invalid"},
		{name: "prompt with NUL", mutate: func(r *cmuxcontract.ArtifactRequest) {
			r.Prompt = []byte("a\x00b")
			r.PromptSHA256 = digestBytes(r.Prompt)
		}, want: "digest is invalid"},
		{name: "material digest malformed", mutate: func(r *cmuxcontract.ArtifactRequest) { r.MaterialSHA256 = strings.Repeat("B", 64) }, want: "digest is invalid"},
		{name: "host missing", mutate: func(r *cmuxcontract.ArtifactRequest) { r.HostExecutable = filepath.Join(r.CWD, "missing") }, want: "host executable is unavailable"},
		{name: "host not executable", mutate: func(r *cmuxcontract.ArtifactRequest) {
			plain := filepath.Join(r.CWD, "plain")
			if err := os.WriteFile(plain, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			r.HostExecutable = plain
		}, want: "host executable is unavailable"},
		{name: "root with backslash", mutate: func(r *cmuxcontract.ArtifactRequest) { r.Root = r.Root + `\x` }, want: "root is not safe"},
		{name: "root with newline", mutate: func(r *cmuxcontract.ArtifactRequest) { r.Root = r.Root + "\nx" }, want: "root is not safe"},
		{name: "cwd with newline", mutate: func(r *cmuxcontract.ArtifactRequest) { r.CWD = r.CWD + "\nx" }, want: "identity is not safe"},
		{name: "socket with CR", mutate: func(r *cmuxcontract.ArtifactRequest) { r.SocketPath = socketPath + "\r" }, want: "identity is not safe"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := launcherRequestFixture(t)
			test.mutate(&request)
			built := false
			_, err := PrepareLauncher(request, func(string, string, string, string, string) ([]string, error) {
				built = true
				return []string{"x", "p"}, nil
			})
			if err == nil || !strings.Contains(err.Error(), test.want) || built {
				t.Fatalf("err=%v built=%v want %q", err, built, test.want)
			}
			if entries, readErr := os.ReadDir(request.Root); readErr == nil && len(entries) != 0 {
				t.Fatalf("rejected request left artifacts: %v", entries)
			}
		})
	}
}

func TestPrepareLauncherRemovesArtifactDirectoryWhenArgvBuildFails(t *testing.T) {
	request := launcherRequestFixture(t)
	boom := errors.New("unsupported host")
	_, err := PrepareLauncher(request, func(string, string, string, string, string) ([]string, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	entries, readErr := os.ReadDir(request.Root)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("failed preparation left %v (err=%v)", entries, readErr)
	}
}

func TestPrepareLauncherRefusesUnsafeArtifactRoot(t *testing.T) {
	t.Run("group-readable root", func(t *testing.T) {
		request := launcherRequestFixture(t)
		if err := os.Mkdir(request.Root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(request.Root, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareTestLauncher(request); err == nil || !strings.Contains(err.Error(), "private non-symlink directory") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("symlinked root", func(t *testing.T) {
		request := launcherRequestFixture(t)
		target := filepath.Join(filepath.Dir(request.Root), "real")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, request.Root); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareTestLauncher(request); err == nil || !strings.Contains(err.Error(), "private non-symlink directory") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("root is a regular file", func(t *testing.T) {
		request := launcherRequestFixture(t)
		if err := os.WriteFile(request.Root, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareTestLauncher(request); err == nil {
			t.Fatal("regular file accepted as artifact root")
		}
	})
}

func TestCleanupLauncherRemovesDirectoryAndIgnoresBlankPath(t *testing.T) {
	request := launcherRequestFixture(t)
	prepared, err := prepareTestLauncher(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := CleanupLauncher(prepared); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prepared.Directory); !os.IsNotExist(err) {
		t.Fatalf("directory still present: %v", err)
	}
	if err := os.Mkdir(request.CWD+"/sentinel", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CleanupLauncher(cmuxcontract.PreparedLauncher{Directory: "  "}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(request.CWD + "/sentinel"); err != nil {
		t.Fatalf("blank-directory cleanup removed unrelated state: %v", err)
	}
}

func TestReadBootstrapReceiptParsesAllFieldsAndRejectsMalformedFiles(t *testing.T) {
	dir := canonicalTempDir(t)
	want := BootstrapReceipt{Status: "ok", CWD: "/w", WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface, SocketPath: socketPath,
		HostExecutable: "/h", PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64), HostArgvSHA256: strings.Repeat("c", 64)}
	good := filepath.Join(dir, "good")
	want.PID = 77
	if err := os.WriteFile(good, receiptBytes(want, "77"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBootstrapReceipt(good)
	if err != nil || got != want {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := ReadBootstrapReceipt(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing receipt err=%v", err)
	}
	for name, data := range map[string][]byte{
		"too few fields":  []byte("1\x00ok\x00"),
		"too many fields": append(receiptBytes(want, "77"), []byte("extra\x00")...),
		"wrong version":   []byte(strings.Replace(string(receiptBytes(want, "77")), "1\x00ok", "2\x00ok", 1)),
		"non numeric pid": receiptBytes(want, "abc"),
		"zero pid":        receiptBytes(want, "0"),
		"negative pid":    receiptBytes(want, "-5"),
		"empty":           nil,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if receipt, err := ReadBootstrapReceipt(path); err == nil {
				t.Fatalf("accepted %+v", receipt)
			}
		})
	}
}

func TestValidateBootstrapReceiptRejectsMissingObserverIncompleteProcessAndSymlinkAliases(t *testing.T) {
	prepared, expected, receipt := bootstrapFixture(t)
	_ = prepared
	receipt.PID = 5150
	process := issueopscontract.NativeProcessReceipt{PID: 5150, StartedAt: "t", Executable: expected.HostExecutable}
	observe := func(p issueopscontract.NativeProcessReceipt) func(int) (issueopscontract.NativeProcessReceipt, error) {
		return func(int) (issueopscontract.NativeProcessReceipt, error) { return p, nil }
	}
	if _, err := ValidateBootstrapReceipt(receipt, expected, nil); err == nil || !strings.Contains(err.Error(), "observer is unavailable") {
		t.Fatalf("nil observer err=%v", err)
	}
	boom := errors.New("no such process")
	if _, err := ValidateBootstrapReceipt(receipt, expected, func(int) (issueopscontract.NativeProcessReceipt, error) {
		return issueopscontract.NativeProcessReceipt{}, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("observer error not propagated: %v", err)
	}
	for name, mutate := range map[string]func(*issueopscontract.NativeProcessReceipt){
		"pid differs":      func(p *issueopscontract.NativeProcessReceipt) { p.PID++ },
		"no start time":    func(p *issueopscontract.NativeProcessReceipt) { p.StartedAt = "" },
		"no executable":    func(p *issueopscontract.NativeProcessReceipt) { p.Executable = "" },
		"relative exe":     func(p *issueopscontract.NativeProcessReceipt) { p.Executable = "codex" },
		"unclean exe":      func(p *issueopscontract.NativeProcessReceipt) { p.Executable = "/opt/native/../native/codex" },
		"unresolvable exe": func(p *issueopscontract.NativeProcessReceipt) { p.Executable = "/opt/native/other" },
	} {
		t.Run(name, func(t *testing.T) {
			p := process
			mutate(&p)
			if _, err := ValidateBootstrapReceipt(receipt, expected, observe(p)); err == nil {
				t.Fatal("accepted mismatching process")
			}
		})
	}
	t.Run("host-side mismatches", func(t *testing.T) {
		for name, mutate := range map[string]func(*BootstrapReceipt, *cmuxcontract.BootstrapExpectation){
			"status not ok":   func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.Status = "identity_mismatch" },
			"workspace":       func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.WorkspaceID = "x" },
			"surface":         func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.SurfaceID = "x" },
			"socket":          func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.SocketPath = "/x" },
			"window":          func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.WindowID = "other" },
			"host":            func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.HostExecutable = "/x" },
			"prompt digest":   func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.PromptSHA256 = "x" },
			"material digest": func(r *BootstrapReceipt, _ *cmuxcontract.BootstrapExpectation) { r.MaterialSHA256 = "x" },
			"expected argv empty": func(r *BootstrapReceipt, e *cmuxcontract.BootstrapExpectation) {
				e.HostArgvSHA256, r.HostArgvSHA256 = "", ""
			},
			"expected argv upper": func(r *BootstrapReceipt, e *cmuxcontract.BootstrapExpectation) {
				e.HostArgvSHA256 = strings.Repeat("C", 64)
				r.HostArgvSHA256 = e.HostArgvSHA256
			},
		} {
			t.Run(name, func(t *testing.T) {
				r, e := receipt, expected
				mutate(&r, &e)
				_, err := ValidateBootstrapReceipt(r, e, observe(process))
				if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
					t.Fatalf("err=%v", err)
				}
			})
		}
	})
	t.Run("symlink alias of the expected host is the same executable", func(t *testing.T) {
		dir := canonicalTempDir(t)
		real := filepath.Join(dir, "codex-real")
		if err := os.WriteFile(real, []byte("x"), 0o700); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(dir, "codex")
		if err := os.Symlink(real, alias); err != nil {
			t.Fatal(err)
		}
		e, r := expected, receipt
		e.HostExecutable, r.HostExecutable = alias, alias
		p := process
		p.Executable = real
		if _, err := ValidateBootstrapReceipt(r, e, observe(p)); err != nil {
			t.Fatalf("symlink alias rejected: %v", err)
		}
	})
}

func TestObserveEndpointRejectsNonCanonicalPathNonSocketAndBadParent(t *testing.T) {
	dir := shortTempDir(t)
	if _, err := ObserveEndpoint("rel.sock", os.Getuid()); err == nil || !strings.Contains(err.Error(), "absolute and clean") {
		t.Fatalf("relative err=%v", err)
	}
	if _, err := ObserveEndpoint(dir+"/../"+filepath.Base(dir)+"/x.sock", os.Getuid()); err == nil || !strings.Contains(err.Error(), "absolute and clean") {
		t.Fatalf("unclean err=%v", err)
	}
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveEndpoint(regular, os.Getuid()); err == nil || !strings.Contains(err.Error(), "not a Unix socket") {
		t.Fatalf("regular file err=%v", err)
	}

	t.Run("symlinked parent directory", func(t *testing.T) {
		real := shortTempDir(t)
		listener, err := net.Listen("unix", filepath.Join(real, "s.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		if err := os.Chmod(filepath.Join(real, "s.sock"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		// Lstat of link/s.sock resolves the intermediate symlink, so the leaf is a
		// socket while the parent component is a symlink and must be refused.
		if _, err := ObserveEndpoint(filepath.Join(link, "s.sock"), os.Getuid()); err == nil || !strings.Contains(err.Error(), "parent must be a non-symlink") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("world-writable parent without sticky bit", func(t *testing.T) {
		open := shortTempDir(t)
		listener, err := net.Listen("unix", filepath.Join(open, "s.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		if err := os.Chmod(filepath.Join(open, "s.sock"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(open, 0o777); err != nil {
			t.Fatal(err)
		}
		if _, err := ObserveEndpoint(filepath.Join(open, "s.sock"), os.Getuid()); err == nil || !strings.Contains(err.Error(), "parent permissions are unsafe") {
			t.Fatalf("err=%v", err)
		}
		if err := os.Chmod(open, 0o777|os.ModeSticky); err != nil {
			t.Fatal(err)
		}
		got, err := ObserveEndpoint(filepath.Join(open, "s.sock"), os.Getuid())
		if err != nil || got.ParentMode != 0o1777 {
			t.Fatalf("sticky parent: mode=%#o err=%v", got.ParentMode, err)
		}
	})
	t.Run("socket owned by another uid", func(t *testing.T) {
		own := shortTempDir(t)
		listener, err := net.Listen("unix", filepath.Join(own, "s.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		if err := os.Chmod(filepath.Join(own, "s.sock"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ObserveEndpoint(filepath.Join(own, "s.sock"), os.Getuid()+1); err == nil || !strings.Contains(err.Error(), "owner") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestValidateEndpointRejectsEachIncompleteOrUnsafeField(t *testing.T) {
	base := endpointFixture()
	uid := int(base.OwnerUID)
	tests := []struct {
		name   string
		mutate func(*issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation)
		want   string
	}{
		{name: "path differs", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.Path = "/other" }, want: "incomplete"},
		{name: "kind", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.Kind = "file" }, want: "incomplete"},
		{name: "zero inode", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.Inode = 0 }, want: "incomplete"},
		{name: "non-positive ctime", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.CTimeNS = 0 }, want: "incomplete"},
		{name: "zero parent inode", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.ParentInode = 0 }, want: "incomplete"},
		{name: "mode too open", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.Mode = 0o660 }, want: "0600"},
		{name: "mode too closed", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.Mode = 0o400 }, want: "0600"},
		{name: "parent owned by stranger", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.ParentOwnerUID = 999 }, want: "parent owner"},
		{name: "parent group-writable", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.ParentMode = 0o770 }, want: "parent permissions"},
		{name: "parent world-writable", mutate: func(e *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) { e.ParentMode = 0o702 }, want: "parent permissions"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := base
			test.mutate(&endpoint)
			if err := ValidateEndpoint(endpoint, base.Path, uid); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
		})
	}
	own := base
	own.ParentOwnerUID = base.OwnerUID
	own.ParentMode = 0o700
	if err := ValidateEndpoint(own, base.Path, uid); err != nil {
		t.Fatalf("user-owned private parent rejected: %v", err)
	}
}

func TestPermissionModeKeepsStickyBitAndDropsFileType(t *testing.T) {
	for mode, want := range map[os.FileMode]uint32{
		0o600:                              0o600,
		os.ModeSocket | 0o600:              0o600,
		os.ModeDir | os.ModeSticky | 0o777: 0o1777,
		os.ModeDir | 0o755:                 0o755,
	} {
		if got := permissionMode(mode); got != want {
			t.Errorf("permissionMode(%v)=%#o want %#o", mode, got, want)
		}
	}
}

func TestReadPromptRejectsUnsafePathsAndFileShapes(t *testing.T) {
	root := canonicalTempDir(t)
	value := []byte("sealed")
	sum := digestBytes(value)
	write := func(rel string, mode os.FileMode) string {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, value, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	good := write("prompt", 0o600)
	nested := write("a/b/prompt", 0o600)
	group := write("grouped", 0o640)
	if got, err := ReadPrompt(root, nested, sum); err != nil || string(got) != "sealed" {
		t.Fatalf("nested prompt got=%q err=%v", got, err)
	}
	other := canonicalTempDir(t)
	outside := filepath.Join(other, "prompt")
	if err := os.WriteFile(outside, value, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name               string
		root, path, digest string
	}{
		{name: "relative root", root: "rel", path: good, digest: sum},
		{name: "unclean root", root: root + "/.", path: good, digest: sum},
		{name: "relative path", root: root, path: "prompt", digest: sum},
		{name: "unclean path", root: root, path: root + "/a/../prompt", digest: sum},
		{name: "NUL in path", root: root, path: good + "\x00", digest: sum},
		{name: "malformed digest", root: root, path: good, digest: "abc"},
		{name: "uppercase digest", root: root, path: good, digest: strings.ToUpper(sum)},
		{name: "path is the root", root: root, path: root, digest: sum},
		{name: "path outside root", root: root, path: outside, digest: sum},
		{name: "parent of root", root: filepath.Join(root, "a"), path: good, digest: sum},
		{name: "missing file", root: root, path: filepath.Join(root, "absent"), digest: sum},
		{name: "missing directory", root: root, path: filepath.Join(root, "no", "dir", "prompt"), digest: sum},
		{name: "intermediate component is a file", root: root, path: filepath.Join(good, "prompt"), digest: sum},
		{name: "missing root", root: filepath.Join(root, "gone"), path: filepath.Join(root, "gone", "prompt"), digest: sum},
		{name: "group readable leaf", root: root, path: group, digest: sum},
		{name: "digest mismatch", root: root, path: good, digest: strings.Repeat("0", 64)},
		{name: "leaf is a directory", root: root, path: filepath.Join(root, "a"), digest: sum},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, err := ReadPrompt(test.root, test.path, test.digest); err == nil {
				t.Fatalf("accepted %q", got)
			}
		})
	}
}

func TestReadPromptRejectsContentChangedWhileReadingAndSizeGrowth(t *testing.T) {
	root := canonicalTempDir(t)
	path := filepath.Join(root, "prompt")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readPromptPlatform(root, path, digestBytes(original), func() {
		if err := os.WriteFile(path, []byte("original but grown"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") && !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("grown prompt err=%v", err)
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = readPromptPlatform(root, path, digestBytes(original), func() {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("mode change during read err=%v", err)
	}
}

func TestOpenPromptDirectoriesStopsAtFirstUnsafeComponentWithoutLeakingDescriptors(t *testing.T) {
	root := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "ok"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "ok"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(rootFD)

	components, last, err := openPromptDirectories(rootFD, []string{"ok"})
	if err != nil || len(components) != 1 || last != components[0].fd || components[0].name != "ok" {
		t.Fatalf("components=%+v last=%d err=%v", components, last, err)
	}
	closePromptComponents(components)
	if err := unix.Fstat(last, &unix.Stat_t{}); err == nil {
		t.Fatal("closePromptComponents left the directory descriptor open")
	}

	if components, last, err := openPromptDirectories(rootFD, nil); err != nil || len(components) != 0 || last != rootFD {
		t.Fatalf("empty chain components=%d last=%d err=%v", len(components), last, err)
	}
	for _, names := range [][]string{{"ok", "missing"}, {"ok", "link"}, {"file"}, {"link"}} {
		if components, _, err := openPromptDirectories(rootFD, names); err == nil {
			closePromptComponents(components)
			t.Fatalf("accepted directory chain %v", names)
		}
	}
}
