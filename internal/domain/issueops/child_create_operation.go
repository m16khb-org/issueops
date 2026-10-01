package issueops

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
	"net/url"
	"strconv"
)

func ChildRequestDigest(provider, parent, title, body string, labels, assignees []string) string {
	labels = append([]string(nil), labels...)
	sort.Strings(labels)
	assignees = append([]string(nil), assignees...)
	sort.Strings(assignees)
	payload, _ := json.Marshal([]any{provider, parent, strings.TrimSpace(title), strings.TrimSpace(body), labels, assignees})
	return fmt.Sprintf("%x", sha256.Sum256(payload))
}

func FindChildOperation(record model.IssueOpsRecord, id, digest string) (model.ChildCreateOperation, bool) {
	for _, op := range record.ChildCreateOperations {
		if id != "" && op.OperationID == id || id == "" && op.Origin == "implicit" && op.RequestSHA256 == digest {
			return op, true
		}
	}
	return model.ChildCreateOperation{}, false
}

func SealChildOperation(record model.IssueOpsRecord, id, origin, provider, title, body, digest, now string, labels, assignees []string) (model.ChildCreateOperation, string, error) {
	marker := "<!-- issueops:child-create:" + id + " -->"
	body = strings.TrimSpace(body) + "\n\n" + marker
	project, err := childProjectAuthority(record.IssueURL, provider)
	if err != nil {
		return model.ChildCreateOperation{}, "", err
	}
	op := model.ChildCreateOperation{IssueOpsIssueCreateIntent: model.IssueOpsIssueCreateIntent{OperationID: id, Marker: marker, Provider: provider, ProjectAuthority: project, Title: strings.TrimSpace(title), BodySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(body))), Labels: labels, Assignees: assignees, Status: model.IssueCreateIntentPending, Attempt: 1, StartedAt: now, UpdatedAt: now}, Origin: origin, RequestSHA256: digest, ParentURL: record.IssueURL}
	if record.Execution != nil {
		op.Generation = record.Execution.Lease.Generation
		op.Holder = record.Execution.Lease.Holder
		op.CWD = record.Execution.Workspace.Root
	}
	if err := model.ValidateChildCreateOperations([]model.ChildCreateOperation{op}); err != nil {
		return op, "", err
	}
	if op.ProjectAuthority == "" {
		return op, "", fmt.Errorf("child provider must match linked parent project")
	}
	return op, body, nil
}

func ValidateChildOperationAuthority(record model.IssueOpsRecord, op model.ChildCreateOperation) error {
	project, err := childProjectAuthority(record.IssueURL, op.Provider)
	if err != nil || record.IssueURL != op.ParentURL || project != op.ProjectAuthority {
		return fmt.Errorf("child operation parent/project changed")
	}
	if record.Execution == nil {
		if op.Generation != 0 {
			return fmt.Errorf("child operation authority changed")
		}
		return nil
	}
	if record.Execution.Lease.Generation != op.Generation || !reflect.DeepEqual(record.Execution.Lease.Holder, op.Holder) || record.Execution.Workspace.Root != op.CWD {
		return fmt.Errorf("child operation authority changed; explicit reconcile required")
	}
	return nil
}

func BeginChildOperation(record model.IssueOpsRecord, request model.ChildCreateOperation, explicitID string) (model.IssueOpsRecord, model.ChildCreateOperation, bool, error) {
	if err := ValidateChildOperationAuthority(record, request); err != nil {
		return record, request, false, err
	}
	if op, found := FindChildOperation(record, explicitID, request.RequestSHA256); found {
		if op.RequestSHA256 != request.RequestSHA256 {
			return record, op, false, fmt.Errorf("child operation payload mismatch")
		}
		if err := ValidateChildOperationAuthority(record, op); err != nil {
			return record, op, false, err
		}
		if op.Status == model.IssueCreateIntentCompleted {
			return record, op, false, nil
		}
		if op.Status != model.IssueCreateIntentNotInvoked {
			return record, op, false, fmt.Errorf("unresolved child operation; reconcile before creating")
		}
		request = op
		request.Status = model.IssueCreateIntentPending
		request.Failure = ""
		request.Attempt++
	}
	for _, op := range record.ChildCreateOperations {
		if op.OperationID != request.OperationID && op.Status != model.IssueCreateIntentCompleted {
			return record, op, false, fmt.Errorf("unresolved child operation; reconcile before creating")
		}
	}
	record = putChildOperation(record, request)
	return record, request, true, nil
}

func RecordChildOutcome(record model.IssueOpsRecord, id, status, url, failure, now string) (model.IssueOpsRecord, error) {
	op, ok := FindChildOperation(record, id, "")
	if !ok {
		return record, fmt.Errorf("child operation not found")
	}
	if err := ValidateChildOperationAuthority(record, op); err != nil {
		return record, err
	}
	if err := ValidateIssueCreateTransition(op.Status, status); err != nil {
		return record, err
	}
	if url != "" {
		project, err := childProjectAuthority(url, op.Provider)
		if err != nil || project != op.ProjectAuthority {
			return record, fmt.Errorf("child operation project mismatch")
		}
		if url == op.ParentURL || op.CanonicalURL != "" && url != op.CanonicalURL {
			return record, fmt.Errorf("child operation canonical URL changed")
		}
		op.CanonicalURL = url
	}
	op.Status, op.Failure, op.UpdatedAt = status, failure, now
	return putChildOperation(record, op), nil
}

func CompleteChildOperation(record model.IssueOpsRecord, id, url, now string) (model.IssueOpsRecord, error) {
	updated, err := RecordChildOutcome(record, id, model.IssueCreateIntentCompleted, url, "", now)
	if err != nil {
		return record, err
	}
	op, _ := FindChildOperation(updated, id, "")
	for _, link := range updated.IssueLinks {
		if link.Type == "child" && link.URL == url {
			return updated, nil
		}
	}
	return AppendIssueRelation(updated, model.IssueOpsIssueLink{Type: "child", URL: url, Title: op.Title}, now)
}

func RebindChildOperation(record model.IssueOpsRecord, id string) (model.IssueOpsRecord, error) {
	op, ok := FindChildOperation(record, id, "")
	if !ok {
		return record, fmt.Errorf("child operation not found")
	}
	if record.IssueURL != op.ParentURL {
		return record, fmt.Errorf("child operation parent changed")
	}
	op.Generation, op.Holder, op.CWD = 0, nil, ""
	if record.Execution != nil {
		op.Generation = record.Execution.Lease.Generation
		op.Holder = record.Execution.Lease.Holder
		op.CWD = record.Execution.Workspace.Root
	}
	return putChildOperation(record, op), nil
}

func putChildOperation(record model.IssueOpsRecord, op model.ChildCreateOperation) model.IssueOpsRecord {
	record.ChildCreateOperations = append([]model.ChildCreateOperation(nil), record.ChildCreateOperations...)
	for i, old := range record.ChildCreateOperations {
		if old.OperationID == op.OperationID {
			record.ChildCreateOperations[i] = op
			record.UpdatedAt = op.UpdatedAt
			return record
		}
	}
	record.ChildCreateOperations = append(record.ChildCreateOperations, op)
	record.UpdatedAt = op.UpdatedAt
	return record
}

func childProjectAuthority(raw, provider string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid child project URL")
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	projectEnd := 0
	switch provider {
	case "github":
		if !strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) != 4 || parts[2] != "issues" {
			return "", fmt.Errorf("invalid github child URL")
		}
		projectEnd = 2
	case "gitlab":
		if strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) < 5 || parts[len(parts)-3] != "-" || (parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") {
			return "", fmt.Errorf("invalid gitlab child URL")
		}
		projectEnd = len(parts) - 3
	default:
		return "", fmt.Errorf("invalid child provider")
	}
	number := parts[len(parts)-1]
	if _, err := strconv.ParseUint(number, 10, 64); err != nil || number == "" || strings.ContainsAny(number, "+-") {
		return "", fmt.Errorf("invalid child issue number")
	}
	for _, part := range parts[:projectEnd] {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid child project path")
		}
	}
	host := strings.ToLower(parsed.Host)
	if parsed.Port() == "443" {
		host = strings.TrimSuffix(host, ":443")
	}
	return host + "/" + strings.ToLower(strings.Join(parts[:projectEnd], "/")), nil
}
