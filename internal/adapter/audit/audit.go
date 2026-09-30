package audit

import (
	"encoding/json"
	"fmt"
	auditcontract "issueops/internal/contract/audit"
	"os"
	"path/filepath"
	"time"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type CommandWriter struct {
	Filename     string
	ResolveError error
}

func NewCommandWriter() CommandWriter {
	path, err := commandAuditLogPath()
	return CommandWriter{Filename: path, ResolveError: err}
}

func (writer CommandWriter) Path() (string, error) { return writer.Filename, writer.ResolveError }

func (CommandWriter) Append(path string, record auditcontract.CommandAuditRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func commandAuditLogPath() (string, error) {
	if path := os.Getenv("ISSUEOPS_AUDIT_LOG"); path != "" {
		return filepath.Abs(path)
	}
	dir := os.Getenv("ISSUEOPS_STATE_DIR")
	if dir == "" {
		base, err := os.UserHomeDir()
		if err != nil || base == "" {
			return "", fmt.Errorf("resolve home for audit log: %w", err)
		}
		dir = filepath.Join(base, ".local", "state", "issueops")
	}
	return filepath.Join(dir, "audit", "command-policy.jsonl"), nil
}
