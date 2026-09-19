package issueopsinventory

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func TestServiceListCyclesFiltersAndProjectsInventory(t *testing.T) {
	now := time.Date(2026, time.August, 11, 3, 4, 5, 0, time.UTC)
	repository := fakeRepository{
		ids: []string{"planning", "claimable", "done", "elsewhere", "broken"},
		records: map[string]issueopscontract.IssueOpsRecord{
			"planning": {
				ID: "planning", Repo: "/repo", Branch: "84-planning",
				Phase: issueopscontract.IssueOpsPhasePlan,
			},
			"claimable": {
				ID: "claimable", Repo: "/repo", Branch: "84-claimable",
				Phase: issueopscontract.IssueOpsPhaseImplement,
				IssueCreateIntent: &issueopscontract.IssueOpsIssueCreateIntent{
					Status: issueopscontract.IssueCreateIntentInvokedUnknown,
				},
				Execution: &issueopscontract.Execution{
					Mode: issueopscontract.ExecutionModeOrca,
					Workspace: issueopscontract.Workspace{
						Root: "/worktrees/84-claimable",
					},
					Lease: issueopscontract.WriteLease{
						Status: issueopscontract.LeaseStatusClaimable,
					},
					Pending: &issueopscontract.ExternalIntent{
						Kind:      "remote_pr_create",
						StartedAt: "2026-08-11T02:00:00Z",
					},
					Failure: &issueopscontract.ExecutionFailure{
						Code: "external_operation_ambiguous",
						At:   "2026-08-11T02:01:00Z",
					},
					Orca: &issueopscontract.OrcaBinding{
						OwnerModel: "gpt-5.6-terra",
					},
				},
			},
			"done": {
				ID: "done", Repo: "/repo", Branch: "84-done",
				Phase: issueopscontract.IssueOpsPhaseDone,
				RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{
					URL: "https://github.com/acme/repo/pull/1",
				},
				CleanupFinishFailure: &issueopscontract.IssueOpsCleanupFinishFailure{
					Step: "worktree_remove",
					At:   "2026-08-11T02:02:00Z",
				},
			},
			"elsewhere": {
				ID: "elsewhere", Repo: "/other", Phase: issueopscontract.IssueOpsPhasePlan,
			},
		},
		readErrors: map[string]error{"broken": errors.New("invalid record")},
	}
	service := NewService(repository, fixedClock{now: now}, cleanPath{})

	result, err := service.ListCycles(context.Background(), "/state", "/repo/")
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if !result.OK || result.GeneratedAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if result.ScannedRecords != 5 {
		t.Fatalf("scanned records = %d, want 5", result.ScannedRecords)
	}
	if result.ReadErrors != 1 ||
		len(result.UnreadableIDs) != 1 ||
		result.UnreadableIDs[0] != "broken" ||
		len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != "invalid_state" {
		t.Fatalf("unreadable records = %+v", result)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries = %d, want 3: %+v", len(result.Entries), result.Entries)
	}

	entries := make(map[string]issueopsinventorycontract.ListEntry, len(result.Entries))
	for _, entry := range result.Entries {
		entries[entry.ID] = entry
	}
	if entry := entries["planning"]; entry.Claimable || entry.CleanupCandidate {
		t.Fatalf("planning flags: %+v", entry)
	}
	if entry := entries["claimable"]; !entry.Claimable ||
		entry.OwnerModel != "gpt-5.6-terra" ||
		entry.LeaseStatus != string(issueopscontract.LeaseStatusClaimable) ||
		entry.IssueCreateStatus != issueopscontract.IssueCreateIntentInvokedUnknown ||
		entry.PendingKind != "remote_pr_create" ||
		entry.FailureCode != "external_operation_ambiguous" {
		t.Fatalf("claimable projection: %+v", entry)
	}
	if entry := entries["done"]; !entry.CleanupCandidate ||
		!entry.CompletionUnreflected ||
		entry.CleanupFailureStep != "worktree_remove" {
		t.Fatalf("done projection: %+v", entry)
	}
}

func TestServiceListCyclesReusesExactRecordNormalizationWithinRequest(t *testing.T) {
	tests := []struct {
		name        string
		recordCount int
		uniquePaths int
	}{
		{name: "N1_U1", recordCount: 1, uniquePaths: 1},
		{name: "N100_U1", recordCount: 100, uniquePaths: 1},
		{name: "N100_U10", recordCount: 100, uniquePaths: 10},
		{name: "N100_U100", recordCount: 100, uniquePaths: 100},
		{name: "N1000_U1", recordCount: 1000, uniquePaths: 1},
		{name: "N1000_U10", recordCount: 1000, uniquePaths: 10},
		{name: "N1000_U1000", recordCount: 1000, uniquePaths: 1000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := inventoryFixture(test.recordCount, test.uniquePaths)
			paths := newRecordingNormalizer(func(string, int) string { return "/source" })
			service := NewService(repository, fixedClock{now: time.Unix(0, 0).UTC()}, paths)

			result, err := service.ListCycles(context.Background(), "/state", "/filter")
			if err != nil {
				t.Fatalf("list cycles: %v", err)
			}
			if len(result.Entries) != test.recordCount {
				t.Fatalf("entries = %d, want %d", len(result.Entries), test.recordCount)
			}
			for index, entry := range result.Entries {
				if want := fmt.Sprintf("io-%04d", index); entry.ID != want {
					t.Fatalf("entry %d ID = %q, want %q", index, entry.ID, want)
				}
			}
			if got := paths.Calls("/filter"); got != 1 {
				t.Fatalf("repo-filter normalization calls = %d, want 1", got)
			}
			if got, want := paths.TotalCalls(), test.uniquePaths+1; got != want {
				t.Fatalf("normalization calls = %d, want %d (U + separate repo filter)", got, want)
			}
			for unique := 0; unique < test.uniquePaths; unique++ {
				path := fmt.Sprintf("/worktree-%04d", unique)
				if got := paths.Calls(path); got != 1 {
					t.Fatalf("Normalize(%q) calls = %d, want 1", path, got)
				}
			}
		})
	}
}

func TestServiceListCyclesCachesFallbacksButKeepsDistinctInputs(t *testing.T) {
	tests := []struct {
		name        string
		filter      string
		recordPaths []string
		normalize   func(string, int) string
		wantIDs     []string
		wantCalls   map[string]int
	}{
		{
			name:        "deleted_repo_uses_lexical_fallback",
			filter:      "/deleted/repo",
			recordPaths: []string{"/deleted/repo", "/deleted/repo"},
			normalize:   func(path string, _ int) string { return path },
			wantIDs:     []string{"io-0000", "io-0001"},
			wantCalls:   map[string]int{"/deleted/repo": 2},
		},
		{
			name:        "git_failure_fallback_is_reused",
			filter:      "/selected",
			recordPaths: []string{"/git-failure", "/git-failure"},
			normalize:   func(path string, _ int) string { return path },
			wantIDs:     []string{},
			wantCalls:   map[string]int{"/selected": 1, "/git-failure": 1},
		},
		{
			name:        "different_worktrees_are_observed_separately",
			filter:      "/source",
			recordPaths: []string{"/source.worktrees/one", "/source.worktrees/two"},
			normalize: func(path string, _ int) string {
				if path == "/source.worktrees/one" || path == "/source.worktrees/two" {
					return "/source"
				}
				return path
			},
			wantIDs: []string{"io-0000", "io-0001"},
			wantCalls: map[string]int{
				"/source":               1,
				"/source.worktrees/one": 1,
				"/source.worktrees/two": 1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := inventoryRecords(test.recordPaths)
			paths := newRecordingNormalizer(test.normalize)
			service := NewService(repository, fixedClock{now: time.Unix(0, 0).UTC()}, paths)

			result, err := service.ListCycles(context.Background(), "/state", test.filter)
			if err != nil {
				t.Fatalf("list cycles: %v", err)
			}
			if len(result.Entries) != len(test.wantIDs) {
				t.Fatalf("entries = %+v, want IDs %v", result.Entries, test.wantIDs)
			}
			for index, want := range test.wantIDs {
				if result.Entries[index].ID != want {
					t.Fatalf("entry %d ID = %q, want %q", index, result.Entries[index].ID, want)
				}
			}
			for path, want := range test.wantCalls {
				if got := paths.Calls(path); got != want {
					t.Fatalf("Normalize(%q) calls = %d, want %d", path, got, want)
				}
			}
		})
	}
}

func TestServiceListCyclesReobservesRecordPathsBetweenRequests(t *testing.T) {
	repository := inventoryRecords([]string{"/linked"})
	paths := newRecordingNormalizer(func(path string, call int) string {
		if path != "/linked" {
			return path
		}
		if call == 1 {
			return "/source-a"
		}
		return "/source-b"
	})
	service := NewService(repository, fixedClock{now: time.Unix(0, 0).UTC()}, paths)

	first, err := service.ListCycles(context.Background(), "/state", "/source-a")
	if err != nil || len(first.Entries) != 1 {
		t.Fatalf("first list = %+v, %v", first, err)
	}
	second, err := service.ListCycles(context.Background(), "/state", "/source-b")
	if err != nil || len(second.Entries) != 1 {
		t.Fatalf("second list = %+v, %v", second, err)
	}
	if got := paths.Calls("/linked"); got != 2 {
		t.Fatalf("linked path calls = %d, want one fresh observation per request", got)
	}
}

func TestServiceListCyclesKeepsConcurrentRequestCachesIndependent(t *testing.T) {
	repository := inventoryRecords([]string{"/worktree", "/worktree"})
	paths := newRecordingNormalizer(func(path string, _ int) string {
		if path == "/worktree" {
			return "/source"
		}
		return path
	})
	service := NewService(repository, fixedClock{now: time.Unix(0, 0).UTC()}, paths)

	const requests = 16
	start := make(chan struct{})
	errors := make(chan error, requests)
	var group sync.WaitGroup
	for range requests {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			result, err := service.ListCycles(context.Background(), "/state", "/source")
			if err != nil {
				errors <- err
				return
			}
			if len(result.Entries) != 2 {
				errors <- fmt.Errorf("entries = %d, want 2", len(result.Entries))
			}
		}()
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := paths.Calls("/source"); got != requests {
		t.Fatalf("repo-filter calls = %d, want %d", got, requests)
	}
	if got := paths.Calls("/worktree"); got != requests {
		t.Fatalf("record-path calls = %d, want %d request-local observations", got, requests)
	}
}

func BenchmarkServiceListCyclesNormalization(b *testing.B) {
	repository := inventoryFixture(1000, 10)
	paths := &benchmarkNormalizer{}
	service := NewService(repository, fixedClock{now: time.Unix(0, 0).UTC()}, paths)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := service.ListCycles(context.Background(), "/state", "/filter")
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Entries) != 1000 {
			b.Fatalf("entries = %d, want 1000", len(result.Entries))
		}
		benchmarkEntryCountSink = len(result.Entries)
	}
	b.ReportMetric(float64(paths.calls)/float64(b.N), "normalizations/op")
}

func TestServiceListCyclesReturnsRepositoryFailure(t *testing.T) {
	want := errors.New("inventory unavailable")
	service := NewService(
		fakeRepository{listError: want},
		fixedClock{now: time.Unix(0, 0).UTC()},
		cleanPath{},
	)

	result, err := service.ListCycles(context.Background(), "/state", "")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if result.OK {
		t.Fatalf("failed result must not be ok: %+v", result)
	}
}

func inventoryFixture(recordCount, uniquePaths int) fakeRepository {
	paths := make([]string, recordCount)
	for index := range paths {
		paths[index] = fmt.Sprintf("/worktree-%04d", index%uniquePaths)
	}
	return inventoryRecords(paths)
}

func inventoryRecords(paths []string) fakeRepository {
	repository := fakeRepository{
		ids:     make([]string, len(paths)),
		records: make(map[string]issueopscontract.IssueOpsRecord, len(paths)),
	}
	for index, path := range paths {
		id := fmt.Sprintf("io-%04d", index)
		repository.ids[index] = id
		repository.records[id] = issueopscontract.IssueOpsRecord{
			ID:    id,
			Repo:  path,
			Phase: issueopscontract.IssueOpsPhasePlan,
		}
	}
	return repository
}

type recordingNormalizer struct {
	mu        sync.Mutex
	calls     map[string]int
	normalize func(string, int) string
}

func newRecordingNormalizer(normalize func(string, int) string) *recordingNormalizer {
	return &recordingNormalizer{calls: map[string]int{}, normalize: normalize}
}

func (normalizer *recordingNormalizer) Normalize(path string) string {
	normalizer.mu.Lock()
	defer normalizer.mu.Unlock()
	normalizer.calls[path]++
	return normalizer.normalize(path, normalizer.calls[path])
}

func (normalizer *recordingNormalizer) Calls(path string) int {
	normalizer.mu.Lock()
	defer normalizer.mu.Unlock()
	return normalizer.calls[path]
}

func (normalizer *recordingNormalizer) TotalCalls() int {
	normalizer.mu.Lock()
	defer normalizer.mu.Unlock()
	total := 0
	for _, calls := range normalizer.calls {
		total += calls
	}
	return total
}

type benchmarkNormalizer struct{ calls int64 }

var (
	benchmarkDigestSink     [32]byte
	benchmarkEntryCountSink int
)

func (normalizer *benchmarkNormalizer) Normalize(path string) string {
	normalizer.calls++
	digest := sha256.Sum256([]byte(path))
	for range 32 {
		digest = sha256.Sum256(digest[:])
	}
	benchmarkDigestSink = digest
	return "/source"
}

type fakeRepository struct {
	ids        []string
	records    map[string]issueopscontract.IssueOpsRecord
	readErrors map[string]error
	listError  error
}

func (f fakeRepository) Scan(
	context.Context,
	string,
) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error) {
	if f.listError != nil {
		return nil, nil, f.listError
	}
	records := make([]issueopsinventorycontract.Record, 0, len(f.ids))
	diagnostics := make([]issueopsinventorycontract.RecordDiagnostic, 0)
	for _, id := range f.ids {
		if f.readErrors[id] != nil {
			diagnostics = append(
				diagnostics,
				issueopsinventorycontract.RecordDiagnostic{ID: id, Code: "invalid_state"},
			)
			continue
		}
		records = append(records, f.records[id])
	}
	return records, diagnostics, nil
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

type cleanPath struct{}

func (cleanPath) Normalize(path string) string {
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}
