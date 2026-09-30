package issueopslease

import (
	issueopscontract "issueops/internal/contract/issueops"
)

import (
	"context"
	"errors"
	"testing"
)

func TestReseedHandlerRequiresService(t *testing.T) {
	_, err := NewReseedHandler(nil, nil)(context.Background(), "/state", issueopscontract.ExecutionReseedRequest{ID: "io-reseed"})
	if !errors.Is(err, issueopscontract.ErrReseedHandlerUnavailable) {
		t.Fatalf("error=%v", err)
	}
}
