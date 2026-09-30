package issueopslease

import (
	"context"
	"errors"
	leaseapp "issueops/internal/application/issueopslease"
	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"reflect"
	"testing"
)

func TestResumeNextCommandsKeepTheirRenderer(t *testing.T) {
	var calls []string
	makeCommand := func(owner string) func(string, uint64, leasecontract.ResumeArtifacts) string {
		handler := ResumeHandler{nextCommand: func(id string, generation uint64, token, issue, packet string) string {
			if id != "cycle" || generation != 7 || token != "token" || issue != "issue" || packet != "packet" {
				t.Fatal("response fields lost")
			}
			calls = append(calls, owner)
			return owner
		}}
		return handler.resumeNextCommand
	}
	first, second := makeCommand("first"), makeCommand("second")
	artifacts := leasecontract.ResumeArtifacts{ClaimTokenPath: "token", IssueBodySHA256: "issue", ContextPacketSHA256: "packet"}
	for _, render := range []func(string, uint64, leasecontract.ResumeArtifacts) string{first, second, first} {
		render("cycle", 7, artifacts)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second", "first"}) {
		t.Fatalf("renderers crossed instances: %v", calls)
	}
}

func TestLeaseHandlersRejectMissingRendererBeforeService(t *testing.T) {
	ctx := context.Background()
	resume, err := NewResumeHandler(&leaseapp.ResumeService{}, nil)(ctx, "state", issueopscontract.ExecutionResumeRequest{ID: "resume"})
	if !errors.Is(err, issueopscontract.ErrResumeHandlerUnavailable) || resume.ID != "resume" {
		t.Fatalf("resume=%+v err=%v", resume, err)
	}
	reseed, err := NewReseedHandler(&leaseapp.ReseedService{}, nil)(ctx, "state", issueopscontract.ExecutionReseedRequest{ID: "reseed"})
	if !errors.Is(err, issueopscontract.ErrReseedHandlerUnavailable) || reseed.ID != "reseed" {
		t.Fatalf("reseed=%+v err=%v", reseed, err)
	}
}
