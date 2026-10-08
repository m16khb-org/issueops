package install

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	statedomain "issueops/internal/domain/state"
)

// RemoveRetiredState deletes the state-root entries that removed subsystems
// left behind and reports each action as an install message. It only touches
// the exact allowlisted names directly under stateDir, never follows symlinks,
// and keeps the legacy daemon directory while that daemon still runs. Removal
// failures become messages because the install itself already committed.
func RemoveRetiredState(stateDir string, dryRun bool) []string {
	messages := []string{}
	for _, entry := range statedomain.RetiredStateEntries() {
		path := filepath.Join(stateDir, entry.Name)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			messages = append(messages, "retired state path "+path+" could not be inspected; remove it manually: "+err.Error())
			continue
		}
		if entry.IsDir && !info.IsDir() {
			messages = append(messages, "retired state path "+path+" is not a directory; remove it manually")
			continue
		}
		if !entry.IsDir && !info.Mode().IsRegular() {
			messages = append(messages, "retired state path "+path+" is not a regular file; remove it manually")
			continue
		}
		if entry.IsDir && legacyDaemonRunning(path) {
			messages = append(messages, "retired state path "+path+" is kept because the legacy daemon is still running; stop it, then run issueops update again")
			continue
		}
		if dryRun {
			messages = append(messages, "would remove retired state path "+path)
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			messages = append(messages, "retired state path "+path+" could not be removed; remove it manually: "+err.Error())
			continue
		}
		messages = append(messages, "removed retired state path "+path)
	}
	return messages
}

// legacyDaemonRunning reports whether the removed shared daemon still answers
// on its socket or its recorded PID is alive. A running daemon holds its log
// open, so deleting the directory would not free the space.
func legacyDaemonRunning(dir string) bool {
	if conn, err := net.DialTimeout("unix", filepath.Join(dir, "issueops.sock"), 200*time.Millisecond); err == nil {
		_ = conn.Close()
		return true
	}
	data, err := os.ReadFile(filepath.Join(dir, "issueops.pid"))
	if err != nil {
		return false
	}
	var record struct {
		PID int `json:"pid"`
	}
	if json.Unmarshal(data, &record) != nil || record.PID <= 0 {
		return false
	}
	process, err := os.FindProcess(record.PID)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
