// Package selfverify holds the domain rules of the self-verification loop: the step plan, goal scoring,
// failure classification, output budgeting, rerun recipes and the summary contract. It is pure logic
// over the DTOs in internal/contract/selfverify and performs no process or filesystem I/O.
package selfverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	contract "issueops/internal/contract/selfverify"
)

// ContractValue builds the self_verification_summary contract: its version, required fields, goals and
// coverage claims, plus a SHA-256 hash of that content so schema drift is detectable.
func ContractValue() contract.SelfVerificationContract {
	contract := contract.SelfVerificationContract{
		Name:    "self_verification_summary",
		Version: 9,
		RequiredFields: []string{
			"total_runs",
			"total_steps",
			"passed_steps",
			"failed_steps",
			"target_score",
			"contract",
			"minimum_goal_score",
			"termination_eligible",
			"goal_scores",
			"coverage",
			"coverage_gaps",
			"step_labels",
			"planned_steps",
			"not_run_steps",
			"slowest_steps",
			"step_duration_stats",
			"failure_cause",
			"failure_cause_reason",
			"failure_cause_evidence",
		},
		GoalNames:      []string{},
		CoverageClaims: []string{},
	}
	for _, goal := range GoalDefinitions() {
		contract.GoalNames = append(contract.GoalNames, goal.Name)
	}
	for _, coverage := range CoverageDefinitions() {
		contract.CoverageClaims = append(contract.CoverageClaims, coverage.Claim)
	}
	b, _ := json.Marshal(contract)
	sum := sha256.Sum256(b)
	contract.Hash = hex.EncodeToString(sum[:])
	return contract
}

// GoalDefinitions returns the scored goals and the step labels that provide evidence for each goal.
func GoalDefinitions() []contract.SelfVerificationGoalDefinition {
	return []contract.SelfVerificationGoalDefinition{
		{Name: "test_suite", KoreanName: "테스트 스위트", Labels: []string{"go test", "contract golden tests", "Python script tests", "Go test match guard"}},
		{Name: "risk_qa", KoreanName: "위험도 기반 QA", Labels: []string{"risk QA tier"}},
		{Name: "build_release", KoreanName: "빌드 산출물", Labels: []string{"go build"}},
		{Name: "format_parity", KoreanName: "포맷 정합성", Labels: []string{"gofmt"}},
		{Name: "qa_smoke", KoreanName: "QA 스모크", Labels: []string{"harness invariants", "inspect smoke", "docs index smoke", "candidate export", "QA gate"}},
		{Name: "candidate_export", KoreanName: "후보 export", Labels: []string{"candidate export"}},
		{Name: "step_budget_baseline", KoreanName: "단계 budget baseline", Labels: []string{"step budget baseline"}},
		{Name: "install_dry_run", KoreanName: "설치 dry-run", Labels: []string{"install dry-run smoke"}},
		{Name: "policy_security", KoreanName: "정책·보안", Labels: []string{"command policy smoke", "command audit smoke", "preflight fuzz", "redaction audit"}},
		{Name: "mcp_state_regression", KoreanName: "MCP·상태 회귀", Labels: []string{"MCP smoke", "state roundtrip", "contract check", "tool contract conformance"}},
		{Name: "concurrency_isolation", KoreanName: "동시성 격리", Labels: []string{"parallel isolation"}},
		{Name: "worker_lifecycle", KoreanName: "Worker 생명주기", Labels: []string{"worker lifecycle smoke"}},
		{Name: "native_integration", KoreanName: "네이티브 통합", Labels: []string{"native integration"}},
	}
}

// CoverageDefinitions returns the claims the summary must back with evidence and the step labels that
// support each claim.
func CoverageDefinitions() []contract.SelfVerificationCoverageDefinition {
	return []contract.SelfVerificationCoverageDefinition{
		{Claim: "core repository invariants", Labels: []string{"harness invariants"}},
		{Claim: "test suite contract", Labels: []string{"go test", "contract golden tests", "Python script tests", "Go test match guard"}},
		{Claim: "risk-tier static and race QA", Labels: []string{"risk QA tier"}},
		{Claim: "release build artifact", Labels: []string{"go build"}},
		{Claim: "gofmt parity with the CI format gate", Labels: []string{"gofmt"}},
		{Claim: "CLI inspect/docs smoke", Labels: []string{"inspect smoke", "docs index smoke"}},
		{Claim: "self-verification candidate export", Labels: []string{"candidate export"}},
		{Claim: "step duration budget baseline", Labels: []string{"step budget baseline"}},
		{Claim: "install dry-run no-write smoke", Labels: []string{"install dry-run smoke"}},
		{Claim: "command policy boundary", Labels: []string{"command policy smoke"}},
		{Claim: "redacted command audit log", Labels: []string{"command audit smoke"}},
		{Claim: "CLI/MCP compatibility contract", Labels: []string{"contract check"}},
		{Claim: "tool-call schema conformance", Labels: []string{"tool contract conformance"}},
		{Claim: "no-shell worker lifecycle", Labels: []string{"worker lifecycle smoke"}},
		{Claim: "MCP and state regression", Labels: []string{"MCP smoke", "state roundtrip"}},
		{Claim: "parallel temp isolation", Labels: []string{"parallel isolation"}},
		{Claim: "git preflight fuzz", Labels: []string{"preflight fuzz"}},
		{Claim: "native integration", Labels: []string{"native integration"}},
		{Claim: "secret redaction audit", Labels: []string{"redaction audit"}},
		{Claim: "documentation QA gate", Labels: []string{"QA gate"}},
	}
}

// CoverageForLabels reports, for every coverage definition, whether all of its labels appear in
// stepLabels. It also returns a gap description for each claim that is missing labels.
func CoverageForLabels(stepLabels []string) ([]contract.SelfVerificationCoverage, []string) {
	labelSet := map[string]bool{}
	for _, label := range stepLabels {
		labelSet[label] = true
	}
	coverage := []contract.SelfVerificationCoverage{}
	gaps := []string{}
	for _, definition := range CoverageDefinitions() {
		item := contract.SelfVerificationCoverage{
			Claim:          definition.Claim,
			EvidenceLabels: append([]string{}, definition.Labels...),
			Covered:        true,
			MissingLabels:  []string{},
		}
		for _, label := range definition.Labels {
			if !labelSet[label] {
				item.Covered = false
				item.MissingLabels = append(item.MissingLabels, label)
				gaps = append(gaps, definition.Claim+": missing "+label)
			}
		}
		item.NotRun = len(item.MissingLabels) == len(definition.Labels)
		coverage = append(coverage, item)
	}
	return coverage, gaps
}

// NotRunSteps returns the planned step labels, in plan order, that never
// executed. A fail-fast stop leaves the rest of the plan unexecuted; those
// steps have no evidence, which differs from a step that ran and failed.
func NotRunSteps(executedLabels []string) []string {
	ran := make(map[string]bool, len(executedLabels))
	for _, label := range executedLabels {
		ran[label] = true
	}
	notRun := []string{}
	for _, label := range stepOrder {
		if !ran[label] {
			notRun = append(notRun, label)
		}
	}
	return notRun
}
