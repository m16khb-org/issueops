package issueopslease

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

type reseedRejectionHarness struct {
	prepares int
	commits  int
}

func (h *reseedRejectionHarness) artifacts() reseedArtifactsFake {
	return reseedArtifactsFake{
		prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			h.prepares++
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64)}, nil
		},
		cleanup: func(context.Context, leasecontract.Record) error { return nil },
	}
}

func (h *reseedRejectionHarness) repository(record Record) reseedRepositoryFake {
	return reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
		h.commits++
		return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
	}}
}

func passThroughFence() reseedFenceFunc {
	return func(_ context.Context, _ string, fn func(context.Context) error) error {
		return fn(context.Background())
	}
}

type reseedLoadFailureRepository struct{ err error }

func (r reseedLoadFailureRepository) LoadSnapshot(context.Context, string) (ReseedSnapshot, error) {
	return ReseedSnapshot{}, r.err
}

func (reseedLoadFailureRepository) CommitReseed(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
	return RepositoryResult{}, errors.New("commit must not be reached")
}

func TestReseedServiceRequiresAllDependencies(t *testing.T) {
	harness := &reseedRejectionHarness{}
	good := newReseedServiceForTest(passThroughFence(), harness.repository(reseedTestRecord("released", 3)), harness.artifacts())
	for name, mutate := range map[string]func(*ReseedService){
		"fence":      func(s *ReseedService) { s.fence = nil },
		"repository": func(s *ReseedService) { s.repository = nil },
		"inventory":  func(s *ReseedService) { s.inventory = nil },
		"base sync":  func(s *ReseedService) { s.baseSync = nil },
		"artifacts":  func(s *ReseedService) { s.artifacts = nil },
		"clock":      func(s *ReseedService) { s.clock = nil },
		"paths":      func(s *ReseedService) { s.paths = nil },
	} {
		t.Run(name, func(t *testing.T) {
			service := *good
			mutate(&service)
			result, err := service.Reseed(t.Context(), reseedServiceRequest("io-reseed-test"))
			if err == nil || !strings.Contains(err.Error(), "reseed service dependencies are required") || result.ID != "io-reseed-test" || result.OK {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestReseedServiceSurfacesSnapshotLoadFailure(t *testing.T) {
	harness := &reseedRejectionHarness{}
	loadErr := errors.New("snapshot unreadable")
	service := newReseedServiceForTest(passThroughFence(), reseedLoadFailureRepository{err: loadErr}, harness.artifacts())
	if _, err := service.Reseed(t.Context(), reseedServiceRequest("io-reseed-test")); !errors.Is(err, loadErr) {
		t.Fatalf("error=%v", err)
	}
	if harness.prepares != 0 {
		t.Fatalf("prepared artifacts after a failed load: %d", harness.prepares)
	}
}

func TestReseedServiceRejectsIneligibleRecordsBeforeInventoryOrArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		record  func() Record
		request func(*ReseedRequest)
		check   func(error) bool
	}{
		{
			name: "execution not prepared",
			record: func() Record {
				record := reseedTestRecord("released", 3)
				record.Stable.Execution = nil
				return record
			},
			check: func(err error) bool { return errors.Is(err, leasecontract.ErrExecutionNotPrepared) },
		},
		{
			name: "pending external intent",
			record: func() Record {
				record := reseedTestRecord("released", 3)
				record.Stable.Execution.Pending = &leasecontract.ExternalIntent{OperationID: strings.Repeat("d", 32), Kind: "owner_launch"}
				return record
			},
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "pending external intent; run execution reconcile")
			},
		},
		{
			name:    "stale expected generation",
			record:  func() Record { return reseedTestRecord("released", 3) },
			request: func(r *ReseedRequest) { r.ExpectedGeneration = 2 },
			check:   func(err error) bool { return leasedomain.DenyCodeOf(err) == leasedomain.DenyLeaseGeneration },
		},
		{
			name:    "cwd outside the lifecycle roots",
			record:  func() Record { return reseedTestRecord("released", 3) },
			request: func(r *ReseedRequest) { r.CWD = "/unrelated" },
			check:   func(err error) bool { return leasedomain.DenyCodeOf(err) == leasedomain.DenyCanonicalCWD },
		},
		{
			name:    "completion generation without a completion",
			record:  func() Record { return reseedTestRecord("released", 3) },
			request: func(r *ReseedRequest) { r.CompletionGeneration = 3 },
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "completion_generation requires a current completion")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := &reseedRejectionHarness{}
			record := tt.record()
			service := newReseedServiceForTest(passThroughFence(), harness.repository(record), harness.artifacts())
			service.inventory = reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
				t.Fatal("inventory observed for an ineligible record")
				return ReseedInventoryReceipt{}, nil
			})
			request := reseedServiceRequest(record.ID)
			if tt.request != nil {
				tt.request(&request)
			}
			if _, err := service.Reseed(t.Context(), request); !tt.check(err) {
				t.Fatalf("error=%v", err)
			}
			if harness.prepares != 0 || harness.commits != 0 {
				t.Fatalf("prepares=%d commits=%d", harness.prepares, harness.commits)
			}
		})
	}
}

func TestReseedServiceRejectsCompletedExecutionWithoutAUsableBaseBranch(t *testing.T) {
	baseErr := errors.New("git unavailable")
	tests := []struct {
		name          string
		branchPrepare json.RawMessage
		observe       func(context.Context, basesyncport.Request) (basesyncport.Receipt, error)
		want          string
	}{
		{"missing branch_prepare", nil, nil, "completed reseed requires branch_prepare.base_branch"},
		{"undecodable branch_prepare", json.RawMessage(`[1]`), nil, "decode branch_prepare for completed reseed"},
		{"blank base branch", json.RawMessage(`{"base_branch":"  "}`), nil, "completed reseed requires branch_prepare.base_branch"},
		{"base observation failure", json.RawMessage(`{"base_branch":"main"}`), func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
			return basesyncport.Receipt{}, baseErr
		}, "observe completed execution base: git unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := &reseedRejectionHarness{}
			record := completedReseedTestRecord(strings.Repeat("d", 40))
			record.Stable.BranchPrepare = tt.branchPrepare
			observe := tt.observe
			if observe == nil {
				observe = func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
					t.Fatal("base observed without a usable base branch")
					return basesyncport.Receipt{}, nil
				}
			}
			service := newReseedServiceForTestWithBaseSync(passThroughFence(), harness.repository(record), harness.artifacts(), baseSyncInspectorFunc(observe))
			request := reseedServiceRequest(record.ID)
			request.ExpectedGeneration = 4
			_, err := service.Reseed(t.Context(), request)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
			if tt.name == "base observation failure" && !errors.Is(err, baseErr) {
				t.Fatalf("base observation cause lost: %v", err)
			}
			if harness.prepares != 0 || harness.commits != 0 {
				t.Fatalf("prepares=%d commits=%d", harness.prepares, harness.commits)
			}
		})
	}
}

func TestReseedServiceRejectsInventoryAndRecordProblemsBeforePreparingArtifacts(t *testing.T) {
	inventoryErr := errors.New("inventory unavailable")
	tests := []struct {
		name      string
		record    func() Record
		inventory func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error)
		check     func(error) bool
		request   func(*ReseedRequest)
	}{
		{
			name:   "inventory failure",
			record: func() Record { return reseedTestRecord("released", 3) },
			inventory: func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
				return ReseedInventoryReceipt{}, inventoryErr
			},
			check: func(err error) bool { return errors.Is(err, inventoryErr) },
		},
		{
			name: "orca mode without binding",
			record: func() Record {
				record := reseedTestRecord("released", 3)
				record.Stable.Execution.Mode = "orca"
				return record
			},
			check: func(err error) bool { return errors.Is(err, leasecontract.ErrExecutionNotPrepared) },
		},
		{
			name: "record that cannot be cloned",
			record: func() Record {
				record := reseedTestRecord("released", 3)
				record.Stable.Decisions = json.RawMessage(`{not json`)
				return record
			},
			check: func(err error) bool { return err != nil && strings.Contains(err.Error(), "json") },
		},
		{
			name: "completed record with a non-object phase ledger",
			record: func() Record {
				record := completedReseedTestRecord(strings.Repeat("d", 40))
				record.Stable.PhaseLedger = json.RawMessage(`[1]`)
				return record
			},
			request: func(r *ReseedRequest) { r.ExpectedGeneration = 4 },
			check:   func(err error) bool { return err != nil && strings.Contains(err.Error(), "decode phase ledger") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := &reseedRejectionHarness{}
			record := tt.record()
			service := newReseedServiceForTest(passThroughFence(), harness.repository(record), harness.artifacts())
			if tt.inventory != nil {
				service.inventory = reseedInventoryFunc(tt.inventory)
			}
			request := reseedServiceRequest(record.ID)
			if tt.request != nil {
				tt.request(&request)
			}
			if _, err := service.Reseed(t.Context(), request); !tt.check(err) {
				t.Fatalf("error=%v", err)
			}
			if harness.prepares != 0 || harness.commits != 0 {
				t.Fatalf("prepares=%d commits=%d", harness.prepares, harness.commits)
			}
		})
	}
}

func TestReseedServiceReportsBothCommitAndRollbackFailures(t *testing.T) {
	record := reseedTestRecord("released", 3)
	commitErr, rollbackErr := errors.New("commit failed"), errors.New("rollback failed")
	rollbacks := 0
	artifacts := reseedArtifactsFake{
		prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64)}, nil
		},
		rollback: func(context.Context, ReseedArtifactReceipt) error { rollbacks++; return rollbackErr },
		cleanup: func(context.Context, leasecontract.Record) error {
			t.Fatal("superseded cleanup after failed commit")
			return nil
		},
	}
	repository := reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
		return RepositoryResult{}, commitErr
	}}
	service := newReseedServiceForTest(passThroughFence(), repository, artifacts)
	_, err := service.Reseed(t.Context(), reseedServiceRequest(record.ID))
	if !errors.Is(err, commitErr) || !strings.Contains(err.Error(), "replacement residue cleanup failed: rollback failed") || rollbacks != 1 {
		t.Fatalf("error=%v rollbacks=%d", err, rollbacks)
	}
}

func TestReseedServiceChecksBaseDriftOfClaimableCompletedExecution(t *testing.T) {
	harness := &reseedRejectionHarness{}
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	record.Stable.Execution.Lease.Status = "claimable"
	record.Stable.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("b", 64)
	record.Lease = record.Stable.Execution.Lease
	observed := 0
	service := newReseedServiceForTestWithBaseSync(passThroughFence(), harness.repository(record), harness.artifacts(), baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
		observed++
		return basesyncport.Receipt{SyncRequired: true}, nil
	}))
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	_, err := service.Reseed(t.Context(), request)
	var syncRequired *issueopscontract.BaseSyncRequiredError
	if !errors.As(err, &syncRequired) || observed != 1 || harness.prepares != 0 || harness.commits != 0 {
		t.Fatalf("err=%v observed=%d prepares=%d commits=%d", err, observed, harness.prepares, harness.commits)
	}
}

func TestReseedServiceReopensCompletedExecutionWithoutRecordedPhaseLedger(t *testing.T) {
	harness := &reseedRejectionHarness{}
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	record.Stable.PhaseLedger = nil
	service := newReseedServiceForTest(passThroughFence(), harness.repository(record), harness.artifacts())
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	result, err := service.Reseed(t.Context(), request)
	if err != nil || harness.commits != 1 || result.Execution.Completion != nil {
		t.Fatalf("err=%v commits=%d execution=%+v", err, harness.commits, result.Execution)
	}
}
