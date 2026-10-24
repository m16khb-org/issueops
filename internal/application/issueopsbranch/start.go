package issueopsbranch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type CycleRecords interface {
	WithinLock(context.Context, string, func(context.Context) error) error
	Load(string) (model.IssueOpsRecord, error)
	Save(context.Context, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

// RetiredCycles names the IDs of cycles that cleanup already deleted.
type RetiredCycles interface {
	RetiredIDs() (map[string]bool, error)
}

type StartIdentity interface {
	CanonicalRepo(string) string
	SuccessorID(repo, branch string, n int) string
	IndependentID(string) (string, error)
}

type Starter struct {
	Records  CycleRecords
	Retired  RetiredCycles
	Identity StartIdentity
	Now      func() time.Time
}

func (s Starter) Start(ctx context.Context, req model.IssueOpsStartRequest) (model.IssueOpsRecord, error) {
	if err := domain.ValidateNewCycleRequest(req); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	repo := s.Identity.CanonicalRepo(req.Repo)
	branch := strings.TrimSpace(req.Branch)
	var id string
	if req.New {
		var err error
		id, err = s.Identity.IndependentID(repo)
		if err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
	} else {
		var err error
		id, err = s.resumableID(repo, branch)
		if err != nil {
			return model.IssueOpsRecord{OK: false}, err
		}
	}
	var record model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		repo := strings.TrimSpace(req.Repo)
		if repo == "" {
			return fmt.Errorf("repo is required")
		}
		repo = s.Identity.CanonicalRepo(repo)
		if repo == "" {
			return fmt.Errorf("repo is required")
		}
		if err := domain.ValidateBranch(branch); err != nil {
			return err
		}
		recordID := id
		existing, readErr := s.Records.Load(recordID)
		reuse, err := domain.ReuseStartRecord(recordID, req.New, readErr, errors.Is(readErr, fs.ErrNotExist))
		if err != nil {
			return err
		}
		if reuse {
			record = existing
			return nil
		}
		record, err = s.Records.Save(spanCtx, domain.NewCycleRecord(recordID, repo, branch, s.Now().UTC().Format(time.RFC3339Nano)))
		return err
	})
	return record, err
}

// resumableID is the first stable ID for repo and branch that no cleaned-up
// cycle used. Cleanup keeps the outcome row under the deleted record's ID, so
// a new cycle reusing it would share the old cycle's audit lines and
// overwrite its outcome (QA: #42 got #41's ID right after cleanup). The
// successors are derived, so a repeated start still resumes the same cycle.
func (s Starter) resumableID(repo, branch string) (string, error) {
	retired, err := s.Retired.RetiredIDs()
	if err != nil {
		return "", err
	}
	for n := 0; ; n++ {
		if id := s.Identity.SuccessorID(repo, branch, n); !retired[id] {
			return id, nil
		}
	}
}
