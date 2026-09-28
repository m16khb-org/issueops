package issueops

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestSealIssueCreateRequestReusesOperationAndBindsExactBody(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle"}
	request := model.IssueOpsIssueCreateIntentRequest{Title: "Title", StartedAt: "2026-09-28T00:00:00Z"}
	sealed, body := SealIssueCreateRequest(record, request, " Body ")
	digest := sha256.Sum256([]byte(body))
	if len(sealed.OperationID) != 32 || sealed.BodySHA256 != fmt.Sprintf("%x", digest) || body != "Body\n\n<!-- issueops:issue-create:"+sealed.OperationID+" -->" {
		t.Fatalf("seal = %+v body=%s", sealed, body)
	}
	record.IssueCreateIntent = &model.IssueOpsIssueCreateIntent{OperationID: sealed.OperationID, Marker: "<!-- issueops:issue-create:" + sealed.OperationID + " -->"}
	request.StartedAt = "2026-09-29T00:00:00Z"
	retry, retryBody := SealIssueCreateRequest(record, request, "Body")
	if retry.OperationID != sealed.OperationID || retryBody != body || retry.BodySHA256 != sealed.BodySHA256 {
		t.Fatal("retry changed sealed identity")
	}
	_, empty := SealIssueCreateRequest(record, request, " ")
	if strings.Contains(empty, "\n") || empty != record.IssueCreateIntent.Marker {
		t.Fatalf("empty body=%q", empty)
	}
}

func TestIssueCreateFailureClassificationPreservesInvocationEvidence(t *testing.T) {
	for _, tc := range []struct {
		notInvoked bool
		url        string
		want       string
	}{
		{true, "", model.IssueCreateIntentNotInvoked}, {true, "https://github.com/acme/repo/issues/1", model.IssueCreateIntentNotInvoked},
		{false, " ", model.IssueCreateIntentInvokedUnknown}, {false, "https://github.com/acme/repo/issues/1", model.IssueCreateIntentURLObserved},
	} {
		if got := ClassifyIssueCreateFailure(tc.notInvoked, tc.url); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}
