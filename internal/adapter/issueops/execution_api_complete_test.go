package issueops

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestExecuteExecutionCompleteRequiresHandler(t *testing.T) {
	result, err := testExecutionService().Execute(context.Background(), t.TempDir(), issueopscontract.ExecutionActionRequest{Action: issueopscontract.ExecutionActionComplete, ID: "io-complete"}, issueopsport.ExecutionActionDependencies{})
	if !errors.Is(err, issueopscontract.ErrCompleteHandlerUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if got, ok := result.(issueopscontract.ExecutionResult); !ok || got.OK || got.ID != "io-complete" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteExecutionCompleteDelegatesExactRequest(t *testing.T) {
	request := issueopscontract.ExecutionActionRequest{Action: issueopscontract.ExecutionActionComplete, ID: "io-complete", Generation: 7, CWD: "/canonical", FinalHead: "head", VerificationReportPath: "/canonical/report", Verification: []string{"test"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/7", Confirm: true}
	var gotRoot string
	var got issueopscontract.ExecutionCompleteRequest
	result, err := testExecutionService().Execute(context.Background(), t.TempDir(), request, issueopsport.ExecutionActionDependencies{Complete: func(_ context.Context, stateRoot string, req issueopscontract.ExecutionCompleteRequest) (issueopscontract.ExecutionResult, error) {
		gotRoot, got = stateRoot, req
		return issueopscontract.ExecutionResult{OK: true, ID: req.ID}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot == "" {
		t.Fatal("state root was not delegated")
	}
	want := issueopscontract.ExecutionCompleteRequest{ID: request.ID, Generation: request.Generation, Actor: request.Actor, CWD: request.CWD, FinalHead: request.FinalHead, VerificationReportPath: request.VerificationReportPath, Verification: request.Verification, RemoteArtifactURL: request.RemoteArtifactURL, Confirm: request.Confirm}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
	if output := result.(issueopscontract.ExecutionResult); !output.OK || output.ID != request.ID {
		t.Fatalf("result = %+v", output)
	}
}
