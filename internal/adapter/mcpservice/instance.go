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
	HeaderPID     = "Issueops-Mcp-Pid"
	HeaderBuildID = "Issueops-Mcp-Build-Id"
)

var ErrInstanceConflict = errors.New("another issueops mcp http instance holds the service lock (conflict)")

type Record struct {
	SchemaVersion int    `json:"schema_version"`
	PID           int    `json:"pid"`
	StartedAt     string `json:"started_at"`
	Executable    string `json:"executable"`
	BuildID       string `json:"build_id"`
}

type Instance struct {
	lock       *os.File
	recordPath string
	record     Record
}

func httpDir(stateDir string) string { return filepath.Join(stateDir, httpStateDirName) }

// AcquireInstance takes the non-blocking OS lock. It never waits: a held lock
// is a conflict, not something to retry on another port.
func AcquireInstance(stateDir string) (*Instance, error) {
	dir := httpDir(stateDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLock(lock); err != nil {
		_ = lock.Close()
		if errors.Is(err, errLockHeld) {
			return nil, ErrInstanceConflict
		}
		return nil, fmt.Errorf("lock mcp http instance: %w", err)
	}
	return &Instance{lock: lock, recordPath: filepath.Join(dir, recordFileName)}, nil
}

func (instance *Instance) Publish(inspect func(int) (processidentity.Identity, error), executable string) (Record, error) {
	pid := os.Getpid()
	identity, err := inspect(pid)
	if err != nil {
		return Record{}, fmt.Errorf("inspect mcp http process: %w", err)
	}
	buildID, err := BuildID(executable)
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
			w.Header().Set(HeaderPID, w.pid)
			w.Header().Set(HeaderBuildID, w.buildID)
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

func BuildID(path string) (string, error) {
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
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
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
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
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
