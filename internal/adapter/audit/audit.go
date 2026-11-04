// Package audit is the outbound adapter that persists audit records under the issueops state root:
// the redacted command-policy audit log (JSON lines) and the tamper-evident handoff delivery log.
// It implements the audit ports with file I/O and keeps the record formats in internal/contract/audit.
package audit

import (
	"encoding/json"
	"fmt"
	"issueops/internal/adapter/outbound/osfile"
	auditcontract "issueops/internal/contract/audit"
	"os"
	"path/filepath"
	"time"
)

// Clock is the production wall-clock implementation of the time source used by audit callers.
type Clock struct{}

// Now returns the current local time.
func (Clock) Now() time.Time { return time.Now() }

// CommandWriter appends command-policy audit records to a JSON-lines file. Filename is the resolved log
// path; ResolveError carries a failure to resolve it and is reported by Path.
type CommandWriter struct {
	Filename     string
	ResolveError error
}

// NewCommandWriter returns a CommandWriter for the default log path: ISSUEOPS_AUDIT_LOG when set, otherwise
// audit/command-policy.jsonl under ISSUEOPS_STATE_DIR or ~/.local/state/issueops. A resolution failure is
// stored and surfaced later by Path.
func NewCommandWriter() CommandWriter {
	path, err := commandAuditLogPath()
	return CommandWriter{Filename: path, ResolveError: err}
}

// Path returns the resolved audit log path, or the error that prevented resolving it.
func (writer CommandWriter) Path() (string, error) { return writer.Filename, writer.ResolveError }

// Append writes record as one JSON line to path, creating the parent directory (0700) and file (0600)
// as needed and opening it in append mode.
func (CommandWriter) Append(path string, record auditcontract.CommandAuditRecord) error {
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := osfile.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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
