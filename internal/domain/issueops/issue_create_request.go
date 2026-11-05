package issueops

import (
	"crypto/sha256"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ValidateIssueCreateTitle requires a non-blank issue title.
func ValidateIssueCreateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("issue title is required (--title)")
	}
	return nil
}

// SealIssueCreateRequest fixes the operation ID and body digest of an issue
// create request. It reuses the operation ID and marker of an existing
// intent, or derives them from the cycle ID, start time, and title. The
// marker is appended to the trimmed body, and the SHA-256 of the marked body
// becomes BodySHA256. It returns the request and the marked body.
func SealIssueCreateRequest(record model.Record, request model.IssueCreateIntentRequest, body string) (model.IssueCreateIntentRequest, string) {
	var marker string
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

// ClassifyIssueCreateFailure maps a failed create call to an intent status:
// not invoked when the call never ran, URL observed when a URL came back, and
// invoked-unknown otherwise.
func ClassifyIssueCreateFailure(notInvoked bool, url string) string {
	if notInvoked {
		return model.IssueCreateIntentNotInvoked
	}
	if strings.TrimSpace(url) != "" {
		return model.IssueCreateIntentURLObserved
	}
	return model.IssueCreateIntentInvokedUnknown
}
