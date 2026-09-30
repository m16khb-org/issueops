package qualitycli

import (
	"fmt"

	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
	quality "issueops/internal/domain/quality"
	"issueops/internal/domain/qualitycatalog"
	augmentdomain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

func collectSelfAugmentOpenCount(root string) (int, error) {
	plan := planningForTest(root, statestore.StateDir(), hostDeps.Version).Plan(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	return len(augmentdomain.CandidateIDsByStatus(plan.Candidates, augmentcontract.CandidateStatusOpen)), nil
}

func collectSelfVerifyOpenCount(root string) (int, error) {
	result := planningForTest(root, statestore.StateDir(), hostDeps.Version).ExportCandidates()
	return len(verifydomain.CandidateIDsByStatus(result.Candidates, augmentcontract.CandidateStatusOpen)), nil
}

func collectQualityCandidates(root string) []QualityCandidate {
	candidates := qualitycatalog.Candidates()

	plan := planningForTest(root, statestore.StateDir(), hostDeps.Version).Plan(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	statusByID := map[string]augmentcontract.SelfAugmentCandidate{}
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
