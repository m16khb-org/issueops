package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ResolvePublicationDefaults fills the omitted fields of a pull request
// request from the record: the ID, the provider (derived from the issue URL),
// the head branch (the record branch), and the base branch (the prepared
// base). It fails when no provider can be determined.
func ResolvePublicationDefaults(record model.Record, req model.RemotePullRequestRequest) (model.RemotePullRequestRequest, error) {
	req.ID = record.ID
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Provider == "" {
		req.Provider = strings.TrimSpace(ResolveRecordProvider(record))
	}
	if req.Provider == "" {
		return req, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	req.Head = strings.TrimSpace(req.Head)
	if req.Head == "" {
		req.Head = strings.TrimSpace(record.Branch)
	}
	if req.Base == "" && record.BranchPrepare != nil {
		req.Base = record.BranchPrepare.BaseBranch
	}
	return req, nil
}

// ResolveBodySyncProvider returns the provider for a body sync: the trimmed
// override when given, otherwise the provider derived from the record. It
// fails when neither yields one.
func ResolveBodySyncProvider(record model.Record, override string) (string, error) {
	name := strings.TrimSpace(override)
	if name == "" {
		name = strings.TrimSpace(ResolveRecordProvider(record))
	}
	if name == "" {
		return "", fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set or pass --provider github|gitlab")
	}
	return name, nil
}
