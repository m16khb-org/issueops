package issueopslease

import (
	"context"
	"errors"
	"reflect"
	"testing"

	leaseapp "issueops/internal/application/issueopslease"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestReseedNextCommandsKeepTheirRenderer(t *testing.T) {
	var calls []string
	makeCommand := func(owner string) ClaimCommand {
		handler := ReseedHandler{nextCommand: func(id string, generation uint64, _ issueopscontract.NativeActor, _ string) string {
			if id != "cycle" || generation != 7 {
				t.Fatal("response fields lost")
			}
			calls = append(calls, owner)
			return owner
		}}
		return handler.nextCommand
	}
	first, second := makeCommand("first"), makeCommand("second")
	for _, render := range []ClaimCommand{first, second, first} {
		render("cycle", 7, issueopscontract.NativeActor{}, "")
	}
	if !reflect.DeepEqual(calls, []string{"first", "second", "first"}) {
		t.Fatalf("renderers crossed instances: %v", calls)
	}
}

func TestReseedHandlerRejectsMissingRendererBeforeService(t *testing.T) {
	reseed, err := NewReseedHandler(&leaseapp.ReseedService{}, nil)(context.Background(), "state", issueopscontract.ExecutionReseedRequest{ID: "reseed"})
	if !errors.Is(err, issueopscontract.ErrReseedHandlerUnavailable) || reseed.ID != "reseed" {
		t.Fatalf("reseed=%+v err=%v", reseed, err)
	}
}
