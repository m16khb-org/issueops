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

// CycleRecords is the persistence port for cycle records: WithinLock runs fn
// holding the lock of one cycle ID, Load reads and Save writes a record.
type CycleRecords interface {
	WithinLock(context.Context, string, func(context.Context) error) error
	Load(string) (model.Record, error)
	Save(context.Context, model.Record) (model.Record, error)
}

// retiredCycles names the cycles a plain start must not resume: the IDs
// cleanup already deleted, and live cycles that are already done.
type retiredCycles interface {
	RetiredIDs() (map[string]bool, error)
	Done(id string) bool
}

type startIdentity interface {
	CanonicalRepo(string) string
	SuccessorID(repo, branch string, n int) string
	IndependentID(string) (string, error)
}

// Starter creates a cycle record for a repository and branch, or returns the
// resumable one that already exists.
type Starter struct {
	Records  CycleRecords
	Retired  retiredCycles
	Identity startIdentity
	// IsGitRepo refuses a repo outside git (nil skips the check). Probe:
	// `start --repo <plain dir>` created a cycle that could only fail later,
	// at branch prepare.
	IsGitRepo func(string) bool
	Now       func() time.Time
}

// Start validates req and returns the cycle for its repo and branch, creating
// it when absent. Without req.New it resumes the first unfinished cycle;
// with it a new independent cycle is created. A repo outside git is refused.
func (s Starter) Start(ctx context.Context, req model.StartRequest) (model.Record, error) {
	if err := domain.ValidateNewCycleRequest(req); err != nil {
		return model.Record{OK: false}, err
	}
	if repo := strings.TrimSpace(req.Repo); repo != "" && s.IsGitRepo != nil && !s.IsGitRepo(repo) {
		return model.Record{OK: false}, fmt.Errorf("repo %s is not a git repository; an issueops cycle needs one: pass the repository root (git rev-parse --show-toplevel), or git init first", repo)
	}
	repo := s.Identity.CanonicalRepo(req.Repo)
	branch := strings.TrimSpace(req.Branch)
	id, err := s.cycleID(repo, branch, req.New)
	if err != nil {
		return model.Record{OK: false}, err
	}
	var record model.Record
	err = s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
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

// cycleID picks the ID for a new independent cycle, or the resumable ID for repo and branch.
func (s Starter) cycleID(repo, branch string, independent bool) (string, error) {
	if independent {
		return s.Identity.IndependentID(repo)
	}
	return s.resumableID(repo, branch)
}

// resumableID is the first stable ID for repo and branch that no cleaned-up
// cycle used and no completed cycle holds. Cleanup keeps the outcome row under
// the deleted record's ID, so a new cycle reusing it would share the old
// cycle's audit lines and overwrite its outcome (QA: #42 got #41's ID right
// after cleanup). A done cycle awaiting merge takes no new work either; plain
// start used to return it, so a second task in the same checkout was attached
// to the finished cycle. The successors are derived, so a repeated start still
// resumes the same unfinished cycle.
func (s Starter) resumableID(repo, branch string) (string, error) {
	retired, err := s.Retired.RetiredIDs()
	if err != nil {
		return "", err
	}
	for n := 0; ; n++ {
		id := s.Identity.SuccessorID(repo, branch, n)
		if retired[id] {
			continue
		}
		if s.Retired.Done(id) {
			continue
		}
		return id, nil
	}
}
