package looprun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	loopruncontract "issueops/internal/contract/looprun"
	"issueops/internal/port"
	"strings"
)

const loopBucket = "loop"

type Store struct {
	Directory    string
	OpenDatabase func(string) (StateDatabase, error)
	GetExisting  func(dir, bucket, id string) ([]byte, bool, error)
	ListExisting func(dir, bucket string) ([]string, error)
}

func (store Store) open() (StateDatabase, error) { return store.OpenDatabase(store.Directory) }

func (store Store) Read(loopID string) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	db, err := store.open()
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	data, ok, err := db.Get(loopBucket, loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if !ok {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop %s: %w", loopID, fs.ErrNotExist)
	}
	return decodeLoop(loopID, data)
}

// ReadExisting reads one existing loop without creating, repairing, or
// changing permissions on the loop store.
func (store Store) ReadExisting(loopID string) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	data, ok, err := store.GetExisting(store.Directory, loopBucket, loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if !ok {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop %s: %w", loopID, fs.ErrNotExist)
	}
	return decodeLoop(loopID, data)
}

func decodeLoop(loopID string, data []byte) (loopruncontract.LoopRun, error) {
	var loop loopruncontract.LoopRun
	if err := json.Unmarshal(data, &loop); err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if loop.ID != loopID {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop id mismatch: record has %q", loop.ID)
	}
	if err := validateLoopSchemaVersion(loop); err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	loop.OK = true
	return loop, nil
}

func (store Store) Write(ctx context.Context, loop loopruncontract.LoopRun) (loopruncontract.LoopRun, error) {
	if _, err := normalizeLoopID(loop.ID); err != nil {
		loop.OK = false
		return loop, err
	}
	if err := validateLoopSchemaVersion(loop); err != nil {
		loop.OK = false
		return loop, err
	}
	db, err := store.open()
	if err != nil {
		loop.OK = false
		return loop, err
	}
	loop.OK = true
	data, err := json.MarshalIndent(loop, "", "  ")
	if err != nil {
		loop.OK = false
		return loop, err
	}
	if err := db.Apply(ctx, []port.RecordMutation{{Bucket: loopBucket, ID: loop.ID, Data: data}}); err != nil {
		loop.OK = false
		return loop, err
	}
	return loop, nil
}

func (store Store) ListIDs() ([]string, error) {
	ids, err := store.ListExisting(store.Directory, loopBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func newLoopID(repo, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(name)))
	return "loop-" + hex.EncodeToString(sum[:])[:12]
}

func normalizeLoopID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("loop_id is required")
	}
	if !strings.HasPrefix(id, "loop-") || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid loop id %q", id)
	}
	return id, nil
}

func validateLoopSchemaVersion(loop loopruncontract.LoopRun) error {
	if loop.SchemaVersion != LoopRunCurrentSchemaVersion {
		return fmt.Errorf("unsupported loop schema_version %d; current is %d", loop.SchemaVersion, LoopRunCurrentSchemaVersion)
	}
	return nil
}
