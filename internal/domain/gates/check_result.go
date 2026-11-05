package gates

import (
	"fmt"
	"path/filepath"
	"strings"

	model "issueops/internal/contract/gates"
)

// CheckOutcome 타입은 CHECK 명령 하나를 처리한 결과다.
// Passed이면 Evidence가 원장에 남길 증거이고, 아니면 CheckError가 실패 사유이다.
// PolicyDenied는 command policy가 실행을 거부했다는 뜻이며 AuditLogID는 그 감사 기록을 가리킨다.
type CheckOutcome struct {
	Passed               bool
	Evidence, CheckError string
	PolicyDenied         bool
	AuditLogID           string
}

// NormalizeCheck 함수는 검사 요청의 빈 값을 채운 복사본을 돌려준다.
// WorkspaceRoot와 CWD는 서로를 기본값으로 삼고, 0 이하의 제한 시간은 기본값으로 바꾸며,
// 상대 경로 Files는 CWD 아래 경로로 바꾼다.
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
	// --cwd is the gate file directory: a relative --file names a ledger
	// under it, the same place discovery looks, not under the process cwd.
	if req.CWD != "" && len(req.Files) > 0 {
		files := make([]string, len(req.Files))
		for i, file := range req.Files {
			if file != "" && !filepath.IsAbs(file) {
				file = filepath.Join(req.CWD, file)
			}
			files[i] = file
		}
		req.Files = files
	}
	return req
}

// ShouldCheck 함수는 게이트에 CHECK를 실행해야 하는지 판정한다.
// 포기된 게이트, 상태 조회 전용 요청, CHECK가 없는 게이트는 건너뛰며 나머지는 ShouldRun을 따른다.
func ShouldCheck(gate Gate, statusOnly bool) bool {
	return !gate.Abandoned && !statusOnly && strings.TrimSpace(gate.CheckCmd) != "" && ShouldRun(gate)
}

// ApplyCheck 함수는 CHECK 결과를 ledger의 index번째 게이트에 반영하고 응답용 게이트 결과를 만든다.
// outcome이 nil이면 아무것도 바꾸지 않는다. 통과하면 게이트를 체크 처리하고 증거를 기록하며 마지막 반환값이 true가 된다.
// 정책 거부는 file과 게이트 ID를 담은 경고로 돌려준다.
func ApplyCheck(ledger *Ledger, index int, outcome *CheckOutcome, file string) (model.GateResult, []string, bool) {
	gate := &ledger.Gates[index]
	result := model.GateResult{ID: gate.ID, Title: gate.Title, Checked: gate.Checked, HasCheck: strings.TrimSpace(gate.CheckCmd) != "", Evidence: gate.Evidence, AbandonReason: gate.AbandonReason}
	warnings := []string{}
	changed := false
	if outcome != nil {
		result.PolicyDenied = outcome.PolicyDenied
		result.AuditLogID = outcome.AuditLogID
		if outcome.Passed {
			markPass(ledger, index, outcome.Evidence)
			gate.Evidence = outcome.Evidence
			changed = true
		} else {
			result.CheckError = outcome.CheckError
			if outcome.PolicyDenied {
				warnings = append(warnings, fmt.Sprintf("%s %s: check command denied by policy", file, gate.ID))
			}
		}
	}
	result.State = state(*gate)
	result.Checked = gate.Checked
	result.Evidence = gate.Evidence
	return result, warnings, changed
}

// SummarizeFile 함수는 ledger의 게이트 집계(전체, 충족, 미충족, 포기, 완료 여부)를 result에 채워 돌려준다.
func SummarizeFile(result model.FileResult, ledger Ledger) model.FileResult {
	summary := summarize(ledger.Gates)
	result.GateCount = summary.Total
	result.Met = summary.Met
	result.Unmet = summary.Unmet
	result.Abandoned = summary.Abandoned
	result.Complete = summary.Complete
	return result
}

// AddFileResult 함수는 파일 하나의 결과와 경고를 전체 검사 결과에 합산한다.
// 파일에 오류가 있으면 OK가 false가 되고, 미충족 게이트가 없고 OK일 때만 Complete가 true이다.
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

// DeniedCheck 함수는 command policy 거부 사유로 실패한 CHECK 결과를 만든다.
func DeniedCheck(auditLogID string, denyReasons []string) CheckOutcome {
	return CheckOutcome{PolicyDenied: true, AuditLogID: auditLogID, CheckError: deniedMessage(denyReasons)}
}

// CompletedCheck 함수는 실행을 마친 CHECK의 출력, 종료 코드, 시간 초과 여부를 게이트의 EXPECT와 견줘 결과를 만든다.
// 통과한 경우에만 Evidence를 채운다.
func CompletedCheck(gate Gate, stdout, stderr string, exitCode int, timedOut bool, auditLogID string) CheckOutcome {
	decision := decideCheck(gate.Expect, stdout, stderr, exitCode, timedOut)
	outcome := CheckOutcome{AuditLogID: auditLogID, Passed: decision.Passed, CheckError: decision.Error}
	if decision.Passed {
		outcome.Evidence = decision.Evidence
	}
	return outcome
}

// ValidateLedger 함수는 ledger에 게이트가 하나 이상 있는지 확인하고 없으면 오류를 돌려준다.
func ValidateLedger(ledger Ledger) error {
	if len(ledger.Gates) == 0 {
		return fmt.Errorf("no gates found")
	}
	return nil
}
