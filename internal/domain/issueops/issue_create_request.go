package issueops

import (
	"crypto/sha256"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateIssueCreateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("issue title is required")
	}
	return nil
}

func SealIssueCreateRequest(record model.IssueOpsRecord, request model.IssueOpsIssueCreateIntentRequest, body string) (model.IssueOpsIssueCreateIntentRequest, string) {
	marker := ""
	if record.IssueCreateIntent != nil {
		request.OperationID = record.IssueCreateIntent.OperationID
		marker = record.IssueCreateIntent.Marker
	} else {
		seed := sha256.Sum256([]byte(record.ID + "\n" + request.StartedAt + "\n" + request.Title))
		request.OperationID = fmt.Sprintf("%x", seed[:16])
		marker = "<!-- issueops:issue-create:" + request.OperationID + " -->"
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = marker
	} else {
		body += "\n\n" + marker
	}
	digest := sha256.Sum256([]byte(body))
	request.BodySHA256 = fmt.Sprintf("%x", digest[:])
	return request, body
}

func ClassifyIssueCreateFailure(notInvoked bool, url string) string {
	if notInvoked {
		return model.IssueCreateIntentNotInvoked
	}
	if strings.TrimSpace(url) != "" {
		return model.IssueCreateIntentURLObserved
	}
	return model.IssueCreateIntentInvokedUnknown
}
