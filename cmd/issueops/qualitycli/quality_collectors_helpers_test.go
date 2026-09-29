package qualitycli

import (
	"fmt"
	"strings"

	"issueops/cmd/issueops/selfworkflow"
	statestore "issueops/internal/adapter/outbound/state"
	quality "issueops/internal/domain/quality"
	"issueops/internal/domain/qualitycatalog"
	verifydomain "issueops/internal/domain/selfverify"
)

func collectSelfAugmentOpenCount(root string) (int, error) {
	plan := planningForTest(root, statestore.StateDir(), hostDeps.Version).Plan(selfworkflow.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	return len(selfworkflow.SelfAugmentCandidateIDsByStatus(plan.Candidates, selfworkflow.SelfAugmentCandidateStatusOpen)), nil
}

func collectSelfVerifyOpenCount(root string) (int, error) {
	result := planningForTest(root, statestore.StateDir(), hostDeps.Version).ExportCandidates()
	return len(verifydomain.CandidateIDsByStatus(result.Candidates, selfworkflow.SelfAugmentCandidateStatusOpen)), nil
}

func collectQualityCandidates(root string) []QualityCandidate {
	candidates := qualitycatalog.Candidates()

	plan := planningForTest(root, statestore.StateDir(), hostDeps.Version).Plan(selfworkflow.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	statusByID := map[string]selfworkflow.SelfAugmentCandidate{}
	for _, candidate := range plan.Candidates {
		statusByID[candidate.ID] = candidate
	}
	for i := range candidates {
		if projected, ok := statusByID[candidates[i].ID]; ok {
			candidates[i].Status = projected.Status
			candidates[i].Score = projected.Score
		}
	}
	return candidates
}

func parseCoveragePackages(output string, threshold float64) []CoveragePackage {
	return quality.ParseCoveragePackages(output, threshold)
}

func renderCoveragePackages(packages []CoveragePackage) string {
	var output strings.Builder
	for _, item := range packages {
		_, _ = fmt.Fprintf(
			&output,
			"%s coverage: %.1f%% of statements\n",
			item.Package,
			item.Coverage,
		)
	}
	return output.String()
}

var sourceCollectorEffects struct {
	PioneerCoverage func(string) (PioneerCoverage, error)
	BranchFunctions func(string) ([]BranchFunction, []string)
	AuditItems      func(string) ([]AuditItem, []string)
}

func ConfigureSourceCollectors(branches func(string) ([]BranchFunction, []string), audits func(string) ([]AuditItem, []string), pioneer func(string) (PioneerCoverage, error)) {
	sourceCollectorEffects.PioneerCoverage = pioneer
	sourceCollectorEffects.BranchFunctions = branches
	sourceCollectorEffects.AuditItems = audits
}

var pioneerCoverageCollector = func(root string) (PioneerCoverage, error) {
	if sourceCollectorEffects.PioneerCoverage == nil {
		return PioneerCoverage{}, fmt.Errorf("quality pioneer collector is not configured")
	}
	return sourceCollectorEffects.PioneerCoverage(root)
}

func collectBranchFunctions(root string) ([]BranchFunction, []string) {
	if sourceCollectorEffects.BranchFunctions == nil {
		return nil, []string{"branch scan: quality source collector is not configured"}
	}
	return sourceCollectorEffects.BranchFunctions(root)
}
func collectAuditItems(root string) ([]AuditItem, []string) {
	if sourceCollectorEffects.AuditItems == nil {
		return nil, []string{"audit scan: quality source collector is not configured"}
	}
	return sourceCollectorEffects.AuditItems(root)
}

func statusForCount(count int) string { return quality.StatusForCount(count) }

func statusForCollector(err error, fallback string) string {
	return quality.StatusForCollector(err, fallback)
}

func pioneerIsolatedStatus(coverage PioneerCoverage) string {
	return quality.PioneerIsolatedStatus(coverage)
}

func pioneerIsolatedEvidence(coverage PioneerCoverage) []string {
	return quality.PioneerIsolatedEvidence(coverage)
}

func firstQualityWarning(warnings []string) error { return quality.FirstQualityWarning(warnings) }

func coverageEvidence(packages []CoveragePackage) []string { return quality.CoverageEvidence(packages) }

func branchEvidence(functions []BranchFunction, threshold int) []string {
	return quality.BranchEvidence(functions, threshold)
}

func auditEvidence(items []AuditItem) []string { return quality.AuditEvidence(items) }
