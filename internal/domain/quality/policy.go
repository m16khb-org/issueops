package quality

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	contract "issueops/internal/contract/quality"
)

// AddQualityCollectorFailure records a collector failure on result: it appends
// warning to Warnings and to the blocking quality-collector-error finding
// (created at the front when absent), and sets the collection to error, health to
// unknown and the gate to block.
func AddQualityCollectorFailure(result *contract.InspectResult, warning string) {
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
		result.Findings = append([]contract.Finding{{
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
	result.CollectionStatus = contract.CollectionStatusError
	result.HealthStatus = contract.HealthStatusUnknown
	result.GateStatus = contract.GateStatusBlock
}

// AddSNRRegressionFinding appends the blocking code-snr-regression finding for
// the drop from baseline to current and moves health to needs-attention and the
// gate to block.
func AddSNRRegressionFinding(result *contract.InspectResult, baseline, current float64) {
	result.Findings = append(result.Findings, contract.Finding{
		ID:            "code-snr-regression",
		Severity:      "p1",
		Title:         "Code signal-to-noise regressed from baseline",
		Blocking:      true,
		Evidence:      []string{fmt.Sprintf("baseline=%.4f current=%.4f delta=%+.4f", baseline, current, current-baseline)},
		Remediation:   "Inspect the changed production code for avoidable structural noise or explicitly save an approved new baseline.",
		VerifyCommand: "./bin/issueops quality inspect --trend --json",
	})
	result.HealthStatus = contract.HealthStatusNeedsAttention
	result.GateStatus = contract.GateStatusBlock
}

// SuccessfulSignalValue returns the value of the signal with the given id and
// whether that signal reported status "ok". An absent signal yields 0 and false.
func SuccessfulSignalValue(signals []contract.Signal, id string) (float64, bool) {
	for _, signal := range signals {
		if signal.ID == id {
			return signal.Value, signal.Status == "ok"
		}
	}
	return 0, false
}

// CollectQualityFindings builds the findings for the collected evidence in a fixed
// order: collector errors, pioneer skill coverage, pioneer isolated evaluation,
// low coverage, high branch counts, and open audit items. Evidence that is
// clean produces no finding.
func CollectQualityFindings(
	warnings []string,
	lowCoverage []contract.CoveragePackage,
	branchFunctions []contract.BranchFunction,
	auditItems []contract.AuditItem,
	pioneer contract.PioneerCoverage,
) []contract.Finding {
	findings := make([]contract.Finding, 0, 5)
	findings = appendCollectorFinding(findings, warnings)
	findings = appendPioneerSkillFinding(findings, pioneer)
	findings = appendPioneerIsolatedFinding(findings, pioneer)
	findings = appendLowCoverageFinding(findings, lowCoverage)
	findings = appendHighBranchFinding(findings, branchFunctions)
	findings = appendAuditFinding(findings, auditItems)
	return findings
}

func appendCollectorFinding(findings []contract.Finding, warnings []string) []contract.Finding {
	if len(warnings) == 0 {
		return findings
	}
	return append(findings, contract.Finding{
		ID:            "quality-collector-error",
		Severity:      "p0",
		Title:         "Quality evidence collection failed",
		Blocking:      true,
		Evidence:      append([]string(nil), warnings...),
		Remediation:   "Repair the failing collector before relying on repository health.",
		VerifyCommand: "./bin/issueops quality inspect --json",
	})
}

func appendPioneerSkillFinding(findings []contract.Finding, pioneer contract.PioneerCoverage) []contract.Finding {
	if len(pioneer.BenchmarkMissing) == 0 && len(pioneer.ReproductionMissing) == 0 {
		return findings
	}
	evidence := make([]string, 0, len(pioneer.BenchmarkMissing)+len(pioneer.ReproductionMissing))
	for _, name := range pioneer.BenchmarkMissing {
		evidence = append(evidence, "benchmark missing: "+name)
	}
	for _, name := range pioneer.ReproductionMissing {
		evidence = append(evidence, "reproduction missing: "+name)
	}
	return append(findings, contract.Finding{
		ID:            "pioneer-skill-coverage",
		Severity:      "p1",
		Title:         "Canonical pioneer skill evaluation is incomplete",
		Evidence:      evidence,
		Remediation:   "Add one benchmark fixture and one honest reproduction case for every canonical pioneer skill.",
		VerifyCommand: "./bin/issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json",
	})
}

func appendPioneerIsolatedFinding(findings []contract.Finding, pioneer contract.PioneerCoverage) []contract.Finding {
	if pioneer.IsolatedExpected > 0 &&
		(pioneer.IsolatedObserved != pioneer.IsolatedExpected || pioneer.IsolatedFailed > 0) {
		return append(findings, contract.Finding{
			ID:            "pioneer-isolated-evaluation-incomplete",
			Severity:      "p0",
			Title:         "Fresh-context pioneer evaluation evidence is incomplete or failed",
			Blocking:      true,
			Evidence:      PioneerIsolatedEvidence(pioneer),
			Remediation:   "Run every canonical pioneer fixture in an isolated context and record hashes and verdicts in the evaluation manifest.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	if pioneer.IsolatedBlocked > 0 {
		return append(findings, contract.Finding{
			ID:            "pioneer-isolated-evaluation-blocked",
			Severity:      "p1",
			Title:         "Fresh-context pioneer evaluation has capability-blocked cases",
			Evidence:      PioneerIsolatedEvidence(pioneer),
			Remediation:   "Re-run blocked cases only when the named host capability is available; do not relabel them as pass.",
			VerifyCommand: "./bin/issueops quality inspect --json",
		})
	}
	return findings
}

func appendLowCoverageFinding(findings []contract.Finding, lowCoverage []contract.CoveragePackage) []contract.Finding {
	if len(lowCoverage) == 0 {
		return findings
	}
	return append(findings, contract.Finding{
		ID:            "low-coverage-packages",
		Severity:      "p2",
		Title:         "Packages remain below the coverage observation threshold",
		Evidence:      boundedEvidence(CoverageEvidence(lowCoverage), 50),
		Remediation:   "Add behavior-focused boundary tests to the highest-risk packages before promotion.",
		VerifyCommand: "go test -cover ./... -count=1",
	})
}

func appendHighBranchFinding(findings []contract.Finding, branchFunctions []contract.BranchFunction) []contract.Finding {
	highBranchCount := 0
	for _, function := range branchFunctions {
		if function.Branches > 12 {
			highBranchCount++
		}
	}
	if highBranchCount == 0 {
		return findings
	}
	return append(findings, contract.Finding{
		ID:            "high-branch-functions",
		Severity:      "p2",
		Title:         "High-branch production functions need targeted review",
		Evidence:      BranchEvidence(branchFunctions, 12),
		Remediation:   "Review the listed functions for missing invariants and add focused regression tests.",
		VerifyCommand: "./bin/issueops quality inspect --json",
	})
}

func appendAuditFinding(findings []contract.Finding, auditItems []contract.AuditItem) []contract.Finding {
	if len(auditItems) == 0 {
		return findings
	}
	severity := "p1"
	blocking := false
	for _, item := range auditItems {
		if item.Priority == "P0" {
			severity = "p0"
			blocking = true
			break
		}
	}
	return append(findings, contract.Finding{
		ID:            "project-audit-items",
		Severity:      severity,
		Title:         "P0, P1, or P2 project audit items remain open",
		Blocking:      blocking,
		Evidence:      AuditEvidence(auditItems),
		Remediation:   "Resolve or explicitly reclassify the referenced project audit items.",
		VerifyCommand: "./bin/issueops quality inspect --json",
	})
}

func boundedEvidence(evidence []string, limit int) []string {
	if len(evidence) <= limit {
		return evidence
	}
	return append([]string(nil), evidence[:limit]...)
}

// Statuses returns the collection, health and gate statuses for warnings and
// findings. Any warning means error/unknown/block; otherwise findings yield
// needs-attention, blocking when any finding blocks and report-only when none does.
func Statuses(warnings []string, findings []contract.Finding) (string, string, string) {
	if len(warnings) > 0 {
		return contract.CollectionStatusError, contract.HealthStatusUnknown, contract.GateStatusBlock
	}
	if len(findings) == 0 {
		return contract.CollectionStatusOK, contract.HealthStatusHealthy, contract.GateStatusPass
	}
	for _, finding := range findings {
		if finding.Blocking {
			return contract.CollectionStatusOK, contract.HealthStatusNeedsAttention, contract.GateStatusBlock
		}
	}
	return contract.CollectionStatusOK, contract.HealthStatusNeedsAttention, contract.GateStatusReportOnly
}

// ParseCoveragePackages extracts from `go test -cover` output the packages whose
// coverage is below threshold, sorted by coverage and then package name.
func ParseCoveragePackages(output string, threshold float64) []contract.CoveragePackage {
	var packages []contract.CoveragePackage
	for line := range strings.SplitSeq(output, "\n") {
		if pkg, ok := parseLowCoverageLine(line, threshold); ok {
			packages = append(packages, pkg)
		}
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Coverage != packages[j].Coverage {
			return packages[i].Coverage < packages[j].Coverage
		}
		return packages[i].Package < packages[j].Package
	})
	return packages
}

// parseLowCoverageLine extracts the package of one `go test -cover` line whose coverage is below threshold.
func parseLowCoverageLine(line string, threshold float64) (contract.CoveragePackage, bool) {
	if !strings.Contains(line, "coverage:") || !strings.Contains(line, "% of statements") {
		return contract.CoveragePackage{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return contract.CoveragePackage{}, false
	}
	coverageIdx := slices.Index(fields, "coverage:")
	if coverageIdx < 0 || coverageIdx+1 >= len(fields) {
		return contract.CoveragePackage{}, false
	}
	valueText := strings.TrimSuffix(fields[coverageIdx+1], "%")
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil || value >= threshold {
		return contract.CoveragePackage{}, false
	}
	packageName := fields[0]
	if (fields[0] == "ok" || fields[0] == "?") && len(fields) > 1 {
		packageName = fields[1]
	}
	return contract.CoveragePackage{Package: packageName, Coverage: value}, true
}

// FailedTestPackages returns the distinct packages named by `go test`'s
// "FAIL\t<pkg>" summary lines, in output order, keeping at most limit and
// reporting how many more were omitted.
func FailedTestPackages(output string, limit int) ([]string, int) {
	packages := []string{}
	seen := map[string]bool{}
	omitted := 0
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "FAIL" || seen[fields[1]] {
			continue
		}
		seen[fields[1]] = true
		if len(packages) < limit {
			packages = append(packages, fields[1])
		} else {
			omitted++
		}
	}
	return packages, omitted
}

// StatusForCount returns "needs_attention" when count is positive and "ok" otherwise.
func StatusForCount(count int) string {
	if count > 0 {
		return "needs_attention"
	}
	return "ok"
}

// StatusForCollector returns "error" when the collector failed with err and
// fallback otherwise.
func StatusForCollector(err error, fallback string) string {
	if err != nil {
		return "error"
	}
	return fallback
}

// PioneerIsolatedStatus classifies the fresh-context pioneer evaluation: "unknown"
// when nothing is expected, "needs_attention" when observations are missing or
// any case failed or is blocked, and "ok" otherwise.
func PioneerIsolatedStatus(coverage contract.PioneerCoverage) string {
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

// PioneerIsolatedEvidence renders the isolated pioneer evaluation counts as
// finding evidence, with a note that the fixtures are not hidden holdouts.
func PioneerIsolatedEvidence(coverage contract.PioneerCoverage) []string {
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

// FirstQualityWarning returns the first warning as an error, or nil when there are none.
func FirstQualityWarning(warnings []string) error {
	if len(warnings) == 0 {
		return nil
	}
	return errors.New(warnings[0])
}

// CoverageEvidence renders one "package coverage%" line per package, or the
// `go test -cover ./...` command when there are none.
func CoverageEvidence(packages []contract.CoveragePackage) []string {
	evidence := []string{}
	for _, pkg := range packages {
		evidence = append(evidence, fmt.Sprintf("%s %.1f%%", pkg.Package, pkg.Coverage))
	}
	if len(evidence) == 0 {
		return []string{"go test -cover ./..."}
	}
	return evidence
}

// BranchEvidence renders up to 10 lines for functions whose branch count exceeds
// threshold, or a scan description when none does.
func BranchEvidence(functions []contract.BranchFunction, threshold int) []string {
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

// AuditEvidence renders one "ID priority title" line per audit item, or the audit
// document path when there are none.
func AuditEvidence(items []contract.AuditItem) []string {
	evidence := []string{}
	for _, item := range items {
		evidence = append(evidence, fmt.Sprintf("%s %s %s", item.ID, item.Priority, item.Title))
	}
	if len(evidence) == 0 {
		return []string{".issueops/PROJECT_AUDIT.md"}
	}
	return evidence
}

// SNREvidence renders the signal, noise and total line counts of snr, with a note
// on what the ratio measures.
func SNREvidence(snr contract.SNRResult) []string {
	return []string{
		fmt.Sprintf("signal=%d noise=%d total=%d lines", snr.SignalLines, snr.NoiseLines, snr.TotalLines),
		"Shannon-style code signal-to-noise (logic vs blank/comment/structural); higher is denser",
	}
}

// SNRStructuralOnly reports whether line is non-empty and made only of the
// structural characters { } ( ) and comma, which count as noise.
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

// ValidSNRRatio reports whether ratio is a finite number in [0, 1].
func ValidSNRRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio >= 0 && ratio <= 1
}

// SNRRegressed reports whether current fell below baseline by more than 0.01.
func SNRRegressed(baseline, current float64) bool {
	return current < baseline-0.01
}
