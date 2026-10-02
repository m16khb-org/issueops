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

type StartIdentity interface {
	CanonicalRepo(string) string
	StableID(string, string) string
	IndependentID(string) (string, error)
}

type Starter struct {
	Records  CycleRecords
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
		id = s.Identity.StableID(repo, branch)
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
		if !req.New {
			recordID = s.Identity.StableID(repo, branch)
		}
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
