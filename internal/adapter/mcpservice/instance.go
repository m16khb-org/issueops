// Package mcpservice is the outbound adapter that runs the IssueOps MCP HTTP
// server as a per-user background service under launchd (darwin) or systemd
// --user (linux), and guards it with a single-instance lock and an identity
// record so status, start, and stop can prove which process answers.
package mcpservice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/contract/processidentity"
)

const (
	httpStateDirName = "mcp-http"
	lockFileName     = "server.lock"
	recordFileName   = "server.json"
	bearerFileName   = "bearer"
	recordSchema     = 1

	// Identity headers ride only on successful (authenticated) responses so a
	// readiness probe can prove which process answers on the fixed port.
	headerPID     = "Issueops-Mcp-Pid"
	headerBuildID = "Issueops-Mcp-Build-Id"
)

var errInstanceConflict = errors.New("another issueops mcp http instance holds the service lock (conflict)")

// Record is the identity of the running MCP HTTP server, persisted as server.json
// in the state directory: process id and start time, executable, and the build id
// of that executable.
type Record struct {
	SchemaVersion int    `json:"schema_version"`
	PID           int    `json:"pid"`
	StartedAt     string `json:"started_at"`
	Executable    string `json:"executable"`
	BuildID       string `json:"build_id"`
}

// Instance is a held single-instance lock for the MCP HTTP server together with
// the identity record it published. Release gives both up.
type Instance struct {
	lock       *os.File
	recordPath string
	record     Record
}

func httpDir(stateDir string) string { return filepath.Join(stateDir, httpStateDirName) }

// serviceLogPath is the file the launchd and systemd units append the server's
// stdout and stderr to.
func serviceLogPath(stateDir string) string { return filepath.Join(httpDir(stateDir), "server.log") }

// AcquireInstance takes the non-blocking OS lock. It never waits: a held lock
// is a conflict, not something to retry on another port.
func AcquireInstance(stateDir string) (*Instance, error) {
	dir := httpDir(stateDir)
	if err := osfile.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLock(lock); err != nil {
		_ = lock.Close()
		if errors.Is(err, errLockHeld) {
			return nil, errInstanceConflict
		}
		return nil, fmt.Errorf("lock mcp http instance: %w", err)
	}
	return &Instance{lock: lock, recordPath: filepath.Join(dir, recordFileName)}, nil
}

// Publish inspects the current process, computes the build id of executable, and
// writes the resulting Record to the state directory. It returns the record and
// remembers it for Release.
func (instance *Instance) Publish(inspect func(int) (processidentity.Identity, error), executable string) (Record, error) {
	pid := os.Getpid()
	identity, err := inspect(pid)
	if err != nil {
		return Record{}, fmt.Errorf("inspect mcp http process: %w", err)
	}
	buildID, err := buildID(executable)
	if err != nil {
		return Record{}, fmt.Errorf("mcp http build id: %w", err)
	}
	record := Record{SchemaVersion: recordSchema, PID: pid, StartedAt: identity.StartTime, Executable: identity.Executable, BuildID: buildID}
	if err := writeRecord(instance.recordPath, record); err != nil {
		return Record{}, err
	}
	instance.record = record
	return record, nil
}

// Release removes the identity record if it still belongs to this process and
// then drops the instance lock.
func (instance *Instance) Release() error {
	if instance.record.PID != 0 {
		if current, err := readRecord(instance.recordPath); err == nil && current.PID == instance.record.PID {
			_ = os.Remove(instance.recordPath)
		}
	}
	return instance.lock.Close()
}

// IdentityHandler adds the pid/build headers to 2xx responses only, so a
// rejected (401/403) request learns nothing about the serving process.
func IdentityHandler(next http.Handler, record Record) http.Handler {
	pid := strconv.Itoa(record.PID)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&identityWriter{ResponseWriter: w, pid: pid, buildID: record.BuildID}, r)
	})
}

type identityWriter struct {
	http.ResponseWriter
	pid, buildID string
	wrote        bool
}

func (w *identityWriter) WriteHeader(status int) {
	if !w.wrote {
		w.wrote = true
		if status >= 200 && status < 300 {
			w.Header().Set(headerPID, w.pid)
			w.Header().Set(headerBuildID, w.buildID)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *identityWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *identityWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *identityWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func buildID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readRecord(path string) (Record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, fmt.Errorf("decode mcp http instance record: %w", err)
	}
	if record.SchemaVersion != recordSchema || record.PID <= 0 || record.StartedAt == "" || record.Executable == "" || record.BuildID == "" {
		return Record{}, fmt.Errorf("invalid mcp http instance record")
	}
	return record, nil
}

func writeRecord(path string, record Record) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := osfile.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ReadBearer reads the installed bearer without creating one. A missing file
// returns "" so dry-run planning stays write-free.
func ReadBearer(stateDir string) (string, error) {
	path := filepath.Join(httpDir(stateDir), bearerFileName)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || osfile.Perm(path, info)&0o077 != 0 {
		return "", fmt.Errorf("mcp http bearer file must be an owner-only regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(trimSpace(raw)), nil
}

func trimSpace(raw []byte) []byte {
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\n' || raw[start] == '\r' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\n' || raw[end-1] == '\r' || raw[end-1] == '\t') {
		end--
	}
	return raw[start:end]
}
