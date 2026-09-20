package cmux

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/nativehost"
)

// MaximumPromptBytes keeps the prompt argv plus its terminating NUL below
// Linux MAX_ARG_STRLEN (32 pages, 128 KiB on 4 KiB-page systems) with a 2x
// margin and remains portable to Darwin. ReadPrompt and PrepareLauncher must
// enforce this one shared bound.
const MaximumPromptBytes = 64 << 10

type ArtifactRequest struct {
	Root           string
	CWD            string
	WindowID       string
	WorkspaceID    string
	SurfaceID      string
	SocketPath     string
	Host           string
	HostExecutable string
	Model          string
	Effort         string
	Prompt         []byte
	PromptSHA256   string
	MaterialSHA256 string
}

type PreparedLauncher struct {
	Directory      string
	PromptPath     string
	LauncherPath   string
	ReceiptPath    string
	Command        string
	HostArgvSHA256 string
}

type BootstrapReceipt struct {
	Status         string
	PID            int
	CWD            string
	WindowID       string
	WorkspaceID    string
	SurfaceID      string
	SocketPath     string
	HostExecutable string
	PromptSHA256   string
	MaterialSHA256 string
	HostArgvSHA256 string
}

type BootstrapExpectation struct {
	CWD            string
	WindowID       string
	WorkspaceID    string
	SurfaceID      string
	SocketPath     string
	HostExecutable string
	HostArgvSHA256 string
	PromptSHA256   string
	MaterialSHA256 string
}

func PrepareLauncher(request ArtifactRequest) (PreparedLauncher, error) {
	if err := requireSupportedPlatform(); err != nil {
		return PreparedLauncher{}, err
	}
	if !filepath.IsAbs(request.Root) || !filepath.IsAbs(request.CWD) || !filepath.IsAbs(request.SocketPath) ||
		!validUUID(request.WindowID) || !validUUID(request.WorkspaceID) || !validUUID(request.SurfaceID) {
		return PreparedLauncher{}, fmt.Errorf("cmux launcher artifact scope is invalid")
	}
	if len(request.Prompt) > MaximumPromptBytes || bytes.IndexByte(request.Prompt, 0) >= 0 || digest(request.Prompt) != request.PromptSHA256 || !validDigest(request.MaterialSHA256) {
		return PreparedLauncher{}, fmt.Errorf("cmux launcher prompt or material digest is invalid")
	}
	hostInfo, err := os.Stat(request.HostExecutable)
	if err != nil || !hostInfo.Mode().IsRegular() || hostInfo.Mode().Perm()&0o111 == 0 {
		return PreparedLauncher{}, fmt.Errorf("native host executable is unavailable")
	}
	if strings.ContainsAny(request.Root, "\\\r\n\x00") {
		return PreparedLauncher{}, fmt.Errorf("cmux launcher artifact root is not safe for raw input")
	}
	if strings.ContainsAny(request.CWD+request.SocketPath, "\r\n\x00") {
		return PreparedLauncher{}, fmt.Errorf("cmux launcher identity is not safe for raw input")
	}
	if err := ensurePrivateDirectory(request.Root); err != nil {
		return PreparedLauncher{}, err
	}
	directory, err := os.MkdirTemp(request.Root, "issueops-cmux-")
	if err != nil {
		return PreparedLauncher{}, err
	}
	prepared := PreparedLauncher{
		Directory: directory, PromptPath: filepath.Join(directory, "prompt"),
		LauncherPath: filepath.Join(directory, "launch.sh"), ReceiptPath: filepath.Join(directory, "bootstrap.receipt"),
	}
	fail := func(cause error) (PreparedLauncher, error) {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			cause = errors.Join(cause, fmt.Errorf("cleanup cmux launcher artifact: %w", cleanupErr))
		}
		return PreparedLauncher{}, cause
	}
	if err := os.WriteFile(prepared.PromptPath, request.Prompt, 0o600); err != nil {
		return fail(err)
	}
	argv, err := nativehost.BuildInteractiveArgv(request.Host, request.HostExecutable, request.Model, request.Effort, "prompt-placeholder")
	if err != nil {
		return fail(err)
	}
	argvDigest := digest([]byte(strings.Join(argv[:len(argv)-1], "\x00") + "\x00" + request.PromptSHA256))
	prepared.HostArgvSHA256 = argvDigest
	script := launcherScript(request, prepared, argv, argvDigest)
	if err := os.WriteFile(prepared.LauncherPath, []byte(script), 0o700); err != nil {
		return fail(err)
	}
	prepared.Command = "ISSUEOPS_CMUX_EXPECTED_WINDOW_ID=" + shellQuote(request.WindowID) +
		" ISSUEOPS_CMUX_EXPECTED_WORKSPACE_ID=" + shellQuote(request.WorkspaceID) +
		" ISSUEOPS_CMUX_EXPECTED_SURFACE_ID=" + shellQuote(request.SurfaceID) +
		" ISSUEOPS_CMUX_EXPECTED_SOCKET_PATH=" + shellQuote(request.SocketPath) +
		" /bin/sh " + shellQuote(prepared.LauncherPath)
	return prepared, nil
}

func (prepared PreparedLauncher) Cleanup() error {
	if strings.TrimSpace(prepared.Directory) == "" {
		return nil
	}
	return os.RemoveAll(prepared.Directory)
}

func launcherScript(request ArtifactRequest, prepared PreparedLauncher, argv []string, argvDigest string) string {
	var command strings.Builder
	command.WriteString("exec")
	for _, argument := range argv[:len(argv)-1] {
		command.WriteByte(' ')
		command.WriteString(shellQuote(argument))
	}
	command.WriteString(" \"$prompt\"\n")
	return "#!/bin/sh\n" +
		"set -eu\n" +
		"umask 077\n" +
		"status=ok\n" +
		"[ \"$PWD\" = " + shellQuote(request.CWD) + " ] || status=identity_mismatch\n" +
		"[ \"${ISSUEOPS_CMUX_EXPECTED_WINDOW_ID-}\" = " + shellQuote(request.WindowID) + " ] || status=identity_mismatch\n" +
		"[ \"${ISSUEOPS_CMUX_EXPECTED_WORKSPACE_ID-}\" = " + shellQuote(request.WorkspaceID) + " ] || status=identity_mismatch\n" +
		"[ \"${ISSUEOPS_CMUX_EXPECTED_SURFACE_ID-}\" = " + shellQuote(request.SurfaceID) + " ] || status=identity_mismatch\n" +
		"[ \"${ISSUEOPS_CMUX_EXPECTED_SOCKET_PATH-}\" = " + shellQuote(request.SocketPath) + " ] || status=identity_mismatch\n" +
		"[ -z \"${CMUX_WINDOW_ID-}\" ] || [ \"${CMUX_WINDOW_ID-}\" = \"${ISSUEOPS_CMUX_EXPECTED_WINDOW_ID-}\" ] || status=identity_mismatch\n" +
		"[ \"${CMUX_WORKSPACE_ID-}\" = \"${ISSUEOPS_CMUX_EXPECTED_WORKSPACE_ID-}\" ] || status=identity_mismatch\n" +
		"[ \"${CMUX_SURFACE_ID-}\" = \"${ISSUEOPS_CMUX_EXPECTED_SURFACE_ID-}\" ] || status=identity_mismatch\n" +
		"[ \"${CMUX_SOCKET_PATH-}\" = \"${ISSUEOPS_CMUX_EXPECTED_SOCKET_PATH-}\" ] || status=identity_mismatch\n" +
		"receipt_tmp=" + shellQuote(prepared.ReceiptPath+".tmp") + "\n" +
		"printf '%s\\000' '1' \"$status\" \"$$\" \"$PWD\" \"${CMUX_WINDOW_ID-}\" \"${CMUX_WORKSPACE_ID-}\" \"${CMUX_SURFACE_ID-}\" \"${CMUX_SOCKET_PATH-}\" " +
		shellQuote(request.HostExecutable) + " " + shellQuote(request.PromptSHA256) + " " + shellQuote(request.MaterialSHA256) + " " + shellQuote(argvDigest) + " > \"$receipt_tmp\"\n" +
		"/bin/mv \"$receipt_tmp\" " + shellQuote(prepared.ReceiptPath) + "\n" +
		"[ \"$status\" = ok ] || exit 78\n" +
		"prompt=$({ /bin/cat " + shellQuote(prepared.PromptPath) + "; printf '\\001'; })\n" +
		"prompt=${prompt%?}\n" +
		"/bin/rm -f " + shellQuote(prepared.PromptPath) + " " + shellQuote(prepared.LauncherPath) + "\n" +
		command.String()
}

func ReadBootstrapReceipt(path string) (BootstrapReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BootstrapReceipt{}, err
	}
	parts := splitReceipt(data)
	if len(parts) != 12 || parts[0] != "1" {
		return BootstrapReceipt{}, fmt.Errorf("cmux bootstrap receipt is malformed")
	}
	pid, err := strconv.Atoi(parts[2])
	if err != nil || pid <= 0 {
		return BootstrapReceipt{}, fmt.Errorf("cmux bootstrap receipt pid is invalid")
	}
	return BootstrapReceipt{
		Status: parts[1], PID: pid, CWD: parts[3], WindowID: parts[4], WorkspaceID: parts[5], SurfaceID: parts[6], SocketPath: parts[7],
		HostExecutable: parts[8], PromptSHA256: parts[9], MaterialSHA256: parts[10], HostArgvSHA256: parts[11],
	}, nil
}

func ValidateBootstrapReceipt(receipt BootstrapReceipt, expected BootstrapExpectation, observe func(int) (issueopscontract.NativeProcessReceipt, error)) (issueopscontract.NativeProcessReceipt, error) {
	if receipt.Status != "ok" || receipt.CWD != expected.CWD || receipt.WorkspaceID != expected.WorkspaceID ||
		receipt.SurfaceID != expected.SurfaceID || receipt.SocketPath != expected.SocketPath ||
		(receipt.WindowID != "" && receipt.WindowID != expected.WindowID) ||
		receipt.HostExecutable != expected.HostExecutable || receipt.PromptSHA256 != expected.PromptSHA256 ||
		receipt.MaterialSHA256 != expected.MaterialSHA256 || !validDigest(expected.HostArgvSHA256) || receipt.HostArgvSHA256 != expected.HostArgvSHA256 {
		return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux bootstrap receipt identity mismatch")
	}
	if observe == nil {
		return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux receiver process observer is unavailable")
	}
	process, err := observe(receipt.PID)
	if err != nil {
		return issueopscontract.NativeProcessReceipt{}, err
	}
	if process.PID != receipt.PID || process.StartedAt == "" || process.Executable == "" {
		return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux receiver process receipt is incomplete")
	}
	if !sameExecutableIdentity(process.Executable, expected.HostExecutable) {
		return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux receiver process executable does not match the expected native host")
	}
	return process, nil
}

func sameExecutableIdentity(observed, expected string) bool {
	if !filepath.IsAbs(observed) || filepath.Clean(observed) != observed || !filepath.IsAbs(expected) || filepath.Clean(expected) != expected {
		return false
	}
	if observed == expected {
		return true
	}
	observedResolved, observedErr := filepath.EvalSymlinks(observed)
	expectedResolved, expectedErr := filepath.EvalSymlinks(expected)
	return observedErr == nil && expectedErr == nil && observedResolved == expectedResolved
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("cmux launcher artifact root must be a private non-symlink directory")
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func splitReceipt(value []byte) []string {
	parts := strings.Split(string(value), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
