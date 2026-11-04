package github

import (
	"context"
	"encoding/json"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	"issueops/internal/port"
)

func (Provider) ReadIssueSnapshot(ctx context.Context, req executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return executionissue.Snapshot{}, err
	}
	issueURL := strings.TrimSpace(req.URL)
	if issueURL == "" {
		return executionissue.Snapshot{}, fmt.Errorf("issue URL is required")
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "gh", "issue", "view", issueURL, "--json", "url,body,state")
	if err != nil {
		return executionissue.Snapshot{}, fmt.Errorf("gh issue snapshot read failed: %w", err)
	}
	var payload struct {
		URL   string `json:"url"`
		Body  string `json:"body"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return executionissue.Snapshot{}, fmt.Errorf("parse gh issue snapshot: %w", err)
	}
	if strings.TrimSpace(payload.URL) != issueURL {
		return executionissue.Snapshot{}, fmt.Errorf("gh issue snapshot URL does not match the linked issue")
	}
	return executionissue.Snapshot{URL: issueURL, Body: payload.Body, State: payload.State}, nil
}

var _ port.ExecutionIssueSnapshotReader = Provider{}
