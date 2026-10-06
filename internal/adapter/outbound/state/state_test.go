package state

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	statecontract "issueops/internal/contract/state"
)

func TestStateReadUsesGenericInvalidStateAndPreservesAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	for name, raw := range map[string]string{
		"malformed":      `{`,
		"missing-schema": `{"key":"missing-schema","content":"x","bytes":1}`,
		"future-schema":  `{"schema_version":2,"key":"future-schema","content":"x","bytes":1}`,
		"key-mismatch":   `{"schema_version":1,"key":"other","content":"x","bytes":1}`,
		"byte-mismatch":  `{"schema_version":1,"key":"byte-mismatch","content":"x","bytes":2}`,
		"legacy-field":   `{"schema_version":1,"key":"legacy-field","content":"x","updated_at":"2000-01-01T00:00:00Z","bytes":1,"legacy_field":"x"}`,
		"trailing-json":  `{"schema_version":1,"key":"trailing-json","content":"x","updated_at":"2000-01-01T00:00:00Z","bytes":1}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			writeRawStateRow(t, dir, name, raw)
			_, err := NewService().Read(name)
			if !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
				t.Fatalf("error=%v", err)
			}
		})
	}
	_, err := NewService().Read("absent")
	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("absent state identity drift: %v", err)
	}
}

func TestWithKeyLockPropagatesActiveRoot(t *testing.T) {
	dir := t.TempDir()
	err := WithKeyLock(context.Background(), dir, "outer", func(spanCtx context.Context) error {
		db, err := sqlstore.Open(dir)
		if err != nil {
			return err
		}
		return db.WithSpan(spanCtx, func(context.Context) error { return nil })
	})
	var nested *sqlstore.NestedSpanError
	if !errors.As(err, &nested) {
		t.Fatalf("expected NestedSpanError, got %v", err)
	}
}

func TestWriteStateRecordRejectsKeyMismatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	// A caller-built record whose Key diverges from the write key would persist a
	// record NewService().Read later rejects; WriteStateRecord must reject it up front.
	if _, err := WriteStateRecord(context.Background(), dir, "foo", statecontract.RecordEnvelope{Key: "bar", SchemaVersion: statecontract.SchemaVersion, Content: "x", Bytes: 1}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected key-mismatch error, got %v", err)
	}
	// Matching key (or empty Key) is accepted and round-trips.
	if _, err := WriteStateRecord(context.Background(), dir, "foo", statecontract.RecordEnvelope{Key: "foo", SchemaVersion: statecontract.SchemaVersion, Content: "x", Bytes: 1}); err != nil {
		t.Fatalf("matching key should write: %v", err)
	}
	if read, err := NewService().Read("foo"); err != nil || read.Record.Content != "x" {
		t.Fatalf("round-trip failed: %q err=%v", read.Record.Content, err)
	}
}

func TestStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	content := "seed=42\nLore: state roundtrip\n"
	written, err := NewService().Write(context.Background(), "checkpoint-1", content)
	if err != nil {
		t.Fatalf("NewService().Write: %v", err)
	}
	if !written.OK {
		t.Fatalf("NewService().Write ok=false: %+v", written)
	}
	if written.StateDir != dir {
		t.Fatalf("StateDir=%q want %q", written.StateDir, dir)
	}
	if written.Path != filepath.Join(dir, "checkpoint-1.json") {
		t.Fatalf("Path=%q", written.Path)
	}
	if written.Record.Bytes != len([]byte(content)) {
		t.Fatalf("Bytes=%d want %d", written.Record.Bytes, len([]byte(content)))
	}
	if written.Record.SchemaVersion != statecontract.SchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", written.Record.SchemaVersion, statecontract.SchemaVersion)
	}

	read, err := NewService().Read("checkpoint-1")
	if err != nil {
		t.Fatalf("NewService().Read: %v", err)
	}
	if read.Record.Content != content {
		t.Fatalf("content=%q want %q", read.Record.Content, content)
	}

	listed, err := NewService().List()
	if err != nil {
		t.Fatalf("NewService().List: %v", err)
	}
	if len(listed.Keys) != 1 || listed.Keys[0] != "checkpoint-1" {
		t.Fatalf("Keys=%v", listed.Keys)
	}
	if len(listed.Records) != 1 || listed.Records[0].Bytes != len([]byte(content)) {
		t.Fatalf("Records=%+v", listed.Records)
	}
	if listed.Records[0].SchemaVersion != statecontract.SchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", listed.Records[0].SchemaVersion, statecontract.SchemaVersion)
	}
}

func TestStateReadMissingStoreDoesNotCreateFiles(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "missing")
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	if _, err := NewService().Read("checkpoint"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("NewService().Read missing store error=%v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("NewService().Read created files or directories: %v", entries)
	}
}

func TestStateWriteWaitsForKeyLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	locked := make(chan struct{})
	release := make(chan struct{})
	lockErr := make(chan error, 1)
	go func() {
		lockErr <- NewService().WithKeyLock(context.Background(), dir, "locked-key", func(context.Context) error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	started := make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		close(started)
		_, err := NewService().Write(context.Background(), "locked-key", "locked content")
		writeDone <- err
	}()
	<-started

	select {
	case err := <-writeDone:
		t.Fatalf("NewService().Write completed while key lock was held: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	if err := <-lockErr; err != nil {
		t.Fatalf("NewService().WithKeyLock: %v", err)
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("NewService().Write after lock release: %v", err)
	}
	read, err := NewService().Read("locked-key")
	if err != nil {
		t.Fatalf("NewService().Read: %v", err)
	}
	if read.Record.Content != "locked content" {
		t.Fatalf("content=%q", read.Record.Content)
	}
}

func TestStateRejectsPathTraversalKeys(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, key := range []string{"", "../x", "x/y", "x\\y", "x..y"} {
		if _, err := NewService().Write(context.Background(), key, "content"); err == nil {
			t.Fatalf("NewService().Write(context.Background(), %q) succeeded; want error", key)
		}
	}
}

func TestStatePruneDryRunAndConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	if _, err := NewService().Write(context.Background(), "old", "old content"); err != nil {
		t.Fatalf("NewService().Write old: %v", err)
	}
	if _, err := NewService().Write(context.Background(), "fresh", "fresh content"); err != nil {
		t.Fatalf("NewService().Write fresh: %v", err)
	}
	old, err := NewService().Read("old")
	if err != nil {
		t.Fatalf("NewService().Read old: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(old.Record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeRawStateRow(t, dir, "old", string(b)+"\n")

	dry, err := NewService().Prune(context.Background(), time.Hour, false)
	if err != nil {
		t.Fatalf("NewService().Prune dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || !containsString(dry.DeletedKeys, "old") || !containsString(dry.KeptKeys, "fresh") {
		t.Fatalf("unexpected dry-run prune result: %+v", dry)
	}
	if _, err := NewService().Read("old"); err != nil {
		t.Fatalf("dry-run removed old key: %v", err)
	}

	confirmed, err := NewService().Prune(context.Background(), time.Hour, true)
	if err != nil {
		t.Fatalf("NewService().Prune confirmed: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !containsString(confirmed.DeletedKeys, "old") {
		t.Fatalf("unexpected confirmed prune result: %+v", confirmed)
	}
	if _, err := NewService().Read("old"); err == nil {
		t.Fatalf("old key still exists after confirmed prune")
	}
	if _, err := NewService().Read("fresh"); err != nil {
		t.Fatalf("fresh key removed unexpectedly: %v", err)
	}
}

func TestStatePrunePrefixAppliesAgeAndCountOnlyToMatchingKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	write := func(key, updatedAt string) {
		t.Helper()
		if _, err := NewService().Write(context.Background(), key, "content "+key); err != nil {
			t.Fatalf("NewService().Write %s: %v", key, err)
		}
		read, err := NewService().Read(key)
		if err != nil {
			t.Fatalf("NewService().Read %s: %v", key, err)
		}
		read.Record.UpdatedAt = updatedAt
		b, err := json.MarshalIndent(read.Record, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", key, err)
		}
		writeRawStateRow(t, dir, key, string(b)+"\n")
	}

	write("external-llm-usage-old", "2000-01-01T00:00:00Z")
	write("external-llm-usage-recent-1", "2026-07-03T00:00:01Z")
	write("external-llm-usage-recent-2", "2026-07-03T00:00:02Z")
	write("external-llm-usage-recent-3", "2026-07-03T00:00:03Z")
	write("self-augment-lesson-old", "2000-01-01T00:00:00Z")

	result, err := NewService().PrunePrefix(context.Background(), "external-llm-usage-", 365*24*time.Hour, 2, true)
	if err != nil {
		t.Fatalf("NewService().PrunePrefix: %v", err)
	}
	if !result.OK || !result.Confirm || !containsString(result.DeletedKeys, "external-llm-usage-old") || !containsString(result.DeletedKeys, "external-llm-usage-recent-1") {
		t.Fatalf("unexpected prefix prune result: %+v", result)
	}
	for _, key := range []string{"external-llm-usage-recent-2", "external-llm-usage-recent-3", "self-augment-lesson-old"} {
		if _, err := NewService().Read(key); err != nil {
			t.Fatalf("%s should be kept: %v", key, err)
		}
	}
	for _, key := range []string{"external-llm-usage-old", "external-llm-usage-recent-1"} {
		if _, err := NewService().Read(key); err == nil {
			t.Fatalf("%s should be pruned", key)
		}
	}
}

func TestStatePruneRejectsInvalidMaxAge(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	if _, err := NewService().Prune(context.Background(), 0, false); err == nil {
		t.Fatalf("NewService().Prune accepted zero max age")
	}
}
