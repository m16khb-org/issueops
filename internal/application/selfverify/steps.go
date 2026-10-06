package selfverify

import (
	"time"

	contract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

type SelfVerifyPlannedStep struct {
	Label string
	Run   func() contract.StepResult
}

type RiskQAEvidence struct {
	Step             contract.StepResult
	CoversFullGoTest bool
}

// Check before importing repository helpers, then exec the suite runner with
// the same interpreter. The existing runner owns the timeout and output budget.
const pythonRuntimeDiscovery = `import os, sys
print("Python runtime: " + sys.version.split()[0] + " (requires Python 3.10+)", flush=True)
if sys.version_info < (3, 10):
    sys.exit("Python script tests require Python 3.10+; found " + sys.version.split()[0])
os.execv(sys.executable, [sys.executable] + sys.argv[1:])
`

const selfVerifyGoTestTimeout = 10 * time.Minute

type SelfVerifyStepDeps struct {
	IssueOpsRoot                  func() string
	RunCommandStep                func(string, string, time.Duration, string, string, ...string) contract.StepResult
	ValidateHarnessInvariants     func(string) contract.StepResult
	ValidateGoFormat              func(string) contract.StepResult
	ValidateRiskQATier            func(string) RiskQAEvidence
	ValidateRiskQATierWithScope   func(string, string) RiskQAEvidence
	ValidateInspect               func(string, string) contract.StepResult
	ValidateDocsIndex             func(string, string) contract.StepResult
	ValidateSelfVerifyCandidate   func(string, string, int64) contract.StepResult
	ValidateStepBudgetBaseline    func(string, string, int64) contract.StepResult
	ValidateInstallDryRunSmoke    func(string, string, int64) contract.StepResult
	ValidateCommandPolicy         func(string, string) contract.StepResult
	ValidateCommandAudit          func(string, string, int64) contract.StepResult
	ValidateContractCheck         func(string, string) contract.StepResult
	ValidateToolConformance       func(string, string) contract.StepResult
	ValidateWorkerLifecycle       func(string, string, int64) contract.StepResult
	ValidateMCP                   func(string, string) contract.StepResult
	ValidateStateRoundtrip        func(string, string, int64) contract.StepResult
	ValidateParallelTempIsolation func(string, string, int64) contract.StepResult
	ValidatePreflightFuzz         func(string, string, int64) contract.StepResult
	ValidateWebFetchBattery       func(string, string, int64) contract.StepResult
	ValidateNativeIntegration     func(string) contract.StepResult
	ValidateRedactionAudit        func(string) contract.StepResult
	ValidateQAGate                func(string) contract.StepResult
}

func PlannedSteps(root string, tempBin string, seed int64, goTestStep *contract.StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	var riskQAEvidence RiskQAEvidence
	planned := []SelfVerifyPlannedStep{
		{Label: "harness invariants", Run: func() contract.StepResult { return deps.ValidateHarnessInvariants(root) }},
		// CI의 Format check와 같은 게이트를 로컬에서도 무조건 실행한다. gofmt는
		// 값싸고 결정적이므로 긴 go test보다 앞에 두어 fail-fast 모드에서 먼저 드러낸다.
		{Label: "gofmt", Run: func() contract.StepResult { return deps.ValidateGoFormat(root) }},
		{Label: "Python script tests", Run: func() contract.StepResult {
			return deps.RunCommandStep(root, "Python script tests", 5*time.Minute, "", "python3", "-c", pythonRuntimeDiscovery, "scripts/python_suite_runner.py")
		}},
		{Label: "Go test match guard", Run: func() contract.StepResult {
			return deps.RunCommandStep(root, "Go test match guard", 30*time.Second, "", "bash", "scripts/verify-go-test-match-test.sh")
		}},
		{Label: "risk QA tier", Run: func() contract.StepResult {
			riskQAEvidence = deps.ValidateRiskQATier(root)
			return riskQAEvidence.Step
		}},
		{Label: "go test", Run: func() contract.StepResult {
			if selfverifydomain.ReuseRiskRaceAsFullTest(riskQAEvidence.Step.OK, riskQAEvidence.CoversFullGoTest) {
				*goTestStep = contract.StepResult{
					Label:   "go test",
					Command: riskQAEvidence.Step.Command,
					OK:      true,
					Reused:  true,
					Stdout:  "reused successful full-suite coverage from risk QA race test",
				}
				return *goTestStep
			}
			*goTestStep = deps.RunCommandStep(root, "go test", selfVerifyGoTestTimeout, "", "go", "test", "./...", "-count=1")
			return *goTestStep
		}},
		{Label: "contract golden tests", Run: func() contract.StepResult {
			return CachedContractGoldenStep(*goTestStep, deps)
		}},
		{Label: "go build", Run: func() contract.StepResult {
			return deps.RunCommandStep(root, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/issueops")
		}},
		{Label: "binary drift", Run: func() contract.StepResult {
			// tempBin runs doctor; the observation concerns root/bin/issueops.
			step := deps.RunCommandStep(root, "binary drift", 10*time.Second, "", tempBin, "doctor", "--static-only", "--json", "--repo", root)
			return binaryDriftEvidence(step)
		}},
		{Label: "inspect smoke", Run: func() contract.StepResult { return deps.ValidateInspect(tempBin, root) }},
		{Label: "docs index smoke", Run: func() contract.StepResult { return deps.ValidateDocsIndex(tempBin, root) }},
		{Label: "candidate export", Run: func() contract.StepResult { return deps.ValidateSelfVerifyCandidate(tempBin, root, seed) }},
		{Label: "step budget baseline", Run: func() contract.StepResult { return deps.ValidateStepBudgetBaseline(tempBin, root, seed) }},
		{Label: "install dry-run smoke", Run: func() contract.StepResult { return deps.ValidateInstallDryRunSmoke(tempBin, root, seed) }},
		{Label: "command policy smoke", Run: func() contract.StepResult { return deps.ValidateCommandPolicy(tempBin, root) }},
		{Label: "command audit smoke", Run: func() contract.StepResult { return deps.ValidateCommandAudit(tempBin, root, seed) }},
		{Label: "contract check", Run: func() contract.StepResult { return deps.ValidateContractCheck(tempBin, root) }},
		{Label: "tool contract conformance", Run: func() contract.StepResult { return deps.ValidateToolConformance(tempBin, root) }},
		{Label: "worker lifecycle smoke", Run: func() contract.StepResult { return deps.ValidateWorkerLifecycle(tempBin, root, seed) }},
		{Label: "MCP smoke", Run: func() contract.StepResult { return deps.ValidateMCP(tempBin, root) }},
		{Label: "state roundtrip", Run: func() contract.StepResult { return deps.ValidateStateRoundtrip(tempBin, root, seed) }},
		{Label: "parallel isolation", Run: func() contract.StepResult { return deps.ValidateParallelTempIsolation(tempBin, root, seed) }},
		{Label: "preflight fuzz", Run: func() contract.StepResult { return deps.ValidatePreflightFuzz(tempBin, root, seed) }},
		{Label: "web fetch battery", Run: func() contract.StepResult { return deps.ValidateWebFetchBattery(tempBin, root, seed) }},
		{Label: "native integration", Run: func() contract.StepResult { return deps.ValidateNativeIntegration(root) }},
		{Label: "redaction audit", Run: func() contract.StepResult { return deps.ValidateRedactionAudit(root) }},
		{Label: "QA gate", Run: func() contract.StepResult { return deps.ValidateQAGate(root) }},
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

func CachedContractGoldenStep(goTestStep contract.StepResult, deps SelfVerifyStepDeps) contract.StepResult {
	if selfverifydomain.ReuseFullTestAsGolden(goTestStep.OK) {
		return contract.StepResult{
			Label:      "contract golden tests",
			Command:    "covered by go test ./... -count=1",
			OK:         true,
			Reused:     true,
			DurationMS: 0,
			Stdout:     "contract golden tests already executed by full go test suite",
		}
	}
	step := deps.RunCommandStep(deps.IssueOpsRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "-json", "./cmd/issueops/contractgolden", "./cmd/issueops/issueopsapp", "-run", "^(TestCLIUsageGolden|TestMCPToolsGolden|TestMCPResourcesGolden|TestResponseContractsGolden)$", "-count=1")
	if !step.OK {
		return step
	}
	if err := requireGoldenEvidence(step); err != nil {
		step.OK = false
		step.Error = "contract golden evidence: " + err.Error()
	}
	return step
}
