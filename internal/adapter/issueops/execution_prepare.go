package issueops

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
)

func newExecutionOperationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

var issueOpsOwnerReportLabels = []string{
	"Status",
	"Lifecycle",
	"Mode/host/model",
	"Worktree/branch/final HEAD",
	"Lease generation/completion",
	"Issue/packet digests",
	"Commits",
	"Changed files",
	"Acceptance evidence",
	"Verification",
	"AI-slop clean",
	"Draft PR/MR",
	"Deviations",
	"Blockers",
}

func renderExecutionOwnerReportContract(record issueops.IssueOpsRecord, req issueops.ExecutionPrepareRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if record.Execution != nil {
		mode = string(record.Execution.Mode)
	}
	values := []string{
		"completed | blocked",
		record.ID,
		mode + " / " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) + " / " + strings.TrimSpace(req.OwnerModel) + " (" + strings.TrimSpace(req.OwnerEffort) + ")",
		"<exact values>",
		"<generation + receipt or blocker>",
		"<verified | drift, 원문 secret 없음>",
		"<ordered SHA + subject>",
		"<exact paths>",
		"<AC-ID → test/command/result mapping>",
		"<every command + PASS/FAIL>",
		"<removed duplication/legacy/noise or none>",
		"<URL or none>",
		"<issue-vs-code mismatch with file:line evidence or none>",
		"<exact state/error/next command or none>",
	}
	lines := []string{"## IssueOps v1 Owner Report"}
	for index, label := range issueOpsOwnerReportLabels {
		lines = append(lines, "- "+label+": "+values[index])
	}
	return strings.Join(lines, "\n")
}

func executionNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}
