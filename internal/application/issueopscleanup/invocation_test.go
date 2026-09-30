package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

func TestFinishInvocationPreservesObservationOrderAndStopsBeforeEffects(t *testing.T) {
	denied := errors.New("cwd unavailable")
	var events []string
	service := Invocation{
		Read: func(string) (model.IssueOpsRecord, error) {
			events = append(events, "read")
			return model.IssueOpsRecord{ID: "cycle", IssueURL: "https://github.com/example/repo/issues/1"}, nil
		},
		Provider:         func(string) (port.IssueProvider, error) { events = append(events, "provider"); return nil, nil },
		CurrentDirectory: func() (string, error) { events = append(events, "cwd"); return "", denied },
		RunFinish: func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error) {
			t.Fatal("cleanup ran without cwd")
			return model.CleanupFinishResult{}, nil
		},
	}
	_, err := service.Finish(context.Background(), model.CleanupFinishRequest{ID: "cycle"}, "")
	failure, ok := errors.AsType[*port.CleanupInvocationError](err)
	if !ok || !errors.Is(failure, denied) || !reflect.DeepEqual(events, []string{"read", "provider", "cwd"}) {
		t.Fatalf("err=%v events=%v", err, events)
	}
}
func TestAbandonInvocationOnlyReadsWhenCommandNeedsBinding(t *testing.T) {
	failed := errors.New("executor refused")
	for _, command := range []string{"", "issueops cleanup abandon --id cycle --preview"} {
		t.Run(command, func(t *testing.T) {
			var events []string
			service := Invocation{Read: func(string) (model.IssueOpsRecord, error) {
				events = append(events, "read")
				return model.IssueOpsRecord{}, nil
			}, RunAbandon: func(context.Context, model.CleanupAbandonRequest) (model.CleanupAbandonResult, error) {
				events = append(events, "execute")
				return model.CleanupAbandonResult{NextCommand: command}, failed
			}}
			result, err := service.Abandon(context.Background(), model.CleanupAbandonRequest{ID: "cycle"})
			want := []string{"execute"}
			if command != "" {
				want = append(want, "read")
			}
			if !errors.Is(err, failed) || result.NextCommand != command || !reflect.DeepEqual(events, want) {
				t.Fatalf("result=%+v err=%v events=%v", result, err, events)
			}
		})
	}
}

type invocationObserver struct {
	calls *int
	err   error
}

func (o invocationObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	*o.calls++
	return provenanceport.Receipt{ExecutablePath: "/repo/bin/issueops", ExecutableSHA256: strings.Repeat("a", 64)}, o.err
}
func TestFinishInvocationKeepsInitialGenerationAndErrorPrecedence(t *testing.T) {
	executorFailure := errors.New("executor refused")
	for _, tc := range []struct {
		name                     string
		executionErr, observeErr error
		wantCalls                int
		wantInvocationError      bool
	}{
		{name: "result error still binds", executionErr: executorFailure, wantCalls: 1},
		{name: "observation error skips binding", executionErr: &port.CleanupFinishObservationError{Err: executorFailure}, wantInvocationError: true},
		{name: "binding error takes precedence", executionErr: executorFailure, observeErr: errors.New("binary unavailable"), wantCalls: 1, wantInvocationError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			generation := uint64(3)
			observed := 0
			reads := 0
			service := Invocation{Read: func(string) (model.IssueOpsRecord, error) {
				reads++
				return model.IssueOpsRecord{ID: "cycle", IssueURL: "https://github.com/example/repo/issues/1", Execution: &model.Execution{Lease: model.WriteLease{Generation: generation}}}, nil
			}, Provider: func(string) (port.IssueProvider, error) { return nil, nil }, CurrentDirectory: func() (string, error) { return "/cwd", nil }, Provenance: invocationObserver{&observed, tc.observeErr},
				RunFinish: func(_ context.Context, r model.CleanupFinishRequest, _ port.IssueProvider) (model.CleanupFinishResult, error) {
					if r.ID != "cycle" || r.CWD != "/cwd" {
						t.Fatalf("request=%+v", r)
					}
					generation = 4
					return model.CleanupFinishResult{ID: "cycle", NextCommand: "issueops cleanup finish --id cycle --preview --json"}, tc.executionErr
				},
			}
			result, err := service.Finish(context.Background(), model.CleanupFinishRequest{ID: "cycle"}, "")
			_, isInvocation := errors.AsType[*port.CleanupInvocationError](err)
			if reads != 1 || observed != tc.wantCalls || isInvocation != tc.wantInvocationError {
				t.Fatalf("reads=%d observes=%d err=%v", reads, observed, err)
			}
			if tc.observeErr != nil {
				if errors.Is(err, executorFailure) {
					t.Fatal("executor error hid binding error")
				}
			} else if !errors.Is(err, executorFailure) {
				t.Fatalf("lost executor error: %v", err)
			}
			if !tc.wantInvocationError && !strings.Contains(result.NextCommand, "--generated-for-generation 3") {
				t.Fatalf("initial generation lost: %s", result.NextCommand)
			}
		})
	}
}
