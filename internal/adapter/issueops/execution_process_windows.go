//go:build windows

package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"issueops/internal/adapter/outbound/osprocess"
	"issueops/internal/contract/issueops"
)

// ObserveNativeProcessReceipt는 PID 재사용을 방지하는 데 쓰는 운영체제
// identity를 읽는다. Windows는 ps lstart와 같은 초 단위 생성 시각과 전체
// image 경로를 쓴다.
func ObserveNativeProcessReceipt(pid int) (issueops.NativeProcessReceipt, error) {
	if pid <= 0 {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	if !osprocess.Alive(pid) {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("native process pid %d is not live", pid)
	}
	started, executable, err := osprocess.Identity(pid)
	if err != nil {
		return issueops.NativeProcessReceipt{}, fmt.Errorf("read native process identity: %w", err)
	}
	return issueops.NativeProcessReceipt{PID: pid, StartedAt: started.Format(time.RFC3339), Executable: executable}, nil
}

// observeNativeProcessSnapshot omits processes whose identity cannot be read.
// Windows keeps a dead parent's PID and may reuse it, so a parent that is
// absent or started after its child ends the chain, as PID 1 does on Unix.
func observeNativeProcessSnapshot() (map[int]nativeProcessSnapshotEntry, error) {
	entries, err := osprocess.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("read native process ancestry: %w", err)
	}
	started := make(map[int]time.Time, len(entries))
	snapshot := make(map[int]nativeProcessSnapshotEntry, len(entries))
	for _, entry := range entries {
		if entry.PID <= 0 || entry.Started.IsZero() {
			continue
		}
		started[entry.PID] = entry.Started
		snapshot[entry.PID] = nativeProcessSnapshotEntry{
			ParentPID: entry.ParentPID,
			Receipt: issueops.NativeProcessReceipt{
				PID: entry.PID, StartedAt: entry.Started.Format(time.RFC3339), Executable: entry.Executable,
			},
		}
	}
	for pid, entry := range snapshot {
		parentStarted, ok := started[entry.ParentPID]
		if !ok || entry.ParentPID == pid || parentStarted.After(started[pid]) {
			entry.ParentPID = 0
			snapshot[pid] = entry
		}
	}
	return snapshot, nil
}

// processTableArguments returns one space-joined argv row per process whose
// command line this user can read, the shape of `ps -o args=` rows.
func processTableArguments(ctx context.Context) (string, error) {
	entries, err := osprocess.Snapshot()
	if err != nil {
		return "", err
	}
	var rows strings.Builder
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		line, err := osprocess.CommandLine(entry.PID)
		if err != nil || line == "" {
			continue
		}
		if args, err := windows.DecomposeCommandLine(line); err == nil {
			rows.WriteString(strings.Join(args, " "))
			rows.WriteByte('\n')
		}
	}
	return rows.String(), nil
}

func nativePIDAlive(pid int) (bool, error) { return osprocess.Alive(pid), nil }

// nativeProcessArguments returns the live argv of the receipt's process while
// its start identity still matches the receipt.
func nativeProcessArguments(receipt issueops.NativeProcessReceipt) ([]string, error) {
	if receipt.PID <= 0 {
		return nil, fmt.Errorf("native process pid must be positive")
	}
	line, err := osprocess.CommandLine(receipt.PID)
	if err != nil {
		return nil, fmt.Errorf("read native process arguments: %w", err)
	}
	started, _, err := osprocess.Identity(receipt.PID)
	if err != nil {
		return nil, fmt.Errorf("read native process arguments: %w", err)
	}
	if started.Format(time.RFC3339) != receipt.StartedAt {
		return nil, fmt.Errorf("native process identity does not match live PID %d", receipt.PID)
	}
	args, err := windows.DecomposeCommandLine(line)
	if err != nil || len(args) == 0 {
		return nil, fmt.Errorf("native process arguments are unavailable for pid %d", receipt.PID)
	}
	return args, nil
}

// inspectWorkspaceProcessesWithin reports processes whose working directory
// is inside root, the lsof "cwd" occupancy. Windows itself refuses to remove a
// file another process holds open without FILE_SHARE_DELETE, so open write
// handles are not enumerated. Processes this user cannot read are skipped, as
// lsof skips them. The snapshot does not spawn, so timeout is not needed.
func inspectWorkspaceProcessesWithin(root string, excluded map[int]bool, _ time.Duration) ([]workspaceProcess, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	entries, err := osprocess.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("workspace process inventory: %w", err)
	}
	result := []workspaceProcess{}
	for _, entry := range entries {
		if entry.PID <= 0 || excluded[entry.PID] {
			continue
		}
		cwd, err := osprocess.CurrentDirectory(entry.PID)
		if err != nil || cwd == "" || !pathWithinResolved(cwd, root) {
			continue
		}
		result = append(result, workspaceProcess{
			PID: entry.PID, Command: filepath.Base(entry.Executable), FD: "cwd", Path: filepath.Clean(cwd),
		})
	}
	return result, nil
}
