package selfverify

import (
	"fmt"
	"strconv"
)

// RerunCommands returns copy-paste commands to reproduce a failure: the targeted command for failedStep
// when one is known, followed by the full self-verify rerun for baseSeed and targetScore.
func RerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	commands := []string{}
	if command, ok := StepRerunCommand(failedStep); ok {
		commands = append(commands, command)
	}
	commands = append(commands, fmt.Sprintf("./bin/issueops self-verify --collect-all-steps --seed=%d --target-score=%s --llm-eval=false --progress=jsonl --json", baseSeed, FormatScore(targetScore)))
	return commands
}

var selfVerifyStepRerunCommands = map[string]string{
	"Python script tests":    "python3 scripts/python_suite_runner.py",
	"Go test match guard":    "bash scripts/verify-go-test-match-test.sh",
	"go test":                "go test ./... -count=1",
	"contract golden tests":  "go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1",
	"risk QA tier":           "go vet ./... && go test -race ./... -count=1",
	"go build":               "go build -o bin/issueops ./cmd/issueops",
	"gofmt":                  "gofmt -l $(git ls-files '*.go')",
	"inspect smoke":          "./bin/issueops inspect --json",
	"docs index smoke":       "./bin/issueops docs --json",
	"candidate export":       "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify candidates --save-state --state-key self-verify-candidates-test --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state read --key self-verify-candidates-test --json; rm -rf \"$tmp_state\"",
	"step budget baseline":   "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-budget-baseline --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify compare --baseline-key self-verify-budget-baseline --candidate-key self-verify-budget-baseline --json; rm -rf \"$tmp_state\"",
	"install dry-run smoke":  "tmp_home=\"$(mktemp -d)\" tmp_root=\"$(mktemp -d)\" && mkdir -p \"$tmp_root/skills/atomic-commit-push\" && printf -- '---\\nname: atomic-commit-push\\ndescription: smoke\\n---\\n' > \"$tmp_root/skills/atomic-commit-push/SKILL.md\" && HOME=\"$tmp_home\" CODEX_HOME=\"$tmp_home/.codex\" ISSUEOPS_ROOT=\"$tmp_root\" ./bin/issueops install --dry-run --project-local --json; rm -rf \"$tmp_home\" \"$tmp_root\"",
	"command policy smoke":   "./bin/issueops policy check --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short",
	"command audit smoke":    "tmp_audit=$(mktemp) && ISSUEOPS_AUDIT_LOG=\"$tmp_audit\" ./bin/issueops policy audit --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short",
	"contract check":         "./bin/issueops contract check --json",
	"worker lifecycle smoke": "tmp_worker=$(mktemp -d) && ISSUEOPS_WORKER_DIR=\"$tmp_worker\" ./bin/issueops worker enqueue --kind smoke --json",
	"MCP smoke":              "./bin/issueops mcp",
	"state roundtrip":        "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state write --key smoke --value smoke --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state read --key smoke --json; rm -rf \"$tmp_state\"",
	"parallel isolation":     "./bin/issueops self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
	"preflight fuzz":         "./bin/issueops preflight --json \"$PWD\"",
	"native integration":     "./scripts/install-native.sh && ./bin/issueops install --dry-run --json",
	"redaction audit":        "go test ./cmd/issueops -run Test -count=1",
	"QA gate":                "go test ./cmd/issueops -run Test -count=1",
	"harness invariants":     "go test ./cmd/issueops -run Test -count=1",
}

// StepRerunCommand returns the shell command that reruns the step with label, and false when the label
// has no known command.
func StepRerunCommand(label string) (string, bool) {
	command, ok := selfVerifyStepRerunCommands[label]
	return command, ok
}

// FormatScore formats score without a fractional part when it is a whole number.
func FormatScore(score float64) string {
	if score == float64(int64(score)) {
		return strconv.FormatInt(int64(score), 10)
	}
	return strconv.FormatFloat(score, 'f', -1, 64)
}
