//go:build unix

package issueops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"issueops/internal/contract/issueops"
)

// psUTC makes ps render lstart in UTC, parsed back with time.UTC. Rendering in
// the caller's zone and parsing with Go's time.Local broke on a POSIX TZ such
// as KST-9: libc applied it, Go fell back to UTC, and the receipt moved by
// nine hours, so another session saw a live holder as a different process.
const psUTC = "TZ=UTC0"

// processTableArguments returns one `ps -o args=` row per process.
func processTableArguments(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "ps", "-axww", "-o", "args=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	return string(out), err
}

// ObserveNativeProcessReceipt 함수는 PID 재사용을 방지하는 데 쓰는 운영체제
// identity를 읽는다. 호출자는 자체 보고한 시간이 아니라 이 receipt를
// 그대로 영속화한다.
func ObserveNativeProcessReceipt(pid int) (issueops.NativeProcessReceipt, error) {
	if pid <= 0 {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	alive, err := nativePIDAlive(pid)
	if err != nil {
		return issueops.NativeProcessReceipt{}, err
	}
	if !alive {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("native process pid %d is not live", pid)
	}
	// One ps call for both fields: two calls cost a spawn more (next checks the
	// holder on every call) and could straddle a PID reuse. The row splits like
	// a snapshot row, so the receipt compares equal to whoami's.
	row, err := processField(pid, "lstart=,comm=")
	if err != nil {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("read native process identity: %w", err)
	}
	fields := strings.Fields(row)
	if len(fields) < 6 {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("native process executable identity is empty")
	}
	started, err := parseNativeProcessStart(strings.Join(fields[:5], " "))
	if err != nil {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("parse native process start identity: %w", err)
	}
	executable := strings.Join(fields[5:], " ")
	return issueops.NativeProcessReceipt{
		PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
	}, nil
}
func observeNativeProcessSnapshot() (map[int]nativeProcessSnapshotEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-ww", "-axo", "pid=,ppid=,lstart=,comm=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", psUTC)
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("read native process ancestry: %w", err)
	}
	return parseNativeProcessSnapshot(string(output))
}
func nativePIDAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = process.Signal(syscall.Signal(0))
	switch {
	case err == nil, errors.Is(err, os.ErrPermission):
		return true, nil
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

func processField(pid int, field string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", field)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", psUTC)
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("ps returned an empty %s field", field)
	}
	return value, nil
}

// nativeProcessArguments reads `lstart` and the full `args` in one ps call and
// returns the whitespace-split argv after the five lstart fields.
func nativeProcessArguments(receipt issueops.NativeProcessReceipt) ([]string, error) {
	if receipt.PID <= 0 {
		return nil, fmt.Errorf("native process pid must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-ww", "-p", strconv.Itoa(receipt.PID), "-o", "lstart=,args=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", psUTC)
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("read native process arguments: %w", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 6 {
		return nil, fmt.Errorf("native process arguments are unavailable for pid %d", receipt.PID)
	}
	started, err := parseNativeProcessStart(strings.Join(fields[:5], " "))
	if err != nil {
		return nil, fmt.Errorf("parse native process start identity: %w", err)
	}
	if started.UTC().Format(time.RFC3339) != receipt.StartedAt {
		return nil, fmt.Errorf("native process identity does not match live PID %d", receipt.PID)
	}
	return fields[5:], nil
}

// inspectWorkspaceProcessesWithin은 lsof 상한을 호출자가 정한다. cleanup은 부하가
// 큰 머신에서도 관측 실패로 fail-closed되지 않도록 lease 경로보다 긴 상한을 쓴다.
func inspectWorkspaceProcessesWithin(root string, excluded map[int]bool, timeout time.Duration) ([]workspaceProcess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nPw", "-Fpcfna")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !(ok && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "") {
			return nil, fmt.Errorf("lsof workspace inventory: %s", strings.TrimSpace(stderr.String()))
		}
	}
	// probe가 띄운 lsof는 호출자의 cwd를 상속하므로 워크트리 안에서 실행되면
	// 자기 자신을 점유자로 보고한다. 이미 종료한 프로세스라 조상 관측으로는
	// 걸러지지 않으므로 여기서 정확히 제외한다.
	if cmd.Process != nil {
		probeExcluded := make(map[int]bool, len(excluded)+1)
		for pid := range excluded {
			probeExcluded[pid] = true
		}
		probeExcluded[cmd.Process.Pid] = true
		excluded = probeExcluded
	}
	return parseWorkspaceProcessesWithPathPredicate(stdout.String(), root, excluded, pathWithinResolved)
}
