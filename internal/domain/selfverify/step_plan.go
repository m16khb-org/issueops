package selfverify

var stepOrder = []string{
	"harness invariants",
	"gofmt",
	"Python script tests",
	"Go test match guard",
	"risk QA tier",
	"go test",
	"contract golden tests",
	"go build",
	"binary drift",
	"inspect smoke",
	"docs index smoke",
	"candidate export",
	"step budget baseline",
	"install dry-run smoke",
	"command policy smoke",
	"command audit smoke",
	"contract check",
	"tool contract conformance",
	"worker lifecycle smoke",
	"MCP smoke",
	"state roundtrip",
	"parallel isolation",
	"preflight fuzz",
	"web fetch battery",
	"native integration",
	"redaction audit",
	"QA gate",
}

// StepOrder returns a copy of the labels of the self-verify steps in execution order.
func StepOrder() []string { return append([]string(nil), stepOrder...) }

// ReuseRiskRaceAsFullTest reports whether the risk-tier race run can stand in for the full go test step:
// it passed and it covered the full test set.
func ReuseRiskRaceAsFullTest(riskPassed, coversFullGoTest bool) bool {
	return riskPassed && coversFullGoTest
}

// ReuseFullTestAsGolden reports whether a passing full go test run can stand in for the golden test step.
func ReuseFullTestAsGolden(fullTestPassed bool) bool {
	return fullTestPassed
}

// ContinueAfterFailure reports whether the loop keeps running steps after a failure, which happens only
// when collectAllSteps is set; otherwise it fails fast.
func ContinueAfterFailure(collectAllSteps bool) bool { return collectAllSteps }
