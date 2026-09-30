package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ResolvePublicationDefaults(record model.IssueOpsRecord, req model.RemotePullRequestRequest) (model.RemotePullRequestRequest, error) {
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

func ResolveBodySyncProvider(record model.IssueOpsRecord, override string) (string, error) {
	name := strings.TrimSpace(override)
	if name == "" {
		name = strings.TrimSpace(ResolveRecordProvider(record))
	}
	if name == "" {
		return "", fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set or pass --provider github|gitlab")
	}
	return name, nil
}
