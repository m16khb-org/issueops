package quality

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/quality"
	catalog "issueops/internal/contract/qualitycatalog"
	policy "issueops/internal/domain/quality"
)

type InspectDeps struct {
	Now                  func() string
	Coverage             func(string) (string, error)
	BranchFunctions      func(string) ([]contract.BranchFunction, []string)
	AuditItems           func(string) ([]contract.AuditItem, []string)
	SelfAugmentOpenCount func(string) (int, error)
	SelfVerifyOpenCount  func(string) (int, error)
	Candidates           func(string) []catalog.Candidate
	CodeSNR              func(string) (contract.SNRResult, error)
	PioneerCoverage      func(string) (contract.PioneerCoverage, error)
}

func Inspect(root string, deps InspectDeps) contract.InspectResult {
	type textResult struct {
		value string
		err   error
	}
	type branchResult struct {
		value    []contract.BranchFunction
		warnings []string
	}
	type auditResult struct {
		value    []contract.AuditItem
		warnings []string
	}
	type pioneerResult struct {
		value contract.PioneerCoverage
		err   error
	}
	type snrResult struct {
		value contract.SNRResult
		err   error
	}
	coverageResults := make(chan textResult, 1)
	branchResults := make(chan branchResult, 1)
	auditResults := make(chan auditResult, 1)
	snrResults := make(chan snrResult, 1)
	pioneerResults := make(chan pioneerResult, 1)
	go func() {
		value, err := deps.Coverage(root)
		coverageResults <- textResult{value: value, err: err}
	}()
	go func() {
		value, warnings := deps.BranchFunctions(root)
		branchResults <- branchResult{value: value, warnings: warnings}
	}()
	go func() {
		value, warnings := deps.AuditItems(root)
		auditResults <- auditResult{value: value, warnings: warnings}
	}()
	go func() {
		value, err := deps.CodeSNR(root)
		snrResults <- snrResult{value: value, err: err}
	}()
	go func() {
		value, err := deps.PioneerCoverage(root)
		pioneerResults <- pioneerResult{value: value, err: err}
	}()

	selfAugmentOpen, selfAugmentErr := deps.SelfAugmentOpenCount(root)
	selfVerifyOpen, selfVerifyErr := deps.SelfVerifyOpenCount(root)
	candidates := deps.Candidates(root)

	coverage := <-coverageResults
	branches := <-branchResults
	audit := <-auditResults
	snr := <-snrResults
	pioneer := <-pioneerResults
	warnings := []string{}
	if coverage.err != nil {
		warnings = append(warnings, coverageWarning(coverage.err, coverage.value))
	}
	lowCoverage := policy.ParseCoveragePackages(coverage.value, 60)
	branchFunctions := branches.value
	warnings = append(warnings, branches.warnings...)
	auditItems := audit.value
	warnings = append(warnings, audit.warnings...)
	if selfAugmentErr != nil {
		warnings = append(warnings, "self-augment candidates: "+selfAugmentErr.Error())
	}
	if selfVerifyErr != nil {
		warnings = append(warnings, "self-verify candidates: "+selfVerifyErr.Error())
	}
	if pioneer.err != nil {
		warnings = append(warnings, "pioneer coverage: "+pioneer.err.Error())
	}
	if snr.err != nil {
		warnings = append(warnings, "code-snr: "+snr.err.Error())
	}
	highBranchCount := 0
	branchCandidateCount := 0
	for _, fn := range branchFunctions {
		if fn.Branches > 6 {
			branchCandidateCount++
		}
		if fn.Branches > 12 {
			highBranchCount++
		}
	}
	signals := []contract.Signal{
		{ID: "self-augment-open-candidates", Category: "candidate", Status: policy.StatusForCollector(selfAugmentErr, "ok"), Value: float64(selfAugmentOpen), Evidence: []string{"self-augment candidate catalog"}},
		{ID: "self-verify-open-candidates", Category: "candidate", Status: policy.StatusForCollector(selfVerifyErr, "ok"), Value: float64(selfVerifyOpen), Evidence: []string{"self-verify candidate export"}},
		{ID: "low-coverage-packages", Category: "coverage", Status: policy.StatusForCollector(coverage.err, policy.StatusForCount(len(lowCoverage))), Value: float64(len(lowCoverage)), Threshold: 60, Evidence: policy.CoverageEvidence(lowCoverage)},
		{ID: "branch-candidate-functions", Category: "complexity", Status: policy.StatusForCollector(policy.FirstQualityWarning(branches.warnings), policy.StatusForCount(branchCandidateCount)), Value: float64(branchCandidateCount), Threshold: 6, Evidence: policy.BranchEvidence(branchFunctions, 6)},
		{ID: "high-branch-functions", Category: "complexity", Status: policy.StatusForCount(highBranchCount), Value: float64(highBranchCount), Threshold: 12, Evidence: policy.BranchEvidence(branchFunctions, 12)},
		{ID: "audit-p0-p1-p2-items", Category: "audit", Status: policy.StatusForCollector(policy.FirstQualityWarning(audit.warnings), policy.StatusForCount(len(auditItems))), Value: float64(len(auditItems)), Evidence: policy.AuditEvidence(auditItems)},
		{ID: "pioneer-benchmark-coverage", Category: "skill", Status: policy.StatusForCollector(pioneer.err, policy.StatusForCount(len(pioneer.value.BenchmarkMissing))), Value: float64(pioneer.value.BenchmarkObserved), Threshold: float64(pioneer.value.Expected), Evidence: append([]string(nil), pioneer.value.BenchmarkMissing...)},
		{ID: "pioneer-reproduction-coverage", Category: "skill", Status: policy.StatusForCollector(pioneer.err, policy.StatusForCount(len(pioneer.value.ReproductionMissing))), Value: float64(pioneer.value.ReproductionObserved), Threshold: float64(pioneer.value.Expected), Evidence: append([]string(nil), pioneer.value.ReproductionMissing...)},
		{ID: "pioneer-isolated-evaluation", Category: "skill", Status: policy.StatusForCollector(pioneer.err, policy.PioneerIsolatedStatus(pioneer.value)), Value: float64(pioneer.value.IsolatedObserved), Threshold: float64(pioneer.value.IsolatedExpected), Evidence: policy.PioneerIsolatedEvidence(pioneer.value)},
		{ID: "code-snr", Category: "quality", Status: policy.StatusForCollector(snr.err, "ok"), Value: snr.value.Ratio, Evidence: policy.SNREvidence(snr.value)},
	}
	findings := policy.CollectQualityFindings(warnings, lowCoverage, branchFunctions, auditItems, pioneer.value)
	collectionStatus, healthStatus, gateStatus := policy.QualityStatuses(warnings, findings)
	return contract.InspectResult{
		OK:               collectionStatus == contract.CollectionStatusOK,
		CollectionStatus: collectionStatus,
		HealthStatus:     healthStatus,
		GateStatus:       gateStatus,
		GeneratedAt:      deps.Now(),
		IssueOpsRoot:     root,
		Summary: contract.Summary{
			SelfAugmentOpenCandidates: selfAugmentOpen,
			SelfVerifyOpenCandidates:  selfVerifyOpen,
			LowCoveragePackages:       len(lowCoverage),
			BranchCandidateFunctions:  branchCandidateCount,
			HighBranchFunctions:       highBranchCount,
			AuditP1P2Items:            len(auditItems),
			CandidateCount:            len(candidates),
		},
		Signals:         signals,
		Findings:        findings,
		PioneerCoverage: pioneer.value,
		Candidates:      candidates,
		Warnings:        warnings,
	}
}

// failedCoveragePackageLimit keeps the collector warning readable when many
// packages fail at once.
const failedCoveragePackageLimit = 10

// coverageWarning names the packages `go test -cover` reported as failing, so
// a collection error points at its cause instead of a bare exit status.
func coverageWarning(err error, output string) string {
	warning := "coverage: " + err.Error()
	packages, omitted := policy.FailedTestPackages(output, failedCoveragePackageLimit)
	if len(packages) == 0 {
		return warning
	}
	list := strings.Join(packages, ", ")
	if omitted > 0 {
		list += fmt.Sprintf(" and %d more", omitted)
	}
	return warning + " (failed packages: " + list + ")"
}
