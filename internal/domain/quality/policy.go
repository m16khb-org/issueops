package quality

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	contract "issueops/internal/contract/quality"
)

const (
	CollectionStatusOK         = contract.CollectionStatusOK
	CollectionStatusError      = contract.CollectionStatusError
	HealthStatusHealthy        = contract.HealthStatusHealthy
	HealthStatusNeedsAttention = contract.HealthStatusNeedsAttention
	HealthStatusUnknown        = contract.HealthStatusUnknown
	GateStatusPass             = contract.GateStatusPass
	GateStatusReportOnly       = contract.GateStatusReportOnly
	GateStatusBlock            = contract.GateStatusBlock
)

type InspectResult = contract.InspectResult
type Signal = contract.Signal
type CoveragePackage = contract.CoveragePackage
type BranchFunction = contract.BranchFunction
type AuditItem = contract.AuditItem
type Finding = contract.Finding
type PioneerCoverage = contract.PioneerCoverage

func AddQualityCollectorFailure(result *InspectResult, warning string) {
	result.Warnings = append(result.Warnings, warning)
	found := false
	for index := range result.Findings {
		if result.Findings[index].ID == "quality-collector-error" {
			result.Findings[index].Evidence = append(result.Findings[index].Evidence, warning)
			found = true
			break
		}
	}
	if !found {
		result.Findings = append([]Finding{{
			ID:            "quality-collector-error",
			Severity:      "p0",
			Title:         "Quality evidence collection failed",
			Blocking:      true,
			Evidence:      []string{warning},
			Remediation:   "Repair the failing collector before relying on repository health.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		}}, result.Findings...)
	}
	result.OK = false
	result.CollectionStatus = CollectionStatusError
	result.HealthStatus = HealthStatusUnknown
	result.GateStatus = GateStatusBlock
}

func AddSNRRegressionFinding(result *InspectResult, baseline, current float64) {
	result.Findings = append(result.Findings, Finding{
		ID:            "code-snr-regression",
		Severity:      "p1",
		Title:         "Code signal-to-noise regressed from baseline",
		Blocking:      true,
		Evidence:      []string{fmt.Sprintf("baseline=%.4f current=%.4f delta=%+.4f", baseline, current, current-baseline)},
		Remediation:   "Inspect the changed production code for avoidable structural noise or explicitly save an approved new baseline.",
		VerifyCommand: "./bin/issueops quality inspect --trend --json",
	})
	result.HealthStatus = HealthStatusNeedsAttention
	result.GateStatus = GateStatusBlock
}

func SuccessfulSignalValue(signals []Signal, id string) (float64, bool) {
	for _, signal := range signals {
		if signal.ID == id {
			return signal.Value, signal.Status == "ok"
		}
	}
	return 0, false
}

func CollectQualityFindings(
	warnings []string,
	lowCoverage []CoveragePackage,
	branchFunctions []BranchFunction,
	auditItems []AuditItem,
	pioneer PioneerCoverage,
) []Finding {
	findings := make([]Finding, 0, 5)
	if len(warnings) > 0 {
		findings = append(findings, Finding{
			ID:            "quality-collector-error",
			Severity:      "p0",
			Title:         "Quality evidence collection failed",
			Blocking:      true,
			Evidence:      append([]string(nil), warnings...),
			Remediation:   "Repair the failing collector before relying on repository health.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	if len(pioneer.BenchmarkMissing) > 0 || len(pioneer.ReproductionMissing) > 0 {
		evidence := make([]string, 0, len(pioneer.BenchmarkMissing)+len(pioneer.ReproductionMissing))
		for _, name := range pioneer.BenchmarkMissing {
			evidence = append(evidence, "benchmark missing: "+name)
		}
		for _, name := range pioneer.ReproductionMissing {
			evidence = append(evidence, "reproduction missing: "+name)
		}
		findings = append(findings, Finding{
			ID:            "pioneer-skill-coverage",
			Severity:      "p1",
			Title:         "Canonical pioneer skill evaluation is incomplete",
			Evidence:      evidence,
			Remediation:   "Add one benchmark fixture and one honest reproduction case for every canonical pioneer skill.",
			VerifyCommand: "./bin/issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json",
		})
	}
	if pioneer.IsolatedExpected > 0 &&
		(pioneer.IsolatedObserved != pioneer.IsolatedExpected || pioneer.IsolatedFailed > 0) {
		findings = append(findings, Finding{
			ID:            "pioneer-isolated-evaluation-incomplete",
			Severity:      "p0",
			Title:         "Fresh-context pioneer evaluation evidence is incomplete or failed",
			Blocking:      true,
			Evidence:      PioneerIsolatedEvidence(pioneer),
			Remediation:   "Run every canonical pioneer fixture in an isolated context and record hashes and verdicts in the evaluation manifest.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	} else if pioneer.IsolatedBlocked > 0 {
		findings = append(findings, Finding{
			ID:            "pioneer-isolated-evaluation-blocked",
			Severity:      "p1",
			Title:         "Fresh-context pioneer evaluation has capability-blocked cases",
			Evidence:      PioneerIsolatedEvidence(pioneer),
			Remediation:   "Re-run blocked cases only when the named host capability is available; do not relabel them as pass.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	if len(lowCoverage) > 0 {
		findings = append(findings, Finding{
			ID:            "low-coverage-packages",
			Severity:      "p2",
			Title:         "Packages remain below the coverage observation threshold",
			Evidence:      BoundedEvidence(CoverageEvidence(lowCoverage), 50),
			Remediation:   "Add behavior-focused boundary tests to the highest-risk packages before promotion.",
			VerifyCommand: "go test -cover ./... -count=1",
		})
	}
	highBranchCount := 0
	for _, function := range branchFunctions {
		if function.Branches > 12 {
			highBranchCount++
		}
	}
	if highBranchCount > 0 {
		findings = append(findings, Finding{
			ID:            "high-branch-functions",
			Severity:      "p2",
			Title:         "High-branch production functions need targeted review",
			Evidence:      BranchEvidence(branchFunctions, 12),
			Remediation:   "Review the listed functions for missing invariants and add focused regression tests.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	if len(auditItems) > 0 {
		severity := "p1"
		blocking := false
		for _, item := range auditItems {
			if item.Priority == "P0" {
				severity = "p0"
				blocking = true
				break
			}
		}
		findings = append(findings, Finding{
			ID:            "project-audit-items",
			Severity:      severity,
			Title:         "P0, P1, or P2 project audit items remain open",
			Blocking:      blocking,
			Evidence:      AuditEvidence(auditItems),
			Remediation:   "Resolve or explicitly reclassify the referenced project audit items.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	return findings
}

func BoundedEvidence(evidence []string, limit int) []string {
	if len(evidence) <= limit {
		return evidence
	}
	return append([]string(nil), evidence[:limit]...)
}

func QualityStatuses(warnings []string, findings []Finding) (string, string, string) {
	if len(warnings) > 0 {
		return CollectionStatusError, HealthStatusUnknown, GateStatusBlock
	}
	if len(findings) == 0 {
		return CollectionStatusOK, HealthStatusHealthy, GateStatusPass
	}
	for _, finding := range findings {
		if finding.Blocking {
			return CollectionStatusOK, HealthStatusNeedsAttention, GateStatusBlock
		}
	}
	return CollectionStatusOK, HealthStatusNeedsAttention, GateStatusReportOnly
}

func ParseCoveragePackages(output string, threshold float64) []CoveragePackage {
	var packages []CoveragePackage
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "coverage:") || !strings.Contains(line, "% of statements") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		coverageIdx := -1
		for i, field := range fields {
			if field == "coverage:" {
				coverageIdx = i
				break
			}
		}
		if coverageIdx < 0 || coverageIdx+1 >= len(fields) {
			continue
		}
		valueText := strings.TrimSuffix(fields[coverageIdx+1], "%")
		value, err := strconv.ParseFloat(valueText, 64)
		if err != nil || value >= threshold {
			continue
		}
		packageName := fields[0]
		if (fields[0] == "ok" || fields[0] == "?") && len(fields) > 1 {
			packageName = fields[1]
		}
		packages = append(packages, CoveragePackage{Package: packageName, Coverage: value})
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Coverage != packages[j].Coverage {
			return packages[i].Coverage < packages[j].Coverage
		}
		return packages[i].Package < packages[j].Package
	})
	return packages
}

func StatusForCount(count int) string {
	if count > 0 {
		return "needs_attention"
	}
	return "ok"
}

func StatusForCollector(err error, fallback string) string {
	if err != nil {
		return "error"
	}
	return fallback
}

func PioneerIsolatedStatus(coverage PioneerCoverage) string {
	if coverage.IsolatedExpected == 0 {
		return "unknown"
	}
	if coverage.IsolatedObserved != coverage.IsolatedExpected ||
		coverage.IsolatedFailed > 0 ||
		coverage.IsolatedBlocked > 0 {
		return "needs_attention"
	}
	return "ok"
}

func PioneerIsolatedEvidence(coverage PioneerCoverage) []string {
	return []string{
		fmt.Sprintf(
			"observed=%d expected=%d pass=%d blocked=%d fail=%d hidden=%d",
			coverage.IsolatedObserved,
			coverage.IsolatedExpected,
			coverage.IsolatedPassed,
			coverage.IsolatedBlocked,
			coverage.IsolatedFailed,
			coverage.HiddenHoldoutObserved,
		),
		"fresh-context fixtures are committed reproduction inputs, not hidden holdouts",
	}
}

func FirstQualityWarning(warnings []string) error {
	if len(warnings) == 0 {
		return nil
	}
	return errors.New(warnings[0])
}

func CoverageEvidence(packages []CoveragePackage) []string {
	evidence := []string{}
	for _, pkg := range packages {
		evidence = append(evidence, fmt.Sprintf("%s %.1f%%", pkg.Package, pkg.Coverage))
	}
	if len(evidence) == 0 {
		return []string{"go test -cover ./..."}
	}
	return evidence
}

func BranchEvidence(functions []BranchFunction, threshold int) []string {
	evidence := []string{}
	for _, fn := range functions {
		if fn.Branches <= threshold {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("%s:%d %s branches=%d", fn.File, fn.Line, fn.Name, fn.Branches))
		if len(evidence) == 10 {
			break
		}
	}
	if len(evidence) == 0 {
		return []string{"Go AST branch scan"}
	}
	return evidence
}

func AuditEvidence(items []AuditItem) []string {
	evidence := []string{}
	for _, item := range items {
		evidence = append(evidence, fmt.Sprintf("%s %s %s", item.ID, item.Priority, item.Title))
	}
	if len(evidence) == 0 {
		return []string{".issueops/PROJECT_AUDIT.md"}
	}
	return evidence
}

func SNREvidence(snr contract.SNRResult) []string {
	return []string{
		fmt.Sprintf("signal=%d noise=%d total=%d lines", snr.SignalLines, snr.NoiseLines, snr.TotalLines),
		"Shannon-style code signal-to-noise (logic vs blank/comment/structural); higher is denser",
	}
}

func SNRStructuralOnly(line string) bool {
	for _, r := range line {
		switch r {
		case '{', '}', '(', ')', ',':
		default:
			return false
		}
	}
	return line != ""
}

func ValidSNRRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio >= 0 && ratio <= 1
}

func SNRRegressed(baseline, current float64) bool {
	return current < baseline-0.01
}
