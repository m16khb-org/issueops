package selfverify

import (
	"time"

	contract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

type StepResult = contract.StepResult

type SelfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

type RiskQAEvidence struct {
	Step             StepResult
	CoversFullGoTest bool
}

// Check before importing repository helpers, then exec the CI discovery argv with
// the same interpreter. The existing runner owns the timeout and output budget.
const pythonRuntimeDiscovery = `import os, sys
print("Python runtime: " + sys.version.split()[0] + " (requires Python 3.10+)", flush=True)
if sys.version_info < (3, 10):
    sys.exit("Python script tests require Python 3.10+; found " + sys.version.split()[0])
os.execv(sys.executable, [sys.executable] + sys.argv[1:])
`

const selfVerifyGoTestTimeout = 10 * time.Minute

type SelfVerifyStepDeps struct {
	IssueOpsRoot                    func() string
	RunCommandStep                  func(string, string, time.Duration, string, string, ...string) StepResult
	ValidateHarnessInvariants       func(string) StepResult
	ValidateGoFormat                func(string) StepResult
	ValidateRiskQATier              func(string) RiskQAEvidence
	ValidateInspect                 func(string, string) StepResult
	ValidateDocsIndex               func(string, string) StepResult
	ValidateSelfVerifyCandidate     func(string, string, int64) StepResult
	ValidateStepBudgetBaseline      func(string, string, int64) StepResult
	ValidateInstallDryRunSmoke      func(string, string, int64) StepResult
	ValidateCommandPolicy           func(string, string) StepResult
	ValidateCommandAudit            func(string, string, int64) StepResult
	ValidateContractCheck           func(string, string) StepResult
	ValidateToolConformance         func(string, string) StepResult
	ValidateWorkerLifecycle         func(string, string, int64) StepResult
	ValidateMCP                     func(string, string) StepResult
	ValidateStateRoundtrip          func(string, string, int64) StepResult
	ValidateParallelTempIsolation   func(string, string, int64) StepResult
	ValidateDaemonRestartResilience func(string, string, int64) StepResult
	ValidatePreflightFuzz           func(string, string, int64) StepResult
	ValidateWebFetchBattery         func(string, string, int64) StepResult
	ValidateNativeIntegration       func(string) StepResult
	ValidateRedactionAudit          func(string) StepResult
	ValidateQAGate                  func(string) StepResult
}

func PlannedSteps(root string, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	var riskQAEvidence RiskQAEvidence
	planned := []SelfVerifyPlannedStep{
		{Label: "harness invariants", Run: func() StepResult { return deps.ValidateHarnessInvariants(root) }},
		// CI의 Format check와 같은 게이트를 로컬에서도 무조건 실행한다. gofmt는
		// 값싸고 결정적이므로 긴 go test보다 앞에 두어 fail-fast 모드에서 먼저 드러낸다.
		{Label: "gofmt", Run: func() StepResult { return deps.ValidateGoFormat(root) }},
		{Label: "Python script tests", Run: func() StepResult {
			return deps.RunCommandStep(root, "Python script tests", 5*time.Minute, "", "python3", "-c", pythonRuntimeDiscovery, "-m", "unittest", "discover", "-s", "scripts", "-p", "*_test.py")
		}},
		{Label: "risk QA tier", Run: func() StepResult {
			riskQAEvidence = deps.ValidateRiskQATier(root)
			return riskQAEvidence.Step
		}},
		{Label: "go test", Run: func() StepResult {
			if selfverifydomain.ReuseRiskRaceAsFullTest(riskQAEvidence.Step.OK, riskQAEvidence.CoversFullGoTest) {
				*goTestStep = StepResult{
					Label:   "go test",
					Command: riskQAEvidence.Step.Command,
					OK:      true,
					Stdout:  "reused successful full-suite coverage from risk QA race test",
				}
				return *goTestStep
			}
			*goTestStep = deps.RunCommandStep(root, "go test", selfVerifyGoTestTimeout, "", "go", "test", "./...", "-count=1")
			return *goTestStep
		}},
		{Label: "contract golden tests", Run: func() StepResult {
			return CachedContractGoldenStep(*goTestStep, deps)
		}},
		{Label: "go build", Run: func() StepResult {
			return deps.RunCommandStep(root, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/issueops")
		}},
		{Label: "binary drift", Run: func() StepResult {
			// 빌드가 성공한 뒤, 커밋된 bin/issueops가 소스 트리 대비 stale하지
			// 않은지 확인한다. 방금 빌드한 tempBin으로 doctor --json을 실행해
			// binary_drift 체크를 살핀다. 갓 빌드한 tempBin은 구조상 항상 최신이므로
			// 이 단계가 false positive를 낼 수는 없지만, self-verify QA 표면의 일부로
			// drift 탐지 경로를 여전히 검증한다.
			return deps.RunCommandStep(root, "binary drift", 10*time.Second, "", tempBin, "doctor", "--static-only", "--json", "--repo", root)
		}},
		{Label: "inspect smoke", Run: func() StepResult { return deps.ValidateInspect(tempBin, root) }},
		{Label: "docs index smoke", Run: func() StepResult { return deps.ValidateDocsIndex(tempBin, root) }},
		{Label: "candidate export", Run: func() StepResult { return deps.ValidateSelfVerifyCandidate(tempBin, root, seed) }},
		{Label: "step budget baseline", Run: func() StepResult { return deps.ValidateStepBudgetBaseline(tempBin, root, seed) }},
		{Label: "install dry-run smoke", Run: func() StepResult { return deps.ValidateInstallDryRunSmoke(tempBin, root, seed) }},
		{Label: "command policy smoke", Run: func() StepResult { return deps.ValidateCommandPolicy(tempBin, root) }},
		{Label: "command audit smoke", Run: func() StepResult { return deps.ValidateCommandAudit(tempBin, root, seed) }},
		{Label: "contract check", Run: func() StepResult { return deps.ValidateContractCheck(tempBin, root) }},
		{Label: "tool contract conformance", Run: func() StepResult { return deps.ValidateToolConformance(tempBin, root) }},
		{Label: "worker lifecycle smoke", Run: func() StepResult { return deps.ValidateWorkerLifecycle(tempBin, root, seed) }},
		{Label: "MCP smoke", Run: func() StepResult { return deps.ValidateMCP(tempBin, root) }},
		{Label: "state roundtrip", Run: func() StepResult { return deps.ValidateStateRoundtrip(tempBin, root, seed) }},
		{Label: "parallel isolation", Run: func() StepResult { return deps.ValidateParallelTempIsolation(tempBin, root, seed) }},
		{Label: "daemon resilience", Run: func() StepResult { return deps.ValidateDaemonRestartResilience(tempBin, root, seed) }},
		{Label: "preflight fuzz", Run: func() StepResult { return deps.ValidatePreflightFuzz(tempBin, root, seed) }},
		{Label: "web fetch battery", Run: func() StepResult { return deps.ValidateWebFetchBattery(tempBin, root, seed) }},
		{Label: "native integration", Run: func() StepResult { return deps.ValidateNativeIntegration(root) }},
		{Label: "redaction audit", Run: func() StepResult { return deps.ValidateRedactionAudit(root) }},
		{Label: "QA gate", Run: func() StepResult { return deps.ValidateQAGate(root) }},
	}
	byLabel := make(map[string]SelfVerifyPlannedStep, len(planned))
	for _, step := range planned {
		byLabel[step.Label] = step
	}
	ordered := make([]SelfVerifyPlannedStep, 0, len(planned))
	for _, label := range selfverifydomain.StepOrder() {
		step, ok := byLabel[label]
		if !ok {
			panic("missing self-verification step implementation: " + label)
		}
		ordered = append(ordered, step)
	}
	if len(ordered) != len(planned) {
		panic("self-verification step implementation is missing from domain plan")
	}
	return ordered
}

func CachedContractGoldenStep(goTestStep StepResult, deps SelfVerifyStepDeps) StepResult {
	if selfverifydomain.ReuseFullTestAsGolden(goTestStep.OK) {
		return StepResult{
			Label:      "contract golden tests",
			Command:    "covered by go test ./... -count=1",
			OK:         true,
			DurationMS: 0,
			Stdout:     "contract golden tests already executed by full go test suite",
		}
	}
	return deps.RunCommandStep(deps.IssueOpsRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/issueops", "-run", "Golden", "-count=1")
}
