package cmux

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/adapter/hostprotocol"
	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
)

func TestValidateBootstrapReceiptRequiresExactProcessCorrelation(t *testing.T) {
	hostPath := testsupport.AbsPath("opt", "native", "codex")
	expected := cmuxcontract.BootstrapExpectation{
		CWD: worktreePath, WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface, SocketPath: socketPath,
		HostExecutable: hostPath, HostArgvSHA256: strings.Repeat("c", 64), PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64),
	}
	receipt := bootstrapReceipt{Status: "ok", PID: 8080, CWD: expected.CWD, WindowID: expected.WindowID, WorkspaceID: expected.WorkspaceID, SurfaceID: expected.SurfaceID,
		SocketPath: expected.SocketPath, HostExecutable: expected.HostExecutable, PromptSHA256: expected.PromptSHA256, MaterialSHA256: expected.MaterialSHA256,
		HostArgvSHA256: strings.Repeat("c", 64)}
	process := issueopscontract.NativeProcessReceipt{PID: 8080, StartedAt: "2026-09-20T10:00:00Z", Executable: expected.HostExecutable}
	got, err := validateBootstrapReceipt(receipt, expected, func(pid int) (issueopscontract.NativeProcessReceipt, error) {
		if pid != receipt.PID {
			t.Fatalf("pid=%d", pid)
		}
		return process, nil
	})
	if err != nil || !reflect.DeepEqual(got, process) {
		t.Fatalf("process=%+v err=%v", got, err)
	}
	receipt.WindowID = ""
	if _, err := validateBootstrapReceipt(receipt, expected, func(int) (issueopscontract.NativeProcessReceipt, error) { return process, nil }); err != nil {
		t.Fatalf("optional absent ambient window rejected: %v", err)
	}
	receipt.WindowID = expected.WindowID
	receipt.CWD = testsupport.AbsPath("tmp", "wrong")
	if _, err := validateBootstrapReceipt(receipt, expected, func(int) (issueopscontract.NativeProcessReceipt, error) { return process, nil }); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("wrong cwd accepted: %v", err)
	}
	receipt.CWD = expected.CWD
	wrongProcess := process
	wrongProcess.Executable = testsupport.AbsPath("usr", "bin", "python3")
	if _, err := validateBootstrapReceipt(receipt, expected, func(int) (issueopscontract.NativeProcessReceipt, error) { return wrongProcess, nil }); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("wrong non-shell receiver executable accepted: %v", err)
	}
	receipt.HostArgvSHA256 = strings.Repeat("d", 64)
	if _, err := validateBootstrapReceipt(receipt, expected, func(int) (issueopscontract.NativeProcessReceipt, error) { return process, nil }); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("wrong host argv digest accepted: %v", err)
	}
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareTestLauncher(request cmuxcontract.ArtifactRequest) (cmuxcontract.PreparedLauncher, error) {
	return PrepareLauncher(request, func(host, executable, model, effort, prompt string) ([]string, error) {
		return hostprotocol.BuildInteractiveArgv(host, executable, model, effort, prompt, nil)
	})
}
