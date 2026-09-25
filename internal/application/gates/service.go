package gates

import (
	"fmt"
	"strings"
	"time"

	gatescontract "issueops/internal/contract/gates"
	gatesdomain "issueops/internal/domain/gates"
)

type LedgerStore interface {
	Discover(cwd string) ([]string, error)
	Read(file string) ([]byte, error)
	WritePreservingMode(file string, data []byte) error
}

type Outcome struct {
	Passed       bool
	Evidence     string
	CheckError   string
	PolicyDenied bool
	AuditLogID   string
}

type Runner interface {
	Run(root, cwd string, request gatescontract.CheckRequest, gate gatesdomain.Gate) Outcome
}

type Clock interface{ Now() time.Time }

type Service struct {
	Store  LedgerStore
	Runner Runner
	Clock  Clock
}

func (service Service) Check(request gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
	root := strings.TrimSpace(request.WorkspaceRoot)
	cwd := strings.TrimSpace(request.CWD)
	if root == "" {
		root = cwd
	}
	if cwd == "" {
		cwd = root
	}
	if request.TimeoutSeconds <= 0 {
		request.TimeoutSeconds = gatescontract.TimeoutDefaultSeconds
	}
	files := request.Files
	if len(files) == 0 {
		var err error
		files, err = service.Store.Discover(cwd)
		if err != nil {
			return gatescontract.CheckResult{}, err
		}
		if len(files) == 0 {
			return gatescontract.CheckResult{}, gatescontract.ErrNoGateFiles
		}
	}
	result := gatescontract.CheckResult{
		OK: true, SchemaVersion: gatescontract.SchemaVersion, StatusOnly: request.StatusOnly,
		GeneratedAt: service.Clock.Now().UTC().Format(time.RFC3339),
	}
	for _, file := range files {
		fileResult, warnings := service.checkFile(root, cwd, request, file)
		result.Files = append(result.Files, fileResult)
		result.Warnings = append(result.Warnings, warnings...)
		result.TotalGates += fileResult.GateCount
		result.TotalMet += fileResult.Met
		result.TotalUnmet += fileResult.Unmet
		result.TotalAbandoned += fileResult.Abandoned
		if fileResult.Error != "" {
			result.OK = false
		}
	}
	result.Complete = result.TotalUnmet == 0 && result.OK
	return result, nil
}

func (service Service) checkFile(root, cwd string, request gatescontract.CheckRequest, file string) (gatescontract.FileResult, []string) {
	fileResult := gatescontract.FileResult{File: file}
	warnings := []string{}
	data, err := service.Store.Read(file)
	if err != nil {
		fileResult.Error = err.Error()
		return fileResult, warnings
	}
	ledger := gatesdomain.Parse(string(data))
	if len(ledger.Gates) == 0 {
		fileResult.Error = "no gates found"
		return fileResult, warnings
	}
	changed := false
	for i := range ledger.Gates {
		gate := &ledger.Gates[i]
		gateResult := gatescontract.GateResult{
			ID: gate.ID, Title: gate.Title, Checked: gate.Checked,
			HasCheck: strings.TrimSpace(gate.CheckCmd) != "", Evidence: gate.Evidence, AbandonReason: gate.AbandonReason,
		}
		if !gate.Abandoned && !request.StatusOnly && gateResult.HasCheck && gatesdomain.ShouldRun(*gate) {
			outcome := service.Runner.Run(root, cwd, request, *gate)
			gateResult.PolicyDenied = outcome.PolicyDenied
			gateResult.AuditLogID = outcome.AuditLogID
			if outcome.Passed {
				gatesdomain.MarkPass(&ledger, i, outcome.Evidence)
				gate.Evidence = outcome.Evidence
				changed = true
			} else {
				gateResult.CheckError = outcome.CheckError
				if outcome.PolicyDenied {
					warnings = append(warnings, fmt.Sprintf("%s %s: check command denied by policy", file, gate.ID))
				}
			}
		}
		gateResult.State = gatesdomain.State(*gate)
		gateResult.Checked = gate.Checked
		gateResult.Evidence = gate.Evidence
		fileResult.Gates = append(fileResult.Gates, gateResult)
	}
	summary := gatesdomain.Summarize(ledger.Gates)
	fileResult.GateCount = summary.Total
	fileResult.Met = summary.Met
	fileResult.Unmet = summary.Unmet
	fileResult.Abandoned = summary.Abandoned
	fileResult.Complete = summary.Complete
	if changed {
		if writeErr := service.Store.WritePreservingMode(file, []byte(gatesdomain.Render(ledger))); writeErr != nil {
			fileResult.Error = writeErr.Error()
			warnings = append(warnings, fmt.Sprintf("%s: failed to write updated ledger: %v", file, writeErr))
		}
	}
	return fileResult, warnings
}
