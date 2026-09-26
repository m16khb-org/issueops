package selfverify

var stepOrder = []string{
	"harness invariants",
	"gofmt",
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
	"daemon resilience",
	"preflight fuzz",
	"web fetch battery",
	"native integration",
	"redaction audit",
	"QA gate",
}

func StepOrder() []string { return append([]string(nil), stepOrder...) }

func ReuseRiskRaceAsFullTest(riskPassed, coversFullGoTest bool) bool {
	return riskPassed && coversFullGoTest
}

func ReuseFullTestAsGolden(fullTestPassed bool) bool {
	return fullTestPassed
}

func ContinueAfterFailure(collectAllSteps bool) bool { return collectAllSteps }
