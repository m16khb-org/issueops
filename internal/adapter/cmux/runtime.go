package cmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/adapter/outbound/gitbin"
	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"
	port "issueops/internal/port/cmux"
)

type directory struct {
	path     string
	identity os.FileInfo
}

// ObserveDirectory resolves path, which must be absolute and clean, to a
// directory and records its identity so later calls can tell whether the same
// directory is still at that path.
func ObserveDirectory(path string) (port.Directory, error) {
	root, identity, err := cmuxCanonicalDirectory(path)
	if err != nil {
		return nil, err
	}
	return &directory{root, identity}, nil
}
func (d *directory) Path() string { return d.path }
func (d *directory) SamePath(candidate string) bool {
	root, identity, err := cmuxCanonicalDirectory(candidate)
	return err == nil && root == d.path && os.SameFile(d.identity, identity)
}
func (d *directory) Pin() (port.DirectoryPin, error) {
	f, err := os.Open(d.path)
	if err != nil {
		return nil, err
	}
	return &directoryPin{d, f}, nil
}

type directoryPin struct {
	initial *directory
	file    *os.File
}

func (p *directoryPin) Matches(observed port.Directory) bool {
	current, ok := observed.(*directory)
	if !ok {
		return false
	}
	opened, err := p.file.Stat()
	return err == nil && current.path == p.initial.path && os.SameFile(p.initial.identity, opened) && os.SameFile(p.initial.identity, current.identity)
}
func (p *directoryPin) Close() { _ = p.file.Close() }
func cmuxCanonicalDirectory(path string) (string, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", nil, fmt.Errorf("path is not absolute and clean")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve path: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", nil, fmt.Errorf("resolved path is not absolute")
	}
	identity, err := os.Stat(resolved)
	if err != nil || !identity.IsDir() {
		return "", nil, fmt.Errorf("path is not an available directory")
	}
	return filepath.Clean(resolved), identity, nil
}

// ValidateHostExecutable checks that path is an absolute clean path to an
// executable regular file.
func ValidateHostExecutable(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("native host executable must be an absolute clean path")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("native host executable is unavailable")
	}
	return nil
}

// GitTop returns the git toplevel directory of root, giving git three seconds
// to answer.
func GitTop(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, gitbin.Path(), "-C", root, "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(output)), err
}

// AwaitBootstrapReceipt polls the receipt file of prepared every 25ms for up
// to three seconds until the receiver process has written a valid receipt that
// matches expected and is no longer a bare shell. observe looks up a process by
// pid. It fails on an invalid final receipt, context cancellation or timeout.
func AwaitBootstrapReceipt(ctx context.Context, prepared cmuxcontract.PreparedLauncher, expected cmuxcontract.BootstrapExpectation, observe func(int) (issueopscontract.NativeProcessReceipt, error)) (issueopscontract.NativeProcessReceipt, error) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := readBootstrapReceipt(prepared.ReceiptPath)
		if err == nil {
			process, validationErr := validateBootstrapReceipt(receipt, expected, observe)
			if validationErr == nil && !cmuxShellProcess(process.Executable) {
				return process, nil
			}
			if validationErr != nil && receipt.Status != "ok" {
				return issueopscontract.NativeProcessReceipt{}, validationErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return issueopscontract.NativeProcessReceipt{}, err
		}
		select {
		case <-ctx.Done():
			return issueopscontract.NativeProcessReceipt{}, ctx.Err()
		case <-timer.C:
			return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux receiver bootstrap receipt timed out")
		case <-ticker.C:
		}
	}
}

func cmuxShellProcess(executable string) bool {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(executable)), ".exe")
	return base == "sh" || base == "bash" || base == "zsh" || base == "dash"
}

// SamePath reports whether left and right resolve, through symlinks, to the
// same path. Either failing to resolve counts as different.
func SamePath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(filepath.Clean(left))
	rightResolved, rightErr := filepath.EvalSymlinks(filepath.Clean(right))
	return leftErr == nil && rightErr == nil && leftResolved == rightResolved
}
