//go:build windows

package update

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"

	"issueops/internal/adapter/outbound/gitbin"
	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/adapter/outbound/osprocess"
)

var (
	activationTransitionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	activationDigestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Install is the Windows counterpart of scripts/install-native.sh, which needs
// bash, python3, and an os.replace over the binary. It runs the same steps:
// build a staged candidate, preflight it with --dry-run, begin the native
// activation, swap it in as bin\issueops.exe, seal with the canonical binary,
// and abort the transition if anything after begin fails.
//
// Windows cannot replace an executable that a host's stdio MCP server is
// running, but it can rename it, so the swap first moves the old binary aside
// to bin\.issueops.retired-*.exe and removes retired copies once unused.
func (r Runtime) Install(root string, args []string) error {
	run := nativeInstallRun{runtime: r, root: stableNativeRoot(root)}
	return run.install(args)
}

type nativeInstallRun struct {
	runtime        Runtime
	root           string
	installArgs    []string
	activationArgs []string
	transition     string
	digest         string
	staged         string
	begun          bool
	committed      bool
}

func (run *nativeInstallRun) install(args []string) (err error) {
	dryRun, skipBuild, transport, err := run.parseArgs(args)
	if err != nil {
		return err
	}
	bin := filepath.Join(run.root, "bin", osfile.ExecutableName("issueops"))
	run.removeRetired()
	defer func() { err = errors.Join(err, run.cleanup(bin)) }()
	switch {
	case dryRun && skipBuild:
		run.log("dry-run: would leave existing issueops binary unchanged")
	case dryRun && fileExists(bin):
		run.log("dry-run: would update issueops binary from current checkout")
	case dryRun:
		run.log("dry-run: would build issueops binary")
	case skipBuild:
		run.log("skip-build: leaving existing harness binary unchanged")
	default:
		if err := run.buildAndSwap(bin, transport); err != nil {
			return err
		}
	}
	if !fileExists(bin) {
		if dryRun {
			run.log("dry-run: binary missing; skipping install plan because " + bin + " does not exist yet")
			return nil
		}
		return fmt.Errorf("issueops binary missing after build: %s", bin)
	}
	if dryRun {
		return run.exec(nil, run.runtime.Output, bin, append([]string{"install"}, run.installArgs...)...)
	}
	if !run.begun {
		if err := run.preflight(bin); err != nil {
			return err
		}
		if err := run.begin(bin); err != nil {
			return err
		}
	}
	return run.seal(bin)
}

func (run *nativeInstallRun) parseArgs(args []string) (dryRun, skipBuild bool, transport string, err error) {
	skipBuild = isTruthy(os.Getenv("ISSUEOPS_SKIP_BUILD"))
	for _, arg := range args {
		switch {
		case arg == "--skip-build":
			skipBuild = true
		case arg == "--dry-run":
			dryRun = true
			run.installArgs = append(run.installArgs, arg)
		case arg == "--adopt-command-file":
			run.installArgs = append(run.installArgs, arg)
			run.activationArgs = append(run.activationArgs, arg)
		case strings.HasPrefix(arg, "--mcp-transport="):
			transport = strings.TrimPrefix(arg, "--mcp-transport=")
			run.installArgs = append(run.installArgs, arg)
			run.activationArgs = append(run.activationArgs, arg)
		case arg == "--mcp-transport":
			return false, false, "", fmt.Errorf("use --mcp-transport=http or --mcp-transport=stdio")
		default:
			run.installArgs = append(run.installArgs, arg)
		}
	}
	if transport == "" {
		transport = "stdio"
	}
	return dryRun, skipBuild, transport, nil
}

func (run *nativeInstallRun) buildAndSwap(bin, transport string) error {
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	run.staged = filepath.Join(filepath.Dir(bin), ".issueops.activate-"+suffix+".exe")
	if fileExists(bin) {
		run.log("staging issueops binary update from current checkout")
	} else {
		run.log("staging initial issueops binary from current checkout")
	}
	if err := run.exec(nil, run.runtime.Diagnostics, "go", "build", "-o", run.staged, "./cmd/issueops"); err != nil {
		return err
	}
	if err := run.exec(nil, io.Discard, run.staged, "version"); err != nil {
		return err
	}
	if err := run.preflight(run.staged); err != nil {
		return err
	}
	if err := run.begin(run.staged); err != nil {
		return err
	}
	if transport == "http" {
		run.log("stopping shared MCP service before binary replacement")
		if err := run.mcpService(run.staged, "stop"); err != nil {
			return err
		}
	}
	if err := swapBinary(run.staged, bin); err != nil {
		return err
	}
	run.staged = ""
	if err := run.exec(nil, io.Discard, bin, "version"); err != nil {
		return err
	}
	if transport == "http" {
		run.log("starting shared MCP service from the new binary")
		return run.mcpService(bin, "start")
	}
	return nil
}

func (run *nativeInstallRun) preflight(executable string) error {
	return run.exec(nil, io.Discard, executable, append(append([]string{"install"}, run.installArgs...), "--dry-run")...)
}

func (run *nativeInstallRun) begin(executable string) error {
	var receipt bytes.Buffer
	args := append([]string{"install", "--path-mode=skip", "--json"}, run.activationArgs...)
	if err := run.exec([]string{"ISSUEOPS_NATIVE_ACTIVATION_STEP=begin"}, &receipt, executable, args...); err != nil {
		// Under --json the install error is the JSON document on stdout.
		_, _ = run.runtime.Diagnostics.Write(receipt.Bytes())
		return err
	}
	var value struct {
		TransitionID string `json:"transition_id"`
		BinarySHA256 string `json:"binary_sha256"`
	}
	if err := json.Unmarshal(receipt.Bytes(), &value); err != nil ||
		!activationTransitionPattern.MatchString(value.TransitionID) || !activationDigestPattern.MatchString(value.BinarySHA256) {
		return fmt.Errorf("invalid native activation begin receipt")
	}
	run.transition, run.digest, run.begun = value.TransitionID, value.BinarySHA256, true
	return nil
}

func (run *nativeInstallRun) seal(bin string) error {
	var receipt bytes.Buffer
	env := []string{"ISSUEOPS_NATIVE_ACTIVATION_STEP=seal", "ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID=" + run.transition}
	sealErr := run.exec(env, &receipt, bin, append(append([]string{"install"}, run.installArgs...), "--json")...)
	_, _ = run.runtime.Output.Write(receipt.Bytes())
	if sealErr != nil {
		return sealErr
	}
	var value struct {
		Committed    bool   `json:"committed"`
		TransitionID string `json:"transition_id"`
	}
	if err := json.Unmarshal(receipt.Bytes(), &value); err != nil || !value.Committed || value.TransitionID != run.transition {
		return fmt.Errorf("native activation seal did not commit transition %s", run.transition)
	}
	run.committed = true
	return nil
}

// cleanup aborts a begun, uncommitted transition with whichever binary still
// has the exact candidate digest, then removes the staged candidate.
func (run *nativeInstallRun) cleanup(bin string) error {
	if run.begun && !run.committed {
		abortBin := ""
		for _, candidate := range []string{run.staged, bin} {
			if candidate != "" && fileSHA256(candidate) == run.digest {
				abortBin = candidate
				break
			}
		}
		if abortBin == "" {
			run.log("native activation abort skipped: no exact candidate binary remains; transition remains pending for recovery")
		} else {
			env := []string{"ISSUEOPS_NATIVE_ACTIVATION_STEP=abort", "ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID=" + run.transition}
			if err := run.exec(env, io.Discard, abortBin, "install", "--path-mode=skip", "--json"); err != nil {
				run.log("native activation abort failed; transition remains pending for recovery")
			}
		}
	}
	if run.staged != "" {
		if err := os.Remove(run.staged); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (run *nativeInstallRun) mcpService(executable, action string) error {
	var output bytes.Buffer
	if err := run.exec(nil, &output, executable, "mcp", "service", action, "--json"); err != nil {
		_, _ = run.runtime.Diagnostics.Write(output.Bytes())
		run.log("shared MCP service " + action + " failed")
		return err
	}
	return nil
}

// removeRetired deletes binaries earlier swaps moved aside; one a process
// still runs stays until a later update.
func (run *nativeInstallRun) removeRetired() {
	matches, _ := filepath.Glob(filepath.Join(run.root, "bin", ".issueops.retired-*.exe"))
	for _, path := range matches {
		_ = os.Remove(path)
	}
}

func (run *nativeInstallRun) exec(env []string, stdout io.Writer, executable string, args ...string) error {
	command := exec.Command(executable, args...)
	command.Dir = run.root
	command.Env = append(append(append([]string(nil), run.runtime.Environment...), "ISSUEOPS_ROOT="+run.root), env...)
	command.Stdin = run.runtime.Input
	command.Stdout = stdout
	command.Stderr = run.runtime.Diagnostics
	return command.Run()
}

func (run *nativeInstallRun) log(message string) {
	// A nil Diagnostics discards output, as it does for exec.Cmd and the Unix runtime.
	if run.runtime.Diagnostics == nil {
		return
	}
	fmt.Fprintf(run.runtime.Diagnostics, "[issueops] %s\n", message)
}

// swapBinary moves staged onto bin. A running bin cannot be overwritten or
// deleted on Windows but can be renamed, so it is moved aside first and put
// back if the staged binary cannot take its name.
func swapBinary(staged, bin string) error {
	if file, err := os.OpenFile(staged, os.O_RDWR, 0); err == nil {
		syncErr := file.Sync()
		if closeErr := file.Close(); syncErr == nil {
			syncErr = closeErr
		}
		if syncErr != nil {
			return syncErr
		}
	} else {
		return err
	}
	retired := ""
	if fileExists(bin) {
		suffix, err := randomSuffix()
		if err != nil {
			return err
		}
		retired = filepath.Join(filepath.Dir(bin), ".issueops.retired-"+suffix+".exe")
		if err := os.Rename(bin, retired); err != nil {
			return fmt.Errorf("move running issueops binary aside: %w", err)
		}
	}
	if err := os.Rename(staged, bin); err != nil {
		if retired != "" {
			err = errors.Join(err, os.Rename(retired, bin))
		}
		return fmt.Errorf("install staged issueops binary: %w", err)
	}
	if retired != "" {
		_ = os.Remove(retired)
	}
	return nil
}

// stableNativeRoot maps a linked worktree to the checkout owning the common
// .git directory, as install-native.sh does.
func stableNativeRoot(root string) string {
	out, err := exec.Command(gitbin.Path(), "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return root
	}
	common := filepath.Clean(strings.TrimSpace(string(out)))
	if info, err := os.Lstat(common); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Base(common) != ".git" {
		return root
	}
	return filepath.Dir(common)
}

// processRows renders the process table as `ps -axo pid=,ppid=,command=` rows
// from one snapshot and each process's command line.
func (r Runtime) processRows() ([]byte, error) {
	entries, err := osprocess.Snapshot()
	if err != nil {
		return nil, err
	}
	var rows bytes.Buffer
	for _, entry := range entries {
		line, err := osprocess.CommandLine(entry.PID)
		if err != nil || line == "" {
			continue
		}
		args, err := windows.DecomposeCommandLine(line)
		if err != nil || len(args) == 0 {
			continue
		}
		rows.WriteString(strconv.Itoa(entry.PID) + " " + strconv.Itoa(entry.ParentPID) + " " + strings.Join(args, " ") + "\n")
	}
	return rows.Bytes(), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileSHA256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func randomSuffix() (string, error) {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func isTruthy(value string) bool {
	switch value {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
