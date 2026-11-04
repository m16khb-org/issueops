// Package gates 패키지는 gates 원장(ledger)의 init·check·lint·abandon 유스케이스를 조율하는 application 계층이다.
// 원장 파싱과 판정 규칙은 domain/gates에, 파일 접근과 CHECK 명령 실행은 주입된 포트에 맡긴다.
package gates

import (
	"fmt"
	"time"

	gatescontract "issueops/internal/contract/gates"
	gatesdomain "issueops/internal/domain/gates"
)

// LedgerStore 인터페이스는 Service가 원장 파일을 찾고 읽고 쓰는 데 쓰는 저장소 포트다.
// WritePreservingMode는 기존 파일 권한을 바꾸지 않고 내용만 교체한다.
type LedgerStore interface {
	ExistsFile(file string) bool
	Create(file string, data []byte) error
	Discover(cwd string) ([]string, error)
	Read(file string) ([]byte, error)
	WritePreservingMode(file string, data []byte) error
}

type runner interface {
	Run(root, cwd string, request gatescontract.CheckRequest, gate gatesdomain.Gate) gatesdomain.CheckOutcome
}

type clock interface{ Now() time.Time }

// Service 타입은 gates 원장 유스케이스(Init·Check·Lint·Abandon)를 제공한다.
// Store는 원장 파일 접근, Runner는 CHECK 실행, Clock은 보고서 시각을 맡는다.
type Service struct {
	Store  LedgerStore
	Runner runner
	Clock  clock
	// Evaluate는 Lint가 CHECK를 실행하지 않고 command policy 판정만 볼 때 쓴다.
	// nil이면 Lint는 정책 판정을 건너뛴다.
	Evaluate policyEvaluate
}

// Check 메서드는 요청한 원장(없으면 cwd에서 탐색한 원장)의 각 게이트를 평가하고 결과를 모은다.
// StatusOnly가 아니면 CHECK가 있고 실행 대상인 게이트만 실행하고, 상태가 바뀐 원장은 다시 쓴다.
// 원장 파일을 하나도 찾지 못하면 ErrNoGateFiles를 감싼 에러를 돌려준다.
func (service Service) Check(request gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
	request = gatesdomain.NormalizeCheck(request)
	root, cwd := request.WorkspaceRoot, request.CWD
	files := request.Files
	if len(files) == 0 {
		var err error
		files, err = service.Store.Discover(cwd)
		if err != nil {
			return gatescontract.CheckResult{Files: []gatescontract.FileResult{}}, err
		}
		if len(files) == 0 {
			// QA (MCP): "no gate files found" read the same for a missing
			// ledger and a wrong cwd; name where discovery looked.
			return gatescontract.CheckResult{Files: []gatescontract.FileResult{}}, fmt.Errorf("%w under %s", gatescontract.ErrNoGateFiles, cwd)
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
	fileResult := gatescontract.FileResult{File: file, Gates: []gatescontract.GateResult{}}
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
	warnings = append(warnings, gatesdomain.LedgerFormatWarnings(file, ledger)...)
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
