package gates

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/gates"
)

type CheckOutcome struct {
	Passed               bool
	Evidence, CheckError string
	PolicyDenied         bool
	AuditLogID           string
}

func NormalizeCheck(req model.CheckRequest) model.CheckRequest {
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	req.CWD = strings.TrimSpace(req.CWD)
	if req.WorkspaceRoot == "" {
		req.WorkspaceRoot = req.CWD
	}
	if req.CWD == "" {
		req.CWD = req.WorkspaceRoot
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = model.TimeoutDefaultSeconds
	}
	return req
}
func ShouldCheck(gate Gate, statusOnly bool) bool {
	return !gate.Abandoned && !statusOnly && strings.TrimSpace(gate.CheckCmd) != "" && ShouldRun(gate)
}
func ApplyCheck(ledger *Ledger, index int, outcome *CheckOutcome, file string) (model.GateResult, []string, bool) {
	gate := &ledger.Gates[index]
	result := model.GateResult{ID: gate.ID, Title: gate.Title, Checked: gate.Checked, HasCheck: strings.TrimSpace(gate.CheckCmd) != "", Evidence: gate.Evidence, AbandonReason: gate.AbandonReason}
	warnings := []string{}
	changed := false
	if outcome != nil {
		result.PolicyDenied = outcome.PolicyDenied
		result.AuditLogID = outcome.AuditLogID
		if outcome.Passed {
			MarkPass(ledger, index, outcome.Evidence)
			gate.Evidence = outcome.Evidence
			changed = true
		} else {
			result.CheckError = outcome.CheckError
			if outcome.PolicyDenied {
				warnings = append(warnings, fmt.Sprintf("%s %s: check command denied by policy", file, gate.ID))
			}
		}
	}
	result.State = State(*gate)
	result.Checked = gate.Checked
	result.Evidence = gate.Evidence
	return result, warnings, changed
}
func SummarizeFile(result model.FileResult, ledger Ledger) model.FileResult {
	summary := Summarize(ledger.Gates)
	result.GateCount = summary.Total
	result.Met = summary.Met
	result.Unmet = summary.Unmet
	result.Abandoned = summary.Abandoned
	result.Complete = summary.Complete
	return result
}
func AddFileResult(result model.CheckResult, file model.FileResult, warnings []string) model.CheckResult {
	result.Files = append(result.Files, file)
	result.Warnings = append(result.Warnings, warnings...)
	result.TotalGates += file.GateCount
	result.TotalMet += file.Met
	result.TotalUnmet += file.Unmet
	result.TotalAbandoned += file.Abandoned
	if file.Error != "" {
		result.OK = false
	}
	result.Complete = result.TotalUnmet == 0 && result.OK
	return result
}
func DeniedCheck(auditLogID string, denyReasons []string) CheckOutcome {
	return CheckOutcome{PolicyDenied: true, AuditLogID: auditLogID, CheckError: "check denied by policy: " + strings.Join(denyReasons, "; ")}
}
func CompletedCheck(gate Gate, stdout, stderr string, exitCode int, timedOut bool, auditLogID string) CheckOutcome {
	decision := DecideCheck(gate.Expect, stdout, stderr, exitCode, timedOut)
	outcome := CheckOutcome{AuditLogID: auditLogID, Passed: decision.Passed, CheckError: decision.Error}
	if decision.Passed {
		outcome.Evidence = decision.Evidence
	}
	return outcome
}

func ValidateLedger(ledger Ledger) error {
	if len(ledger.Gates) == 0 {
		return fmt.Errorf("no gates found")
	}
	return nil
}
