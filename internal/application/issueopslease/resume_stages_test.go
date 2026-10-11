package issueopslease

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

type resumeFailure struct {
	state string
	cause error
}

// resumeScriptRepository drives one pending owner-launch stage and records
// every state transition the service asks the repository to persist.
type resumeScriptRepository struct {
	record          Record
	snapshotErr     error
	beginErr        error
	loadIntentErr   error
	markErr         error
	applyErr        error
	intentState     string
	intentAttempts  int
	applyPending    bool
	begins          int
	loads           int
	marks           int
	failures        []resumeFailure
	applied         []leasecontract.ResumeStageReceipt
	appliedIntentIn []ResumeIntentState
}

func (r *resumeScriptRepository) progress(pending bool) ResumeProgress {
	execution := *r.record.Stable.Execution
	return ResumeProgress{Record: r.record, Execution: execution, Pending: pending}
}

func (r *resumeScriptRepository) LoadSnapshot(context.Context, string, uint64) (ResumeSnapshot, error) {
	return ResumeSnapshot{Record: r.record}, r.snapshotErr
}

func (r *resumeScriptRepository) BeginIntent(context.Context, ResumeSnapshot, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeProgress, error) {
	r.begins++
	return r.progress(true), r.beginErr
}

func (r *resumeScriptRepository) LoadIntent(context.Context, ResumeProgress) (ResumeIntentState, error) {
	r.loads++
	state := r.intentState
	if state == "" {
		state = "not_invoked_proven"
	}
	return ResumeIntentState{Progress: r.progress(true), OperationID: strings.Repeat("d", 32), Stage: "task_create", InvocationState: state, InvocationAttempts: r.intentAttempts}, r.loadIntentErr
}

func (r *resumeScriptRepository) MarkInvoking(_ context.Context, intent ResumeIntentState) (ResumeIntentState, error) {
	r.marks++
	intent.InvocationState = "unknown"
	intent.InvocationAttempts++
	return intent, r.markErr
}

func (r *resumeScriptRepository) RecordFailure(_ context.Context, _ ResumeIntentState, state string, cause error) error {
	r.failures = append(r.failures, resumeFailure{state: state, cause: cause})
	return nil
}

func (r *resumeScriptRepository) ApplyReceipt(_ context.Context, intent ResumeIntentState, receipt leasecontract.ResumeStageReceipt) (ResumeProgress, error) {
	r.applied = append(r.applied, receipt)
	r.appliedIntentIn = append(r.appliedIntentIn, intent)
	if r.applyErr != nil {
		return ResumeProgress{}, r.applyErr
	}
	return r.progress(r.applyPending), nil
}

type resumeScriptStages struct {
	inventory  leasecontract.ResumeStageInventory
	inspectErr error
	invokeErr  error
	receipt    leasecontract.ResumeStageReceipt
	invokes    int
}

func (s *resumeScriptStages) Inspect(context.Context, ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	return s.inventory, s.inspectErr
}

func (s *resumeScriptStages) Invoke(context.Context, ResumeIntentState, []string) (leasecontract.ResumeStageReceipt, error) {
	s.invokes++
	return s.receipt, s.invokeErr
}

func scriptedResumeFixture() (Record, *resumeScriptRepository, *resumeScriptStages, *ResumeService) {
	record := resumeApplicationTestRecord(4)
	repository := &resumeScriptRepository{record: record}
	stages := &resumeScriptStages{inventory: leasecontract.ResumeStageInventory{AuthoritativeZero: true}, receipt: leasecontract.ResumeStageReceipt{TaskID: "task"}}
	return record, repository, stages, resumeApplicationStageService(record, repository, stages)
}

func TestResumeDrivesStageToCompletionWhenInventoryProvesAbsence(t *testing.T) {
	record, repository, stages, service := scriptedResumeFixture()
	result, err := service.Resume(t.Context(), resumeRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Disposition != leasedomain.ResumeCreateTerminal {
		t.Fatalf("result=%+v", result)
	}
	if stages.invokes != 1 || repository.marks != 1 || !reflect.DeepEqual(repository.applied, []leasecontract.ResumeStageReceipt{{TaskID: "task"}}) {
		t.Fatalf("invokes=%d marks=%d applied=%+v", stages.invokes, repository.marks, repository.applied)
	}
	if repository.appliedIntentIn[0].InvocationState != "unknown" || repository.appliedIntentIn[0].InvocationAttempts != 1 {
		t.Fatalf("receipt must apply against the marked-invoking intent: %+v", repository.appliedIntentIn[0])
	}
	if len(repository.failures) != 0 {
		t.Fatalf("unexpected failure records: %+v", repository.failures)
	}
}

func TestResumeAdoptsTheOneInventoryCandidateWithoutInvoking(t *testing.T) {
	record, repository, stages, service := scriptedResumeFixture()
	stages.inventory = leasecontract.ResumeStageInventory{Candidates: []leasecontract.ResumeStageReceipt{{TaskID: "found-task"}}}
	if _, err := service.Resume(t.Context(), resumeRequest(record)); err != nil {
		t.Fatal(err)
	}
	if stages.invokes != 0 || repository.marks != 0 {
		t.Fatalf("adopt repeated the mutation: invokes=%d marks=%d", stages.invokes, repository.marks)
	}
	if !reflect.DeepEqual(repository.applied, []leasecontract.ResumeStageReceipt{{TaskID: "found-task"}}) {
		t.Fatalf("applied=%+v", repository.applied)
	}
}

func TestResumeRetainsIntentWhenInventoryCannotAuthorizeAMutation(t *testing.T) {
	tests := []struct {
		name      string
		inventory leasecontract.ResumeStageInventory
		state     string
		attempts  int
		want      string
		wantState string
	}{
		{"multiple candidates", leasecontract.ResumeStageInventory{Candidates: []leasecontract.ResumeStageReceipt{{TaskID: "a"}, {TaskID: "b"}}}, "unknown", 1, "Orca intent inventory found multiple candidates; intent retained", "unknown"},
		{"non authoritative zero", leasecontract.ResumeStageInventory{}, "not_invoked_proven", 0, "Orca intent inventory returned a non-authoritative zero; intent retained", "not_invoked_proven"},
		{"retries exhausted", leasecontract.ResumeStageInventory{AuthoritativeZero: true}, "not_invoked_proven", 2, "Orca intent retry is exhausted; intent retained", "not_invoked_proven"},
		{"exhausted exact replay", leasecontract.ResumeStageInventory{ExactReplay: true}, "unknown", 2, "Orca intent retry is exhausted; intent retained", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, repository, stages, service := scriptedResumeFixture()
			stages.inventory = tt.inventory
			repository.intentState, repository.intentAttempts = tt.state, tt.attempts
			_, err := service.Resume(t.Context(), resumeRequest(record))
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
			if stages.invokes != 0 || repository.marks != 0 || len(repository.applied) != 0 {
				t.Fatalf("mutation attempted: invokes=%d marks=%d applied=%d", stages.invokes, repository.marks, len(repository.applied))
			}
			if len(repository.failures) != 1 || repository.failures[0].state != tt.wantState || repository.failures[0].cause.Error() != tt.want {
				t.Fatalf("failure records=%+v", repository.failures)
			}
		})
	}
}

func TestResumeInspectFailureRecordsTheUnderlyingCauseAndKeepsInvocationState(t *testing.T) {
	record, repository, stages, service := scriptedResumeFixture()
	inspectErr := errors.New("orca unreachable")
	stages.inspectErr = inspectErr
	repository.intentState = "unknown"
	_, err := service.Resume(t.Context(), resumeRequest(record))
	if !errors.Is(err, inspectErr) || !strings.Contains(err.Error(), "intent retained") {
		t.Fatalf("error=%v", err)
	}
	if len(repository.failures) != 1 || repository.failures[0].state != "unknown" || repository.failures[0].cause != inspectErr {
		t.Fatalf("failure records=%+v", repository.failures)
	}
}

func TestResumeClassifiesInvocationFailureByWhetherOrcaWasReached(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantState string
	}{
		{"orca proved not invoked", &port.OrcaError{Code: "E_ORCA_CLI", Invoked: false}, "not_invoked_proven"},
		{"orca invoked", &port.OrcaError{Code: "E_ORCA_CLI", Invoked: true}, "unknown"},
		{"opaque transport failure", errors.New("connection reset"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, repository, stages, service := scriptedResumeFixture()
			stages.invokeErr = tt.err
			_, err := service.Resume(t.Context(), resumeRequest(record))
			if !errors.Is(err, tt.err) || !strings.Contains(err.Error(), "requires execution reconcile; mutation was not repeated") {
				t.Fatalf("error=%v", err)
			}
			if len(repository.failures) != 1 || repository.failures[0].state != tt.wantState || repository.failures[0].cause != tt.err {
				t.Fatalf("failure records=%+v", repository.failures)
			}
			if stages.invokes != 1 || len(repository.applied) != 0 {
				t.Fatalf("invokes=%d applied=%d", stages.invokes, len(repository.applied))
			}
		})
	}
}

func TestResumeStopsWhenMarkingInvokingFailsBeforeAnyMutation(t *testing.T) {
	record, repository, stages, service := scriptedResumeFixture()
	markErr := errors.New("intent write failed")
	repository.markErr = markErr
	if _, err := service.Resume(t.Context(), resumeRequest(record)); !errors.Is(err, markErr) {
		t.Fatalf("error=%v", err)
	}
	if stages.invokes != 0 || len(repository.failures) != 0 {
		t.Fatalf("invokes=%d failures=%+v", stages.invokes, repository.failures)
	}
}

func TestResumeRecordsUnknownWhenApplyingTheReceiptFails(t *testing.T) {
	record, repository, _, service := scriptedResumeFixture()
	applyErr := errors.New("record moved")
	repository.applyErr = applyErr
	if _, err := service.Resume(t.Context(), resumeRequest(record)); !errors.Is(err, applyErr) {
		t.Fatalf("error=%v", err)
	}
	if len(repository.failures) != 1 || repository.failures[0].state != "unknown" || repository.failures[0].cause != applyErr {
		t.Fatalf("failure records=%+v", repository.failures)
	}
}

func TestResumeGivesUpAfterBoundedStageAttempts(t *testing.T) {
	record, repository, stages, service := scriptedResumeFixture()
	repository.applyPending = true
	// Every stage reports another pending stage; the loop must stop.
	_, err := service.Resume(t.Context(), resumeRequest(record))
	if err == nil || !strings.Contains(err.Error(), "did not complete the owner launch stages") {
		t.Fatalf("error=%v", err)
	}
	if stages.invokes != resumeStageAttempts || len(repository.applied) != resumeStageAttempts {
		t.Fatalf("invokes=%d applied=%d want %d", stages.invokes, len(repository.applied), resumeStageAttempts)
	}
}

func TestResumeSurfacesRepositoryFailuresAroundIntentLifecycle(t *testing.T) {
	boom := errors.New("repository failure")
	tests := []struct {
		name       string
		configure  func(*resumeScriptRepository, *ResumeService)
		wantBegins int
		wantLoads  int
	}{
		{"snapshot", func(r *resumeScriptRepository, _ *ResumeService) { r.snapshotErr = boom }, 0, 0},
		{"begin intent", func(r *resumeScriptRepository, _ *ResumeService) { r.beginErr = boom }, 1, 0},
		{"load intent", func(r *resumeScriptRepository, _ *ResumeService) { r.loadIntentErr = boom }, 1, 1},
		{"operation id", func(_ *resumeScriptRepository, s *ResumeService) {
			s.operationIDs = resumeOperationIDsFunc(func() (string, error) { return "", boom })
		}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, repository, _, service := scriptedResumeFixture()
			tt.configure(repository, service)
			result, err := service.Resume(t.Context(), resumeRequest(record))
			if !errors.Is(err, boom) || result.OK || result.ID != record.ID {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if repository.begins != tt.wantBegins || repository.loads != tt.wantLoads {
				t.Fatalf("begins=%d loads=%d", repository.begins, repository.loads)
			}
		})
	}
}

func TestResumeRejectsBeforeAnyOwnerObservation(t *testing.T) {
	boom := errors.New("collaborator failure")
	tests := []struct {
		name      string
		configure func(Record, *ResumeService) (ResumeRequest, *Record)
		want      string
	}{
		{"unconfirmed", func(r Record, _ *ResumeService) (ResumeRequest, *Record) {
			request := resumeRequest(r)
			request.Confirm = false
			return request, nil
		}, "execution resume requires confirm"},
		{"missing dependency", func(r Record, s *ResumeService) (ResumeRequest, *Record) {
			s.roleAgents = nil
			return resumeRequest(r), nil
		}, "resume service dependencies are required"},
		{"fence failure", func(r Record, s *ResumeService) (ResumeRequest, *Record) {
			s.fence = resumeFenceFunc(func(context.Context, string, func(context.Context) error) error { return boom })
			return resumeRequest(r), nil
		}, "collaborator failure"},
		{"actor not verified", func(r Record, s *ResumeService) (ResumeRequest, *Record) {
			s.verifier = leaseVerifier(func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
				return "", leasedomain.ProcessReceipt{}, boom
			})
			return resumeRequest(r), nil
		}, "collaborator failure"},
		{"no orca binding", func(r Record, _ *ResumeService) (ResumeRequest, *Record) {
			r.Stable.Execution.Orca = nil
			return resumeRequest(r), &r
		}, "execution resume requires an existing Orca binding"},
		{"non canonical cwd", func(r Record, _ *ResumeService) (ResumeRequest, *Record) {
			request := resumeRequest(r)
			request.CWD = "/somewhere-else"
			return request, nil
		}, ""},
		{"artifact verification failure", func(r Record, s *ResumeService) (ResumeRequest, *Record) {
			s.artifacts = resumeArtifactsFunc(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
				return leasecontract.ResumeArtifacts{}, boom
			})
			return resumeRequest(r), nil
		}, "collaborator failure"},
		{"owner inventory failure", func(r Record, s *ResumeService) (ResumeRequest, *Record) {
			s.owners = resumeOwnersFunc(func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, error) {
				return leasedomain.ResumeInventory{}, boom
			})
			return resumeRequest(r), nil
		}, "collaborator failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, repository, _, service := scriptedResumeFixture()
			request, replaced := tt.configure(record, service)
			if replaced != nil {
				repository.record = *replaced
			}
			result, err := service.Resume(t.Context(), request)
			if err == nil || !strings.Contains(err.Error(), tt.want) || result.OK {
				t.Fatalf("result=%+v err=%v want %q", result, err, tt.want)
			}
			if repository.begins != 0 || repository.loads != 0 {
				t.Fatalf("intent lifecycle started: begins=%d loads=%d", repository.begins, repository.loads)
			}
		})
	}
}
