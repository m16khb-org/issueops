package issueopscleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func githubRecord(generation uint64) model.IssueOpsRecord {
	record := model.IssueOpsRecord{ID: "cycle", IssueURL: "https://github.com/example/repo/issues/1"}
	if generation != 0 {
		record.Execution = &model.Execution{Lease: model.WriteLease{Generation: generation}}
	}
	return record
}

func requireInvocationError(t *testing.T, err error, want error) {
	t.Helper()
	failure, ok := errors.AsType[*port.CleanupInvocationError](err)
	if !ok || (want != nil && !errors.Is(failure, want)) {
		t.Fatalf("err=%v, want CleanupInvocationError wrapping %v", err, want)
	}
}

func requireBindingFailure(t *testing.T, err error) {
	t.Helper()
	requireInvocationError(t, err, nil)
	if !strings.Contains(err.Error(), "provenance observation failed") {
		t.Fatalf("err=%v, want provenance observation failure", err)
	}
}

func TestRemoteBranchInvocationRefusesBeforeRunningCleanup(t *testing.T) {
	readFailure, providerFailure := errors.New("record unreadable"), errors.New("provider unavailable")
	runner := func(context.Context, model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error) {
		t.Fatal("remote branch cleanup ran despite a refused precondition")
		return model.CleanupRemoteBranchResult{}, nil
	}
	cases := []struct {
		name    string
		service Invocation
		want    error
		message string
	}{
		{name: "record unreadable", service: Invocation{Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readFailure }}, want: readFailure},
		{name: "provider undeterminable", service: Invocation{Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: "cycle"}, nil }}, message: "cannot determine provider"},
		{name: "provider construction fails", service: Invocation{
			Read:     func(string) (model.IssueOpsRecord, error) { return githubRecord(0), nil },
			Provider: func(string) (port.IssueProvider, error) { return nil, providerFailure },
		}, want: providerFailure},
		{name: "merge verification unavailable", service: Invocation{
			Read:     func(string) (model.IssueOpsRecord, error) { return githubRecord(0), nil },
			Provider: func(string) (port.IssueProvider, error) { return nil, nil },
		}, message: "merge verification is not configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.service.RunRemoteBranch = runner
			result, err := tc.service.RemoteBranch(t.Context(), model.CleanupRemoteBranchRequest{ID: "cycle"})
			requireInvocationError(t, err, tc.want)
			if tc.message != "" && !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("err=%v, want message %q", err, tc.message)
			}
			if result.NextCommand != "" || result.OK {
				t.Fatalf("refusal returned a result: %+v", result)
			}
		})
	}
}

func TestRemoteBranchInvocationBindsNextCommandToRecordGeneration(t *testing.T) {
	runFailure, bindFailure := errors.New("delete refused"), errors.New("binary unavailable")
	for _, tc := range []struct {
		name        string
		observeErr  error
		wantBindErr bool
	}{
		{name: "binds and keeps executor error", wantBindErr: false},
		{name: "binding failure replaces executor error", observeErr: bindFailure, wantBindErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := 0
			service := Invocation{
				Read:                       func(string) (model.IssueOpsRecord, error) { return githubRecord(5), nil },
				Provider:                   func(name string) (port.IssueProvider, error) { return nil, nil },
				MergeVerificationAvailable: true,
				Provenance:                 invocationObserver{&observed, tc.observeErr},
				RunRemoteBranch: func(_ context.Context, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error) {
					return model.CleanupRemoteBranchResult{ID: req.ID, NextCommand: "issueops cleanup remote-branch --id cycle --preview"}, runFailure
				},
			}
			result, err := service.RemoteBranch(t.Context(), model.CleanupRemoteBranchRequest{ID: "cycle"})
			if observed != 1 {
				t.Fatalf("provenance observed %d times, want 1", observed)
			}
			if tc.wantBindErr {
				requireBindingFailure(t, err)
				if errors.Is(err, runFailure) {
					t.Fatal("executor error leaked through binding failure")
				}
				return
			}
			if !errors.Is(err, runFailure) || !strings.Contains(result.NextCommand, "--generated-for-generation 5") {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestLinkedBranchInvocationReadsRecordAndBindsNextCommand(t *testing.T) {
	readFailure, bindFailure, runFailure := errors.New("record unreadable"), errors.New("binary unavailable"), errors.New("branch refused")
	t.Run("record unreadable", func(t *testing.T) {
		service := Invocation{
			Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readFailure },
			RunLinkedBranch: func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
				t.Fatal("linked branch cleanup ran without a record")
				return model.CleanupLinkedBranchResult{}, nil
			},
		}
		_, err := service.LinkedBranch(t.Context(), model.CleanupLinkedBranchRequest{ID: "cycle"})
		requireInvocationError(t, err, readFailure)
	})
	t.Run("binds and keeps executor error", func(t *testing.T) {
		observed := 0
		service := Invocation{
			Read: func(string) (model.IssueOpsRecord, error) { return githubRecord(7), nil }, Provenance: invocationObserver{&observed, nil},
			RunLinkedBranch: func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
				return model.CleanupLinkedBranchResult{NextCommand: "issueops cleanup linked-branch --id cycle --preview"}, runFailure
			},
		}
		result, err := service.LinkedBranch(t.Context(), model.CleanupLinkedBranchRequest{ID: "cycle"})
		if !errors.Is(err, runFailure) || !strings.Contains(result.NextCommand, "--generated-for-generation 7") || observed != 1 {
			t.Fatalf("result=%+v err=%v observed=%d", result, err, observed)
		}
	})
	t.Run("binding failure", func(t *testing.T) {
		observed := 0
		service := Invocation{
			Read: func(string) (model.IssueOpsRecord, error) { return githubRecord(7), nil }, Provenance: invocationObserver{&observed, bindFailure},
			RunLinkedBranch: func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
				return model.CleanupLinkedBranchResult{NextCommand: "issueops cleanup linked-branch --id cycle --preview"}, runFailure
			},
		}
		_, err := service.LinkedBranch(t.Context(), model.CleanupLinkedBranchRequest{ID: "cycle"})
		requireBindingFailure(t, err)
	})
	t.Run("ungenerationed record leaves command untouched", func(t *testing.T) {
		observed := 0
		service := Invocation{
			Read: func(string) (model.IssueOpsRecord, error) { return githubRecord(0), nil }, Provenance: invocationObserver{&observed, nil},
			RunLinkedBranch: func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
				return model.CleanupLinkedBranchResult{NextCommand: "issueops cleanup linked-branch --id cycle"}, nil
			},
		}
		result, err := service.LinkedBranch(t.Context(), model.CleanupLinkedBranchRequest{ID: "cycle"})
		if err != nil || result.NextCommand != "issueops cleanup linked-branch --id cycle" || observed != 0 {
			t.Fatalf("result=%+v err=%v observed=%d", result, err, observed)
		}
	})
}

func TestAbandonInvocationReportsUnreadableRecordWhenBindingNeeded(t *testing.T) {
	readFailure, bindFailure := errors.New("record unreadable"), errors.New("binary unavailable")
	run := func(context.Context, model.CleanupAbandonRequest) (model.CleanupAbandonResult, error) {
		return model.CleanupAbandonResult{NextCommand: "issueops cleanup abandon --id cycle --preview"}, nil
	}
	_, err := (Invocation{Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readFailure }, RunAbandon: run}).Abandon(t.Context(), model.CleanupAbandonRequest{ID: "cycle"})
	requireInvocationError(t, err, readFailure)
	observed := 0
	_, err = (Invocation{Read: func(string) (model.IssueOpsRecord, error) { return githubRecord(2), nil }, Provenance: invocationObserver{&observed, bindFailure}, RunAbandon: run}).Abandon(t.Context(), model.CleanupAbandonRequest{ID: "cycle"})
	requireBindingFailure(t, err)
}

func TestFinishInvocationRefusesBeforeRunningCleanup(t *testing.T) {
	readFailure, providerFailure := errors.New("record unreadable"), errors.New("provider unavailable")
	runner := func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error) {
		t.Fatal("finish ran despite a refused precondition")
		return model.CleanupFinishResult{}, nil
	}
	cwd := func() (string, error) { return "/cwd", nil }
	cases := []struct {
		name     string
		service  Invocation
		override string
		want     error
		message  string
	}{
		{name: "record unreadable", service: Invocation{Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readFailure }}, want: readFailure},
		{name: "provider undeterminable", service: Invocation{Read: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: "cycle"}, nil }}, message: "pass --provider"},
		{name: "provider construction fails", service: Invocation{
			Read:     func(string) (model.IssueOpsRecord, error) { return githubRecord(0), nil },
			Provider: func(string) (port.IssueProvider, error) { return nil, providerFailure },
		}, want: providerFailure},
		{name: "override provider is validated even when record has none", service: Invocation{
			Read:     func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: "cycle"}, nil },
			Provider: func(name string) (port.IssueProvider, error) { return nil, errors.New("unknown provider " + name) },
		}, override: "gitea", message: "unknown provider gitea"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.service.CurrentDirectory, tc.service.RunFinish = cwd, runner
			_, err := tc.service.Finish(t.Context(), model.CleanupFinishRequest{ID: "cycle"}, tc.override)
			requireInvocationError(t, err, tc.want)
			if tc.message != "" && !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("err=%v, want message %q", err, tc.message)
			}
		})
	}
}
