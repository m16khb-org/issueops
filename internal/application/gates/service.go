package gates

import (
	"fmt"
	"time"

	gatescontract "issueops/internal/contract/gates"
	gatesdomain "issueops/internal/domain/gates"
)

type LedgerStore interface {
	ExistsFile(file string) bool
	Create(file string, data []byte) error
	Discover(cwd string) ([]string, error)
	Read(file string) ([]byte, error)
	WritePreservingMode(file string, data []byte) error
}

type Runner interface {
	Run(root, cwd string, request gatescontract.CheckRequest, gate gatesdomain.Gate) gatesdomain.CheckOutcome
}

type Clock interface{ Now() time.Time }

type Service struct {
	Store  LedgerStore
	Runner Runner
	Clock  Clock
}

func (service Service) Check(request gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
	request = gatesdomain.NormalizeCheck(request)
	root, cwd := request.WorkspaceRoot, request.CWD
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
		result = gatesdomain.AddFileResult(result, fileResult, warnings)
	}
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
	if err := gatesdomain.ValidateLedger(ledger); err != nil {
		fileResult.Error = err.Error()
		return fileResult, warnings
	}
	changed := false
	for i := range ledger.Gates {
		var outcome *gatesdomain.CheckOutcome
		if gatesdomain.ShouldCheck(ledger.Gates[i], request.StatusOnly) {
			run := service.Runner.Run(root, cwd, request, ledger.Gates[i])
			outcome = &run
		}
		result, gateWarnings, updated := gatesdomain.ApplyCheck(&ledger, i, outcome, file)
		fileResult.Gates = append(fileResult.Gates, result)
		warnings = append(warnings, gateWarnings...)
		changed = changed || updated
	}
	fileResult = gatesdomain.SummarizeFile(fileResult, ledger)
	if changed {
		if writeErr := service.Store.WritePreservingMode(file, []byte(gatesdomain.Render(ledger))); writeErr != nil {
			fileResult.Error = writeErr.Error()
			warnings = append(warnings, fmt.Sprintf("%s: failed to write updated ledger: %v", file, writeErr))
		}
	}
	return fileResult, warnings
}
