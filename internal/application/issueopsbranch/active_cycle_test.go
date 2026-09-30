package issueopsbranch_test

import (
	"errors"
	"path/filepath"
	"sort"
	"testing"

	"issueops/internal/adapter/issueops/pathutil"
	app "issueops/internal/application/issueopsbranch"
	model "issueops/internal/contract/issueops"
)

type activeTestStore struct {
	records map[string]model.IssueOpsRecord
}

func newActiveTestStore(t *testing.T) *activeTestStore {
	t.Helper()
	return &activeTestStore{records: map[string]model.IssueOpsRecord{}}
}
func (s *activeTestStore) writeRecord(t *testing.T, record model.IssueOpsRecord) {
	t.Helper()
	s.records[record.ID] = record
}
func (s *activeTestStore) reader() app.ActiveCycleReader {
	return app.ActiveCycleReader{CleanPath: pathutil.CleanAbsPath, Scan: func() ([]model.IssueOpsRecord, error) {
		ids := make([]string, 0, len(s.records))
		for id := range s.records {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		records := make([]model.IssueOpsRecord, 0, len(ids))
		for _, id := range ids {
			records = append(records, s.records[id])
		}
		return records, nil
	}}
}

func TestActiveUmbrellaIncludesDeletedWorktreeAndExcludesDoneAndOtherRepos(t *testing.T) {
	store := newActiveTestStore(t)
	repo, other := t.TempDir(), t.TempDir()
	url := "https://github.com/acme/repo/issues/79"
	for _, record := range []model.IssueOpsRecord{
		{ID: "a-done", Repo: repo, Phase: model.IssueOpsPhaseDone},
		{ID: "b-other", Repo: other, Phase: model.IssueOpsPhasePlan},
		{ID: "c-parent", Repo: repo, Phase: model.IssueOpsPhaseImplement, WorktreePath: filepath.Join(t.TempDir(), "gone")},
	} {
		record.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: url}}
		store.writeRecord(t, record)
	}
	got, ok := store.reader().UmbrellaForChildIssue(repo, url)
	if !ok || got.ID != "c-parent" {
		t.Fatalf("active parent selection=%+v found=%v", got, ok)
	}
}

func TestActiveCycleReaderUsesOneScanAndPreservesUnavailableResult(t *testing.T) {
	scans := 0
	fail := false
	reader := app.ActiveCycleReader{CleanPath: pathutil.CleanAbsPath, Scan: func() ([]model.IssueOpsRecord, error) {
		scans++
		if fail {
			return nil, errors.New("unreadable inventory")
		}
		return []model.IssueOpsRecord{{ID: "child", Repo: "/repo", WorktreePath: "/worktree", Phase: model.IssueOpsPhaseImplement, BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: " 78-parent "}}}, nil
	}}
	if base, ok := reader.PreparedBaseBranchForWorkspace("/worktree/../worktree"); !ok || base != "78-parent" || scans != 1 {
		t.Fatalf("base=%q found=%v scans=%d", base, ok, scans)
	}
	fail = true
	if _, ok := reader.ForWorkspace("/repo"); ok || scans != 2 {
		t.Fatalf("unreadable inventory found=%v scans=%d", ok, scans)
	}
	if _, ok := reader.UmbrellaForChildIssue("/repo", "url"); ok || scans != 3 {
		t.Fatalf("unreadable umbrella found=%v scans=%d", ok, scans)
	}
	reader.ForWorkspace(" ")
	reader.UmbrellaForChildIssue("/repo", " ")
	reader.UmbrellaForChildIssue(" ", "url")
	if scans != 3 {
		t.Fatalf("empty input scanned inventory: %d", scans)
	}
}
