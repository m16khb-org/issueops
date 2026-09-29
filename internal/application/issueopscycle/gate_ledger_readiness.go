package issueopscycle

import (
	"fmt"
	"strings"

	gatescontract "issueops/internal/contract/gates"
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func ApplyGateLedgers(ready model.IssueOpsReadiness, root, issueNumber string, ports cycleport.GateLedgerReadiness) model.IssueOpsReadiness {
	root = strings.TrimSpace(root)
	if root == "" {
		return ready
	}
	files, err := ports.Discover(root)
	if err != nil || len(files) == 0 {
		return ready
	}
	files, skipped := cycledomain.ScopeGateLedgers(root, files, issueNumber)
	missing := []string{}
	warnings := []string{}
	if len(skipped) > 0 {
		relative := make([]string, 0, len(skipped))
		for _, file := range skipped {
			relative = append(relative, cycledomain.GateLedgerRelativePath(root, file))
		}
		warnings = append(warnings, fmt.Sprintf("gates_skipped:%d (%s)", len(skipped), strings.Join(relative, ", ")))
	}
	for _, file := range files {
		relative := cycledomain.GateLedgerRelativePath(root, file)
		result, err := ports.Check(gatescontract.CheckRequest{
			WorkspaceRoot: root, CWD: root, Files: []string{file}, StatusOnly: true,
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("gates %s: %v", relative, err))
			continue
		}
		statuses := []cycledomain.GateLedgerStatus{}
		for _, fileResult := range result.Files {
			for _, gate := range fileResult.Gates {
				statuses = append(statuses, cycledomain.GateLedgerStatus{ID: gate.ID, State: gate.State, Title: gate.Title})
			}
		}
		fileMissing, fileWarnings := cycledomain.GateLedgerFileReadiness(relative, result.Complete, statuses)
		missing = append(missing, fileMissing...)
		warnings = append(warnings, fileWarnings...)
	}
	return cycledomain.MergeGateReadiness(ready, missing, warnings)
}
