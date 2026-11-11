package selfverify

import (
	"strings"
	"time"

	contract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

// PlannedStep is one self-verification step: its label and the function that runs it.
type PlannedStep struct {
	Label string
	Run   func() contract.StepResult
}

// RiskQAEvidence is the outcome of the risk-based QA tier. CoversFullGoTest reports that it already
// ran the whole Go suite, letting the plain go test step be reused.
type RiskQAEvidence struct {
	Step             contract.StepResult
	CoversFullGoTest bool
}

// Check before importing repository helpers, then exec the suite runner with
// the same interpreter. The existing runner owns the timeout and output budget.
// Windows has no exec: os.execv starts the child and exits the parent with
// status 0 at once, which would hide a failing suite, so Windows waits for the
// child and returns its exit status instead.
const pythonRuntimeDiscovery = `import os, sys
print("Python runtime: " + sys.version.split()[0] + " (requires Python 3.10+)", flush=True)
if sys.version_info < (3, 10):
    sys.exit("Python script tests require Python 3.10+; found " + sys.version.split()[0])
if os.name == "nt":
    import subprocess
    sys.exit(subprocess.call([sys.executable] + sys.argv[1:]))
os.execv(sys.executable, [sys.executable] + sys.argv[1:])
`

// pythonStubDiagnosis explains a python3 that started and exited before printing the runtime line,
// the way the Windows Microsoft Store alias does.
const pythonStubDiagnosis = "python3 exited before the runtime check ran; on Windows the Microsoft Store alias exits 49 - " +
	"put a Python 3.10+ interpreter first on PATH as python3 (see .issueops/testing/unit-and-contract.md#python-검사-환경)"

const selfVerifyGoTestTimeout = 10 * time.Minute

// StepDeps injects the command runner and every validation that PlannedSteps wires into steps. Each
// field is called with the binary path and root, and sometimes the seed, as its signature shows.
type StepDeps struct {
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

// PlannedSteps returns the self-verification steps for root in the order defined by the domain
// plan. goTestStep records the go test result so the contract golden step can reuse it. It panics
// when an implementation is missing for a domain step, or the reverse.
func PlannedSteps(root string, tempBin string, seed int64, goTestStep *contract.StepResult, deps StepDeps) []PlannedStep {
	var riskQAEvidence RiskQAEvidence
	planned := []PlannedStep{
		{Label: "harness invariants", Run: func() contract.StepResult { return deps.ValidateHarnessInvariants(root) }},
		// CI의 Format check와 같은 게이트를 로컬에서도 무조건 실행한다. gofmt는
		// 값싸고 결정적이므로 긴 go test보다 앞에 두어 fail-fast 모드에서 먼저 드러낸다.
		{Label: "gofmt", Run: func() contract.StepResult { return deps.ValidateGoFormat(root) }},
		{Label: "Python script tests", Run: func() contract.StepResult {
			step := deps.RunCommandStep(root, "Python script tests", 5*time.Minute, "", "python3", "-c", pythonRuntimeDiscovery, "scripts/python_suite_runner.py")
			// A started process that failed with no stdout never reached the runtime line. Not-found
			// and timeout errors do not start with "exit status " and stay as they are.
			if !step.OK && step.StdoutBytes == 0 && !step.StdoutTruncated && strings.HasPrefix(step.Error, "exit status ") {
				step.Error += "; " + pythonStubDiagnosis
			}
			return step
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
	byLabel := make(map[string]PlannedStep, len(planned))
	for _, step := range planned {
		byLabel[step.Label] = step
	}
	ordered := make([]PlannedStep, 0, len(planned))
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

// CachedContractGoldenStep returns the contract golden test step. When the full go test run passed
// it is reused; otherwise the golden tests run on their own and must show passing evidence for
// every required test and package.
func CachedContractGoldenStep(goTestStep contract.StepResult, deps StepDeps) contract.StepResult {
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
