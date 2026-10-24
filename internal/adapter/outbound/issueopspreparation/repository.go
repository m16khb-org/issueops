package issueopspreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	issueopsdomain "issueops/internal/domain/issueops"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

const (
	recordBucket = port.IssueOpsRecordBucket
	holderBucket = port.LeaseHolderBucket
)

type SQLiteRepository struct {
	store port.RecordInventoryStore
}

func NewSQLiteRepository(store port.RecordInventoryStore) *SQLiteRepository {
	return &SQLiteRepository{store: store}
}

func (repository *SQLiteRepository) Load(_ context.Context, id string) (preparationcontract.Snapshot, error) {
	if repository.store == nil {
		return preparationcontract.Snapshot{}, fmt.Errorf("preparation record store is unavailable")
	}
	data, ok, err := repository.store.Get(recordBucket, id)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	if !ok {
		return preparationcontract.Snapshot{}, fmt.Errorf("issueops record %s not found", id)
	}
	record, err := recordcodec.DecodeLease(id, data)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return preparationcontract.Snapshot{}, err
	}
	return preparationcontract.Snapshot{
		Record: record, RecordRaw: append([]byte(nil), data...), ClaimTokenPath: currentClaimTokenPath(record),
		ArtifactDir: remote.IssueArtifactDir(record.IssueURL),
	}, nil
}

func (repository *SQLiteRepository) EnsureRootUnclaimed(_ context.Context, selfID, root string) error {
	if repository.store == nil {
		return fmt.Errorf("preparation record store is unavailable")
	}
	return ensureRootUnclaimed(repository.store, selfID, root)
}

// EnsureActorHoldsNoLease reads the actor's lease-holder index row. Commit
// inserts it with RequireAbsent, so without this check a session holding
// another cycle's lease got a store precondition error only after the
// worktree was provisioned.
func (repository *SQLiteRepository) EnsureActorHoldsNoLease(_ context.Context, actor leasecontract.Actor) error {
	if repository.store == nil {
		return fmt.Errorf("preparation record store is unavailable")
	}
	data, exists, err := repository.store.Get(holderBucket, holderIndexKey(actor))
	if err != nil || !exists {
		return err
	}
	var index holderIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("decode active lease-holder index: %w", err)
	}
	return fmt.Errorf("this session already holds the execution lease of %s (generation %d) and a session holds one lease at a time: release it first (issueops execution release --id %s --generation %d $ACTOR_FLAGS --json), prepare this cycle, and claim %s again later",
		index.LifecycleID, index.Generation, index.LifecycleID, index.Generation, index.LifecycleID)
}

func (repository *SQLiteRepository) CommitDirect(ctx context.Context, commit preparationapp.DirectCommit) (preparationcontract.Result, error) {
	if repository.store == nil {
		return preparationcontract.Result{ID: commit.Command.ID}, fmt.Errorf("preparation record store is unavailable")
	}
	var result preparationcontract.Result
	err := repository.store.WithSpan(ctx, func(spanCtx context.Context) error {
		current, err := repository.Load(spanCtx, commit.Command.ID)
		if err != nil {
			return err
		}
		commit.ArtifactDir = remote.IssueArtifactDir(current.Record.IssueURL)
		record, projected, err := preparationapp.ApplyDirectCommit(current.Record, commit)
		if err != nil {
			return err
		}
		if err := ensureRootUnclaimed(repository.store, current.Record.ID, commit.Workspace.Root); err != nil {
			return err
		}
		actor := *record.Execution.Lease.Holder
		data, err := recordcodec.EncodeLease(record)
		if err != nil {
			return err
		}
		indexData, err := json.Marshal(holderIndex{
			SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID,
			Generation: 1, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		})
		if err != nil {
			return err
		}
		if err := repository.store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: recordBucket, ID: record.ID, Data: data},
			{Bucket: holderBucket, ID: holderIndexKey(actor), Data: indexData, RequireAbsent: true},
		}); err != nil {
			return err
		}
		result = projected
		return nil
	})
	if err != nil {
		return preparationcontract.Result{ID: commit.Command.ID}, err
	}
	return result, nil
}

func ensureRootUnclaimed(store port.RecordInventoryStore, selfID, root string) error {
	target := cleanAbsPath(root)
	if target == "" {
		return fmt.Errorf("canonical worktree root is required")
	}
	rows, err := store.GetAll(recordBucket)
	if err != nil {
		return err
	}
	self := strings.TrimSpace(selfID)
	for _, row := range rows {
		if row.ID == self {
			continue
		}
		record, err := recordcodec.DecodeLease(row.ID, row.Data)
		if err != nil {
			return fmt.Errorf("canonical worktree 소유권 스캔이 lifecycle %s 레코드를 읽지 못했다; 손상 레코드를 먼저 해소하라: %w", row.ID, err)
		}
		executionRoot := ""
		if record.Execution != nil {
			executionRoot = record.Execution.Workspace.Root
		}
		if preparationdomain.CanonicalRootClaimConflict(self, target, row.ID, cleanAbsPath(record.WorktreePath), cleanAbsPath(executionRoot)) {
			return fmt.Errorf(
				"canonical worktree %s는 이미 lifecycle %s(브랜치 %s)가 선점했다; 먼저 그 사이클을 정리하라: issueops cleanup finish --id %s --preview --json",
				target, row.ID, strings.TrimSpace(record.Branch), row.ID,
			)
		}
	}
	return nil
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

type holderIndex struct {
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

func holderIndexKey(actor leasecontract.Actor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}

func currentClaimTokenPath(record leasecontract.Record) string {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return ""
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(record.ID)))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}
