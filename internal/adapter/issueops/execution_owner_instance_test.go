package issueops

import (
	model "issueops/internal/contract/issueops"
	"os"
	"strings"
	"testing"
)

func TestPreparedOwnerBuilderKeepsItsPromptTemplate(t *testing.T) {
	record, req := ownerPacketFixture()
	record.Execution.Workspace.Root = t.TempDir()
	snapshot := model.OwnerSnapshot{Issue: model.OwnerIssue{URL: record.IssueURL, Body: "AC-01", BodySHA256: strings.Repeat("a", 64)}}
	build := ownerContextForTest("", nil).Build
	original := executionOwnerPromptTemplate
	t.Cleanup(func() { executionOwnerPromptTemplate = original })
	executionOwnerPromptTemplate = "{UNCONFIGURED_TEMPLATE_TOKEN}"
	artifacts, err := build(record, req, snapshot, nil)
	if err != nil {
		t.Fatalf("prepared builder followed another context's template: %v", err)
	}
	data, err := os.ReadFile(artifacts.PromptPath)
	if err != nil || len(data) == 0 || strings.Contains(string(data), "UNCONFIGURED_TEMPLATE_TOKEN") {
		t.Fatalf("invalid prepared prompt: bytes=%d err=%v", len(data), err)
	}
}
