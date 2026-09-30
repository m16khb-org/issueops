package issueopslease

import (
	issueopscontract "issueops/internal/contract/issueops"
)

import (
	"context"
	"errors"
	"testing"
)

func TestResumeHandlerFailsClosedWithoutService(t *testing.T) {
	result, err := NewResumeHandler(nil, nil)(context.Background(), "state", issueopscontract.ExecutionResumeRequest{ID: "io-resume"})
	if !errors.Is(err, issueopscontract.ErrResumeHandlerUnavailable) || result.ID != "io-resume" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
