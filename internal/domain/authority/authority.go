// Package authority holds the pure rules of native-issued caller capabilities.
// A grant proves which native session is calling; it never grants lease ownership.
package authority

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	contract "issueops/internal/contract/authority"
)

const TTL = 12 * time.Hour

const (
	scopeKindGit       = "git"
	scopeKindDirectory = "directory"
)

// Error is a public authority failure. Reason never carries a credential or its path.
type Error struct {
	Code   string
	Reason string
}

func (e *Error) Error() string { return e.Code + ": " + e.Reason }

func Required(reason string) error {
	return &Error{Code: contract.CodeRequired, Reason: reason}
}

func Invalid(reason string) error {
	return &Error{Code: contract.CodeInvalid, Reason: reason}
}

// NormalizeIdentity trims the identity fields and drops any ancestry: a grant
// stores who was verified, never the process tree observed at issue time.
func NormalizeIdentity(actor contract.NativeActor) contract.NativeActor {
	identity := contract.NativeActor{
		Host:      strings.ToLower(strings.TrimSpace(actor.Host)),
		SessionID: strings.TrimSpace(actor.SessionID),
		AgentID:   strings.TrimSpace(actor.AgentID),
	}
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		process.StartedAt = strings.TrimSpace(process.StartedAt)
		process.Executable = strings.TrimSpace(process.Executable)
		identity.SessionProcess = &process
	}
	return identity
}

// Key is SHA-256 over the scope kind, its anchor (Git common-dir, or the source
// root outside Git), and the NUL-separated normalized host/session/agent.
func Key(scope contract.Scope, actor contract.NativeActor) string {
	kind, anchor := scopeKindGit, scope.GitCommonDir
	if anchor == "" {
		kind, anchor = scopeKindDirectory, scope.SourceRoot
	}
	identity := NormalizeIdentity(actor)
	sum := sha256.Sum256([]byte(strings.Join([]string{kind, anchor, identity.Host, identity.SessionID, identity.AgentID}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func ValidKey(value string) bool { return isLowerHex(value, sha256.Size*2) }

func ComposeToken(key, secret string) string { return key + "." + secret }

func TokenKey(token string) (string, bool) {
	key, secret, ok := strings.Cut(token, ".")
	if !ok || !ValidKey(key) || secret == "" || strings.ContainsAny(secret, ".\x00\r\n\t ") {
		return "", false
	}
	return key, true
}

func TokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewRecord(key string, scope contract.Scope, actor contract.NativeActor, tokenDigest string, issuedAt time.Time) contract.Record {
	issuedAt = issuedAt.UTC()
	return contract.Record{
		SchemaVersion: contract.SchemaVersion,
		Key:           key,
		SourceRoot:    scope.SourceRoot,
		GitCommonDir:  scope.GitCommonDir,
		Actor:         NormalizeIdentity(actor),
		TokenSHA256:   tokenDigest,
		IssuedAt:      issuedAt.Format(time.RFC3339Nano),
		ExpiresAt:     issuedAt.Add(TTL).Format(time.RFC3339Nano),
	}
}

// EncodeRecord never persists process ancestry.
func EncodeRecord(record contract.Record) ([]byte, error) {
	record.Actor = NormalizeIdentity(record.Actor)
	if err := validateRecord(record, record.Key); err != nil {
		return nil, err
	}
	return json.Marshal(record)
}

// DecodeRecord fails closed with the shared invalid-state error for malformed,
// missing, zero, future, or foreign-key grants.
func DecodeRecord(data []byte, key string) (contract.Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record contract.Record
	if err := decoder.Decode(&record); err != nil || decoder.More() {
		return contract.Record{}, contract.ErrInvalidState
	}
	if err := validateRecord(record, key); err != nil {
		return contract.Record{}, err
	}
	return record, nil
}

func validateRecord(record contract.Record, key string) error {
	if record.SchemaVersion != contract.SchemaVersion || !ValidKey(record.Key) || record.Key != key ||
		!isLowerHex(record.TokenSHA256, sha256.Size*2) || !filepath.IsAbs(record.SourceRoot) ||
		(record.GitCommonDir != "" && !filepath.IsAbs(record.GitCommonDir)) {
		return contract.ErrInvalidState
	}
	if _, err := time.Parse(time.RFC3339Nano, record.IssuedAt); err != nil {
		return contract.ErrInvalidState
	}
	if _, err := time.Parse(time.RFC3339Nano, record.ExpiresAt); err != nil {
		return contract.ErrInvalidState
	}
	actor := record.Actor
	if (actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" && actor.Host != "omp") || actor.SessionID == "" ||
		actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" ||
		actor.SessionProcess.Executable == "" || len(actor.ProcessAncestry) != 0 {
		return contract.ErrInvalidState
	}
	return nil
}

// Check validates one credential against its stored grant at the given instant.
// Process liveness is the caller's observation and is checked separately.
func Check(record contract.Record, token string, scope contract.Scope, now time.Time) error {
	key, ok := TokenKey(token)
	if !ok || key != record.Key {
		return Invalid("authority credential is malformed")
	}
	if subtle.ConstantTimeCompare([]byte(TokenDigest(token)), []byte(record.TokenSHA256)) != 1 {
		return Invalid("authority credential was revoked or never issued")
	}
	expires, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
	if err != nil {
		return contract.ErrInvalidState
	}
	if !now.Before(expires) {
		return Invalid("authority credential expired")
	}
	if !ScopeAllows(record, scope) {
		return Invalid("request workspace is outside the authorized repository scope")
	}
	return nil
}

// ScopeAllows admits every worktree sharing the Git common-dir, or only paths
// inside the bounded source root for a non-Git grant.
func ScopeAllows(record contract.Record, scope contract.Scope) bool {
	if record.GitCommonDir != "" {
		return scope.GitCommonDir == record.GitCommonDir
	}
	return scope.GitCommonDir == "" && within(scope.SourceRoot, record.SourceRoot) &&
		within(scope.WorkspaceRoot, record.SourceRoot) && within(scope.CWD, record.SourceRoot)
}

// MatchIdentity checks a caller-supplied actor against the verified grant. An
// empty supplied actor defers entirely to the grant.
func MatchIdentity(granted, supplied contract.NativeActor) error {
	supplied = NormalizeIdentity(supplied)
	if supplied.Host == "" && supplied.SessionID == "" && supplied.AgentID == "" && supplied.SessionProcess == nil {
		return nil
	}
	granted = NormalizeIdentity(granted)
	if supplied.Host != granted.Host || supplied.SessionID != granted.SessionID || supplied.AgentID != granted.AgentID ||
		(supplied.SessionProcess != nil && (granted.SessionProcess == nil || *supplied.SessionProcess != *granted.SessionProcess)) {
		return Invalid("supplied actor does not match the authorized caller")
	}
	return nil
}

func within(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
